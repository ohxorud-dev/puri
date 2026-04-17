package interceptor

import (
	"context"
	"log"
	"time"

	"connectrpc.com/connect"
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
