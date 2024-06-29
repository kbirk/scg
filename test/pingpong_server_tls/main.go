package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/test/files/output/pingpong"
)

const (
	port = 8000
)

var (
	certFile   string
	keyFile    string
	validToken = "1234"
)

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

func authMiddleware(ctx context.Context, req rpc.Message, next rpc.Handler) (rpc.Message, error) {
	md := rpc.GetMetadataFromContext(ctx)
	if md == nil {
		return nil, fmt.Errorf("no metadata")
	}

	token, ok := md["token"]
	if !ok {
		return nil, fmt.Errorf("no token")
	}

	if token != validToken {
		return nil, fmt.Errorf("invalid token")
	}

	return next(ctx, req)
}

func main() {

	flag.StringVar(&certFile, "cert", "", "Cert file")
	flag.StringVar(&keyFile, "key", "", "Key file")

	flag.Parse()

	if certFile == "" {
		os.Stderr.WriteString("No `--cert` argument provided\n")
		os.Exit(1)
	}

	if keyFile == "" {
		os.Stderr.WriteString("No `--key` argument provided\n")
		os.Exit(1)
	}

	server := rpc.NewServer(rpc.ServerConfig{
		Port: port,
		ErrHandler: func(err error) {
			fmt.Println("Server error handler:", err)
		},
	})
	server.Middleware(authMiddleware)
	pingpong.RegisterPingPongServer(server, &pingpongServer{})

	err := server.ListenAndServeTLS(certFile, keyFile)
	if err != nil {
		fmt.Println(err)
	}
}
