package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "github.com/ohxorud-dev/puri/gen/go/common/v1"
	submissionv1 "github.com/ohxorud-dev/puri/gen/go/submission/v1"
	"github.com/ohxorud-dev/puri/services/submission/executor"
	"github.com/ohxorud-dev/puri/services/submission/problem"
	"github.com/ohxorud-dev/puri/services/submission/queue"
	"github.com/ohxorud-dev/puri/services/submission/repository"
)

type SubmissionServiceHandler struct {
	repo         *repository.SubmissionRepository
	runner       *executor.DockerRunner
	publisher    *queue.Publisher
	problemsPath string
}

func NewSubmissionServiceHandler(repo *repository.SubmissionRepository, runner *executor.DockerRunner, publisher *queue.Publisher, problemsPath string) *SubmissionServiceHandler {
	return &SubmissionServiceHandler{
		repo:         repo,
		runner:       runner,
		publisher:    publisher,
		problemsPath: problemsPath,
	}
}

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
	default:
		return "LANGUAGE_CPP"
	}
}

func (h *SubmissionServiceHandler) CreateSubmission(ctx context.Context, req *connect.Request[submissionv1.CreateSubmissionRequest]) (*connect.Response[submissionv1.CreateSubmissionResponse], error) {
	userID, err := userIDFromHeader(req.Header())
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("missing user id"))
	}

	sub, err := h.repo.Create(ctx, userID, req.Msg.ProblemId, languageToString(req.Msg.Language), req.Msg.SourceCode)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create submission"))
	}

	job := queue.JobMessage{
		SubmissionID: sub.ID,
		ProblemID:    req.Msg.ProblemId,
		Language:     languageToString(req.Msg.Language),
		SourceCode:   req.Msg.SourceCode,
	}
	if err := h.publisher.Publish(ctx, job); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to queue submission"))
	}

	return connect.NewResponse(&submissionv1.CreateSubmissionResponse{Submission: toProtoSubmission(sub)}), nil
}

func (h *SubmissionServiceHandler) RunTest(ctx context.Context, req *connect.Request[submissionv1.RunTestRequest]) (*connect.Response[submissionv1.RunTestResponse], error) {
	meta, err := problem.LoadMetadata(h.problemsPath, req.Msg.ProblemId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("problem not found: %w", err))
	}
	if len(meta.Examples) == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("no example test cases found"))
	}

	pairs := make([]examplePair, len(meta.Examples))
	for i, ex := range meta.Examples {
		pairs[i] = examplePair{Input: ex.Input, Output: ex.Output}
	}

	out := runAgainstExamples(ctx, h.runner, languageToString(req.Msg.Language), req.Msg.SourceCode, pairs, meta.TimeLimit, meta.MemoryLimit)
	if out.Internal != nil {
		return nil, connect.NewError(connect.CodeInternal, out.Internal)
	}

	return connect.NewResponse(&submissionv1.RunTestResponse{
		Passed:    out.Passed,
		Result:    out.Overall,
		TestCases: out.TestCases,
	}), nil
}

func (h *SubmissionServiceHandler) GetSubmission(ctx context.Context, req *connect.Request[submissionv1.GetSubmissionRequest]) (*connect.Response[submissionv1.GetSubmissionResponse], error) {
	sub, err := h.repo.GetByID(ctx, req.Msg.SubmissionId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("database error"))
	}
	if sub == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("submission not found"))
	}
	return connect.NewResponse(&submissionv1.GetSubmissionResponse{Submission: toProtoSubmission(sub)}), nil
}

func (h *SubmissionServiceHandler) ListSubmissions(ctx context.Context, req *connect.Request[submissionv1.ListSubmissionsRequest]) (*connect.Response[submissionv1.ListSubmissionsResponse], error) {
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

	var userID *int64
	var problemID *int32
	if req.Msg.UserId != nil {
		v := *req.Msg.UserId
		userID = &v
	}
	if req.Msg.ProblemId != nil {
		v := *req.Msg.ProblemId
		problemID = &v
	}

	subs, err := h.repo.List(ctx, userID, problemID, limit, offset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("database error"))
	}

	titleCache := make(map[int32]string)
	var protoSubs []*submissionv1.Submission
	for _, s := range subs {
		ps := toProtoSubmission(s)
		if title, ok := titleCache[s.ProblemID]; ok {
			ps.ProblemTitle = title
		} else if meta, err := problem.LoadMetadata(h.problemsPath, s.ProblemID); err == nil {
			titleCache[s.ProblemID] = meta.Title
			ps.ProblemTitle = meta.Title
		}
		protoSubs = append(protoSubs, ps)
	}

	nextPageToken := ""
	if len(subs) == int(limit) {
		nextPageToken = strconv.Itoa(int(offset) + int(limit))
	}

	return connect.NewResponse(&submissionv1.ListSubmissionsResponse{Submissions: protoSubs, NextPageToken: nextPageToken}), nil
}

