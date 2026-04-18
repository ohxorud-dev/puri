package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"connectrpc.com/connect"

	submissionv1 "github.com/puri-cp/puri/gen/submission/v1"
	submissionv1connect "github.com/puri-cp/puri/gen/submission/v1/submissionv1connect"
	"github.com/puri-cp/puri/services/api/auth"
	"github.com/puri-cp/puri/services/api/repository"
)

type SubmissionServiceHandler struct {
	client   submissionv1connect.SubmissionServiceClient
	userRepo repository.UserRepo
}

func NewSubmissionServiceHandler(baseURL string, userRepo repository.UserRepo) *SubmissionServiceHandler {
	return &SubmissionServiceHandler{
		client:   submissionv1connect.NewSubmissionServiceClient(http.DefaultClient, baseURL),
		userRepo: userRepo,
	}
}

func (h *SubmissionServiceHandler) viewerIsAdmin(ctx context.Context) bool {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return false
	}
	user, err := h.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return false
	}
	return user.Role == "admin"
}

func (h *SubmissionServiceHandler) CreateSubmission(ctx context.Context, req *connect.Request[submissionv1.CreateSubmissionRequest]) (*connect.Response[submissionv1.CreateSubmissionResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("unauthenticated"))
	}
	outReq := connect.NewRequest(req.Msg)
	outReq.Header().Set("X-User-Id", strconv.FormatInt(userID, 10))
	return h.client.CreateSubmission(ctx, outReq)
}

func (h *SubmissionServiceHandler) RunTest(ctx context.Context, req *connect.Request[submissionv1.RunTestRequest]) (*connect.Response[submissionv1.RunTestResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("unauthenticated"))
	}
	outReq := connect.NewRequest(req.Msg)
	outReq.Header().Set("X-User-Id", strconv.FormatInt(userID, 10))
	return h.client.RunTest(ctx, outReq)
}

func (h *SubmissionServiceHandler) RunExamples(ctx context.Context, req *connect.Request[submissionv1.RunExamplesRequest]) (*connect.Response[submissionv1.RunExamplesResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("unauthenticated"))
	}
	outReq := connect.NewRequest(req.Msg)
	outReq.Header().Set("X-User-Id", strconv.FormatInt(userID, 10))
	return h.client.RunExamples(ctx, outReq)
}

func (h *SubmissionServiceHandler) GetSubmission(ctx context.Context, req *connect.Request[submissionv1.GetSubmissionRequest]) (*connect.Response[submissionv1.GetSubmissionResponse], error) {
	resp, err := h.client.GetSubmission(ctx, req)
	if err != nil {
		return nil, err
	}
	viewerID, _ := auth.UserIDFromContext(ctx)
	isAdmin := h.viewerIsAdmin(ctx)
	if s := resp.Msg.Submission; s != nil && !isAdmin && viewerID != s.UserId {
		s.SourceCode = ""
	}
	return resp, nil
}

func (h *SubmissionServiceHandler) ListSubmissions(ctx context.Context, req *connect.Request[submissionv1.ListSubmissionsRequest]) (*connect.Response[submissionv1.ListSubmissionsResponse], error) {
	resp, err := h.client.ListSubmissions(ctx, req)
	if err != nil {
		return nil, err
	}
	viewerID, _ := auth.UserIDFromContext(ctx)
	isAdmin := h.viewerIsAdmin(ctx)
	if !isAdmin {
		for _, s := range resp.Msg.Submissions {
			if s.UserId != viewerID {
				s.SourceCode = ""
			}
		}
	}
	return resp, nil
}

func (h *SubmissionServiceHandler) StreamSubmissionStatus(ctx context.Context, req *connect.Request[submissionv1.StreamSubmissionStatusRequest], stream *connect.ServerStream[submissionv1.StreamSubmissionStatusResponse]) error {
	clientStream, err := h.client.StreamSubmissionStatus(ctx, req)
	if err != nil {
		return err
	}
	defer clientStream.Close()

	for clientStream.Receive() {
		if err := stream.Send(clientStream.Msg()); err != nil {
			return err
		}
	}

	if err := clientStream.Err(); err != nil {
		return err
	}
	return nil
}
