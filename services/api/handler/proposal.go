package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "github.com/puri-cp/puri/gen/common/v1"
	proposalv1 "github.com/puri-cp/puri/gen/proposal/v1"
	submissionv1 "github.com/puri-cp/puri/gen/submission/v1"
	submissionv1connect "github.com/puri-cp/puri/gen/submission/v1/submissionv1connect"
	"github.com/puri-cp/puri/services/api/auth"
	"github.com/puri-cp/puri/services/api/repository"
)

type ProposalServiceHandler struct {
	repo      *repository.ProposalRepository
	submitCli submissionv1connect.SubmissionServiceClient
}

func NewProposalServiceHandler(repo *repository.ProposalRepository, submissionURL string) *ProposalServiceHandler {
	return &ProposalServiceHandler{
		repo:      repo,
		submitCli: submissionv1connect.NewSubmissionServiceClient(http.DefaultClient, submissionURL),
	}
}

func (h *ProposalServiceHandler) CreateProposal(ctx context.Context, req *connect.Request[proposalv1.CreateProposalRequest]) (*connect.Response[proposalv1.CreateProposalResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("unauthenticated"))
	}
	p, err := h.repo.Create(ctx, userID, req.Msg.Title)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("create proposal: %w", err))
	}
	return connect.NewResponse(&proposalv1.CreateProposalResponse{ProposalId: p.ID}), nil
}

func (h *ProposalServiceHandler) UpdateProposal(ctx context.Context, req *connect.Request[proposalv1.UpdateProposalRequest]) (*connect.Response[proposalv1.UpdateProposalResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("unauthenticated"))
	}
	p, err := h.repo.GetByID(ctx, req.Msg.ProposalId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if p == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("proposal not found"))
	}
	if p.AuthorUserID != userID {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("not the author"))
	}
	if p.Status != "draft" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("proposal is not a draft"))
	}

	upd := repository.ProposalUpdate{
		Title:                   req.Msg.Title,
		StatementMd:             req.Msg.StatementMd,
		TimeLimit:               req.Msg.TimeLimit,
		MemoryLimit:             req.Msg.MemoryLimit,
		ReferenceSolutionSource: req.Msg.ReferenceSolutionSource,
	}
	if req.Msg.ReferenceSolutionLanguage != nil {
		l := languageToString(*req.Msg.ReferenceSolutionLanguage)
		upd.ReferenceSolutionLanguage = &l
	}
	if len(req.Msg.Examples) > 0 {
		b, err := json.Marshal(req.Msg.Examples)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("marshal examples: %w", err))
		}
		s := string(b)
		upd.ExamplesJSON = &s
	}
	if len(req.Msg.TestcasesGz) > 0 {
		if !isValidGzippedTestcases(req.Msg.TestcasesGz) {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("testcases_gz is not a valid gzipped JSON array"))
		}
		upd.TestcasesGz = req.Msg.TestcasesGz
	}

	if err := h.repo.Update(ctx, req.Msg.ProposalId, upd); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("update proposal: %w", err))
	}
	p, err = h.repo.GetByID(ctx, req.Msg.ProposalId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&proposalv1.UpdateProposalResponse{Proposal: toProtoProposal(p)}), nil
}

