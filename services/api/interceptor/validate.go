package interceptor

import (
	"context"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
)

type ValidationInterceptor struct {
	validator protovalidate.Validator
}

func NewValidationInterceptor() (*ValidationInterceptor, error) {
	v, err := protovalidate.New()
	if err != nil {
		return nil, err
	}
	return &ValidationInterceptor{validator: v}, nil
}

func (i *ValidationInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if msg, ok := req.Any().(proto.Message); ok {
			if err := i.validator.Validate(msg); err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
		}
		return next(ctx, req)
	}
}

func (i *ValidationInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *ValidationInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		return next(ctx, conn)
	}
}
