package test

import (
	"context"
	"fmt"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/test/files/output/pingpong"
)

var (
	validToken   = "1234"
	invalidToken = "5678"
)

func authMiddleware(ctx context.Context, req rpc.Message, next rpc.Handler) (rpc.Message, error) {
	md := rpc.GetMetadataFromContext(ctx)
	if md == nil {
		return nil, fmt.Errorf("no metadata")
	}

	token, ok, err := md.GetString("token")
	if err != nil || !ok {
		return nil, fmt.Errorf("no token")
	}

	if token != validToken {
		return nil, fmt.Errorf("invalid token")
	}

	return next(ctx, req)
}

func alwaysRejectMiddleware(ctx context.Context, req rpc.Message, next rpc.Handler) (rpc.Message, error) {
	return nil, fmt.Errorf("rejected")
}

type pingpongServer struct {
}

func (s *pingpongServer) Ping(ctx context.Context, req *pingpong.PingRequest) (*pingpong.PongResponse, error) {
	return &pingpong.PongResponse{
		Pong: pingpong.Pong{
			Count:   req.Ping.Count + 1,
			Payload: req.Ping.Payload,
		},
	}, nil
}

type pingpongServerFail struct {
}

func (s *pingpongServerFail) Ping(ctx context.Context, req *pingpong.PingRequest) (*pingpong.PongResponse, error) {
	return nil, fmt.Errorf("unable to ping the pong")
}