func (h *ProposalServiceHandler) SubmitProposal(ctx context.Context, req *connect.Request[proposalv1.SubmitProposalRequest]) (*connect.Response[proposalv1.SubmitProposalResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("unauthenticated"))
	}
	p, err := h.repo.GetByID(ctx, req.Msg.ProposalId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if p == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("proposal not found"))
	}
	if p.AuthorUserID != userID {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("not the author"))
	}
	if p.Status != "draft" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("proposal is not a draft"))
	}
	if len(p.TestcasesGz) == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("testcases are required"))
	}
	if p.ReferenceSolutionSource == "" || p.ReferenceSolutionLanguage == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("reference solution is required"))
	}

	var examples []repository.ProposalExample
	if len(p.ExamplesJSON) > 0 {
		if err := json.Unmarshal(p.ExamplesJSON, &examples); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("examples malformed: %w", err))
		}
	}
	if len(examples) == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("at least one example is required"))
	}

	// mark reviewing while we validate
	_ = h.repo.UpdateStatus(ctx, p.ID, "reviewing", nil, nil)

	pairs := make([]*submissionv1.ExamplePair, len(examples))
	for i, ex := range examples {
		pairs[i] = &submissionv1.ExamplePair{Input: ex.Input, Output: ex.Output}
	}

	runReq := connect.NewRequest(&submissionv1.RunExamplesRequest{
		Language:    stringToLanguageEnum(p.ReferenceSolutionLanguage),
		SourceCode:  p.ReferenceSolutionSource,
		Examples:    pairs,
		TimeLimit:   p.TimeLimit,
		MemoryLimit: p.MemoryLimit,
	})
	runReq.Header().Set("X-User-Id", strconv.FormatInt(userID, 10))
	runResp, err := h.submitCli.RunExamples(ctx, runReq)
	if err != nil {
		_ = h.repo.UpdateStatus(ctx, p.ID, "draft", ptrStr("judge error: "+err.Error()), nil)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("run examples: %w", err))
	}

	validations := make([]repository.ProposalExampleValidation, len(runResp.Msg.TestCases))
	for i, tc := range runResp.Msg.TestCases {
		validations[i] = repository.ProposalExampleValidation{
			Index:           tc.Index,
			Passed:          tc.Passed,
			ExpectedOutput:  tc.ExpectedOutput,
			ActualOutput:    tc.ActualOutput,
			Result:          tc.Result,
			ExecutionTimeMs: tc.ExecutionTimeMs,
		}
	}
	validationJSON, _ := json.Marshal(validations)

	newStatus := "draft"
	var notes *string
	if runResp.Msg.Passed {
		newStatus = "submitted"
	} else {
		notes = ptrStr("reference solution failed on examples")
	}
	if err := h.repo.UpdateStatus(ctx, p.ID, newStatus, notes, validationJSON); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("update status: %w", err))
	}

	resp := &proposalv1.SubmitProposalResponse{
		Status:                  statusStringToEnum(newStatus),
		ExampleValidationResult: toProtoValidations(validations),
	}
	if notes != nil {
		resp.ReviewNotes = *notes
	}
	return connect.NewResponse(resp), nil
}

func (h *ProposalServiceHandler) GetProposal(ctx context.Context, req *connect.Request[proposalv1.GetProposalRequest]) (*connect.Response[proposalv1.GetProposalResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("unauthenticated"))
	}
	p, err := h.repo.GetByID(ctx, req.Msg.ProposalId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if p == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("proposal not found"))
	}
	if p.AuthorUserID != userID {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("not the author"))
	}
	return connect.NewResponse(&proposalv1.GetProposalResponse{Proposal: toProtoProposal(p)}), nil
}

func (h *ProposalServiceHandler) ListMyProposals(ctx context.Context, req *connect.Request[proposalv1.ListMyProposalsRequest]) (*connect.Response[proposalv1.ListMyProposalsResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("unauthenticated"))
	}
	limit := req.Msg.PageSize
	if limit == 0 {
		limit = 20
	}
	offset := int32(0)
	if req.Msg.PageToken != "" {
		if o, err := strconv.Atoi(req.Msg.PageToken); err == nil {
			offset = int32(o)
		}
	}
	rows, err := h.repo.ListByAuthor(ctx, userID, limit, offset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*proposalv1.Proposal, 0, len(rows))
	for _, p := range rows {
		pp := toProtoProposal(p)
		// strip large fields from list response
		pp.StatementMd = ""
		pp.ReferenceSolutionSource = ""
		out = append(out, pp)
	}
	next := ""
	if int32(len(rows)) == limit {
		next = strconv.Itoa(int(offset) + int(limit))
	}
	return connect.NewResponse(&proposalv1.ListMyProposalsResponse{Proposals: out, NextPageToken: next}), nil
}

// ---- helpers ----

func languageToString(l commonv1.Language) string {
	switch l {
	case commonv1.Language_LANGUAGE_CPP:
		return "LANGUAGE_CPP"
	case commonv1.Language_LANGUAGE_PYTHON:
		return "LANGUAGE_PYTHON"
	case commonv1.Language_LANGUAGE_JAVA:
		return "LANGUAGE_JAVA"
	case commonv1.Language_LANGUAGE_GO:
		return "LANGUAGE_GO"
	case commonv1.Language_LANGUAGE_JAVASCRIPT:
		return "LANGUAGE_JAVASCRIPT"
	case commonv1.Language_LANGUAGE_RUST:
		return "LANGUAGE_RUST"
	}
	return ""
}