func (h *SubmissionServiceHandler) StreamSubmissionStatus(ctx context.Context, req *connect.Request[submissionv1.StreamSubmissionStatusRequest], stream *connect.ServerStream[submissionv1.StreamSubmissionStatusResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, fmt.Errorf("StreamSubmissionStatus is not implemented yet"))
}

func (h *SubmissionServiceHandler) DeleteSubmission(ctx context.Context, req *connect.Request[submissionv1.DeleteSubmissionRequest]) (*connect.Response[submissionv1.DeleteSubmissionResponse], error) {
	ok, err := h.repo.Delete(ctx, req.Msg.SubmissionId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("database error"))
	}
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("submission not found"))
	}
	return connect.NewResponse(&submissionv1.DeleteSubmissionResponse{}), nil
}

func toProtoSubmission(s *repository.Submission) *submissionv1.Submission {
	sub := &submissionv1.Submission{
		Id:         s.ID,
		UserId:     s.UserID,
		Username:   s.Username,
		ProblemId:  s.ProblemID,
		Language:   stringToLanguage(s.Language),
		SourceCode: s.SourceCode,
		Status:     stringToStatus(s.Status),
		CreatedAt:  timestamppb.Now(),
	}
	if s.Result != nil {
		sub.Result = *s.Result
	}
	if s.ExecutionTimeMs != nil {
		sub.ExecutionTimeMs = *s.ExecutionTimeMs
	}
	if s.MemoryUsageKb != nil {
		sub.MemoryUsageKb = *s.MemoryUsageKb
	}
	return sub
}

func stringToLanguage(s string) commonv1.Language {
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

func stringToStatus(s string) commonv1.SubmissionStatus {
	switch s {
	case "JUDGING":
		return commonv1.SubmissionStatus_SUBMISSION_STATUS_JUDGING
	case "ACCEPTED":
		return commonv1.SubmissionStatus_SUBMISSION_STATUS_ACCEPTED
	case "WRONG_ANSWER":
		return commonv1.SubmissionStatus_SUBMISSION_STATUS_WRONG_ANSWER
	case "TIME_LIMIT_EXCEEDED":
		return commonv1.SubmissionStatus_SUBMISSION_STATUS_TIME_LIMIT_EXCEEDED
	case "RUNTIME_ERROR":
		return commonv1.SubmissionStatus_SUBMISSION_STATUS_RUNTIME_ERROR
	case "MEMORY_LIMIT_EXCEEDED":
		return commonv1.SubmissionStatus_SUBMISSION_STATUS_MEMORY_LIMIT_EXCEEDED
	case "COMPILATION_ERROR":
		return commonv1.SubmissionStatus_SUBMISSION_STATUS_COMPILATION_ERROR
	default:
		return commonv1.SubmissionStatus_SUBMISSION_STATUS_PENDING
	}
}

func normalizeOutput(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
}

func parseDuration(limitStr string) time.Duration {
	limitStr = strings.TrimSpace(limitStr)
	limitStr = strings.ReplaceAll(limitStr, "초", "")
	limitStr = strings.TrimSpace(limitStr)
	if sec, err := strconv.ParseFloat(limitStr, 64); err == nil {
		return time.Duration(sec) * time.Second
	}
	return 3 * time.Second
}

func userIDFromHeader(header http.Header) (int64, error) {
	cookies := header.Values("Cookie")
	for _, c := range cookies {
		parsed, err := http.ParseCookie(c)
		if err != nil {
			continue
		}
		for _, cookie := range parsed {
			if cookie.Name == "puri_session" {
				return 0, fmt.Errorf("cookie found but jwt verification not implemented in submission service")
			}
		}
	}

	userIDs := header.Values("X-User-Id")
	if len(userIDs) > 0 {
		return strconv.ParseInt(userIDs[0], 10, 64)
	}
	return 0, fmt.Errorf("user id not found")
}
