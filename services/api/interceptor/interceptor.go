package interceptor

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	commonv1 "github.com/puri-cp/puri/gen/common/v1"
	"github.com/puri-cp/puri/services/api/auth"
)

type LoggingInterceptor struct{}

func NewLoggingInterceptor() *LoggingInterceptor {
	return &LoggingInterceptor{}
}

func (i *LoggingInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		start := time.Now()
		resp, err := next(ctx, req)
		duration := time.Since(start)
		if err != nil {
			log.Printf("[ERROR] %s duration=%s error=%v", req.Spec().Procedure, duration, err)
		} else {
			log.Printf("[INFO] %s duration=%s", req.Spec().Procedure, duration)
		}
		return resp, err
	}
}

func (i *LoggingInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *LoggingInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		start := time.Now()
		err := next(ctx, conn)
		duration := time.Since(start)
		if err != nil {
			log.Printf("[ERROR] %s (stream) duration=%s error=%v", conn.Spec().Procedure, duration, err)
		} else {
			log.Printf("[INFO] %s (stream) duration=%s", conn.Spec().Procedure, duration)
		}
		return err
	}
}

type AuthInterceptor struct {
	secret string
}

func NewAuthInterceptor(secret string) *AuthInterceptor {
	return &AuthInterceptor{secret: secret}
}

func (i *AuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		ctx, err := i.authenticate(ctx, req.Spec(), req.Header())
		if err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (i *AuthInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *AuthInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		ctx, err := i.authenticate(ctx, conn.Spec(), conn.RequestHeader())
		if err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

func (i *AuthInterceptor) authenticate(ctx context.Context, spec connect.Spec, header http.Header) (context.Context, error) {
	methodDesc, ok := spec.Schema.(protoreflect.MethodDescriptor)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("invalid method descriptor"))
	}
	options, ok := methodDesc.Options().(*descriptorpb.MethodOptions)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("invalid method options"))
	}
	ext := proto.GetExtension(options, commonv1.E_EndpointRule)
	endpoint, ok := ext.(*commonv1.EndpointSecurity)
	if !ok || endpoint == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("missing endpoint rule"))
	}

	if endpoint.IsPublic {
		return ctx, nil
	}

	userID, err := auth.VerifyTokenFromHeader(header, i.secret)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication failed: %w", err))
	}

	return auth.WithUserID(ctx, userID), nil
}
