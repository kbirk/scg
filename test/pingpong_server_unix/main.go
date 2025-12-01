package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/rpc/unix"
	"github.com/kbirk/scg/test/files/output/pingpong"
)

const (
	socketPath = "/tmp/scg_test_unix.sock"
)

type pingpongServer struct {
}

func (s *pingpongServer) Ping(ctx context.Context, req *pingpong.PingRequest) (*pingpong.PongResponse, error) {
	js, _ := json.MarshalIndent(req.Ping, "", "  ")
	fmt.Println("Received ping:", string(js))
	return &pingpong.PongResponse{
		Pong: pingpong.Pong{
			Count:   req.Ping.Count + 1,
			Payload: req.Ping.Payload,
		},
	}, nil
}

func main() {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: unix.NewServerTransport(
			unix.ServerTransportConfig{
				SocketPath: socketPath,
			}),
		ErrHandler: func(err error) {
			fmt.Println("Server error handler:", err)
		},
	})
	pingpong.RegisterPingPongServer(server, &pingpongServer{})

	fmt.Println("Starting Unix socket server on", socketPath)
	err := server.ListenAndServe()
	if err != nil {
		fmt.Println(err)
	}
}