func stringToLanguageEnum(s string) commonv1.Language {
	switch s {
	case "LANGUAGE_PYTHON":
		return commonv1.Language_LANGUAGE_PYTHON
	case "LANGUAGE_JAVA":
		return commonv1.Language_LANGUAGE_JAVA
	case "LANGUAGE_GO":
		return commonv1.Language_LANGUAGE_GO
	case "LANGUAGE_JAVASCRIPT":
		return commonv1.Language_LANGUAGE_JAVASCRIPT
	case "LANGUAGE_RUST":
		return commonv1.Language_LANGUAGE_RUST
	default:
		return commonv1.Language_LANGUAGE_CPP
	}
}

func statusStringToEnum(s string) proposalv1.ProposalStatus {
	switch s {
	case "draft":
		return proposalv1.ProposalStatus_PROPOSAL_STATUS_DRAFT
	case "submitted":
		return proposalv1.ProposalStatus_PROPOSAL_STATUS_SUBMITTED
	case "reviewing":
		return proposalv1.ProposalStatus_PROPOSAL_STATUS_REVIEWING
	case "approved":
		return proposalv1.ProposalStatus_PROPOSAL_STATUS_APPROVED
	case "rejected":
		return proposalv1.ProposalStatus_PROPOSAL_STATUS_REJECTED
	case "published":
		return proposalv1.ProposalStatus_PROPOSAL_STATUS_PUBLISHED
	}
	return proposalv1.ProposalStatus_PROPOSAL_STATUS_UNSPECIFIED
}

func toProtoProposal(p *repository.Proposal) *proposalv1.Proposal {
	out := &proposalv1.Proposal{
		Id:                        p.ID,
		AuthorUserId:              p.AuthorUserID,
		Title:                     p.Title,
		StatementMd:               p.StatementMd,
		TimeLimit:                 p.TimeLimit,
		MemoryLimit:               p.MemoryLimit,
		ReferenceSolutionLanguage: stringToLanguageEnum(p.ReferenceSolutionLanguage),
		ReferenceSolutionSource:   p.ReferenceSolutionSource,
		Status:                    statusStringToEnum(p.Status),
	}
	if p.CreatedAt != nil {
		out.CreatedAt = timestamppb.New(*p.CreatedAt)
	}
	if p.UpdatedAt != nil {
		out.UpdatedAt = timestamppb.New(*p.UpdatedAt)
	}
	if p.ReviewNotes != nil {
		out.ReviewNotes = *p.ReviewNotes
	}
	if p.PublishedProblemID != nil {
		out.PublishedProblemId = p.PublishedProblemID
	}

	var examples []repository.ProposalExample
	if len(p.ExamplesJSON) > 0 {
		_ = json.Unmarshal(p.ExamplesJSON, &examples)
	}
	for _, ex := range examples {
		out.Examples = append(out.Examples, &proposalv1.Example{Input: ex.Input, Output: ex.Output})
	}

	var vals []repository.ProposalExampleValidation
	if len(p.ExampleValidationResult) > 0 {
		_ = json.Unmarshal(p.ExampleValidationResult, &vals)
	}
	out.ExampleValidationResult = toProtoValidations(vals)
	return out
}

func toProtoValidations(vs []repository.ProposalExampleValidation) []*proposalv1.ExampleValidation {
	out := make([]*proposalv1.ExampleValidation, len(vs))
	for i, v := range vs {
		out[i] = &proposalv1.ExampleValidation{
			Index:           v.Index,
			Passed:          v.Passed,
			ExpectedOutput:  v.ExpectedOutput,
			ActualOutput:    v.ActualOutput,
			Result:          v.Result,
			ExecutionTimeMs: v.ExecutionTimeMs,
		}
	}
	return out
}

func ptrStr(s string) *string { return &s }

func isValidGzippedTestcases(b []byte) bool {
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return false
	}
	defer r.Close()
	dec := json.NewDecoder(io.LimitReader(r, 1<<30))
	tok, err := dec.Token()
	if err != nil {
		return false
	}
	d, ok := tok.(json.Delim)
	if !ok || d != '[' {
		return false
	}
	return true
}
