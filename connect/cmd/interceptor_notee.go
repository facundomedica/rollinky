//go:build no_tee
// +build no_tee

package main

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func UnaryInterceptor(logger *zap.Logger) func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		logger.Debug("no report created, not running in a TEE")
		return handler(ctx, req)
	}
}
