package handler

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	submissionv1 "github.com/puri-cp/puri/gen/submission/v1"
)

func (h *SubmissionServiceHandler) RunExamples(ctx context.Context, req *connect.Request[submissionv1.RunExamplesRequest]) (*connect.Response[submissionv1.RunExamplesResponse], error) {
	if len(req.Msg.Examples) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("no examples provided"))
	}

	tl := req.Msg.TimeLimit
	if tl == "" {
		tl = "1초"
	}
	ml := req.Msg.MemoryLimit
	if ml == "" {
		ml = "128MB"
	}

	pairs := make([]examplePair, len(req.Msg.Examples))
	for i, ex := range req.Msg.Examples {
		pairs[i] = examplePair{Input: ex.Input, Output: ex.Output}
	}

	out := runAgainstExamples(ctx, h.runner, languageToString(req.Msg.Language), req.Msg.SourceCode, pairs, tl, ml)
	if out.Internal != nil {
		return nil, connect.NewError(connect.CodeInternal, out.Internal)
	}

	return connect.NewResponse(&submissionv1.RunExamplesResponse{
		Passed:    out.Passed,
		TestCases: out.TestCases,
	}), nil
}
