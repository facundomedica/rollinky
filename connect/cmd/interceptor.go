//go:build !no_tee
// +build !no_tee

package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/edgelesssys/ego/enclave"
	protov1 "github.com/golang/protobuf/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func UnaryInterceptor(logger *zap.Logger) func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			return resp, err
		}

		protoresp := resp.(protov1.Message)
		bz, err := protov1.Marshal(protoresp)
		if err != nil {
			return nil, err
		}

		since := time.Now()
		hash := sha256.Sum256(bz)
		// TODO: if this endpoint is queried a lot (if it's being used for something
		// else other than creating blocks), we should cache the report.
		report, err := enclave.GetRemoteReport(hash[:])
		if err != nil {
			fmt.Println(err)
		}
		trailer := metadata.Pairs(
			"X-Enclave-Report", base64.RawStdEncoding.EncodeToString(report),
		)
		grpc.SetTrailer(ctx, trailer)
		logger.Debug("created report", zap.Duration("time", time.Since(since)))
		return resp, err
	}
}
