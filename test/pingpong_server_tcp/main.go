package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/rpc/tcp"
	"github.com/kbirk/scg/test/files/output/pingpong"
)

const (
	port = 9001
)

type pingpongServer struct {
}

func (s *pingpongServer) Ping(ctx context.Context, req *pingpong.PingRequest) (*pingpong.PongResponse, error) {
	md := rpc.GetMetadataFromContext(ctx)
	if md != nil {
		if sleepStr, ok, _ := md.GetString("sleep"); ok {
			if sleepMs, err := strconv.Atoi(sleepStr); err == nil && sleepMs > 0 {
				time.Sleep(time.Duration(sleepMs) * time.Millisecond)
			}
		}
	}

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
		Transport: tcp.NewServerTransport(
			tcp.ServerTransportConfig{
				Port: port,
			}),
		ErrHandler: func(err error) {
			fmt.Println("Server error handler:", err)
		},
	})
	pingpong.RegisterPingPongServer(server, &pingpongServer{})

	fmt.Println("Starting TCP server on port", port)
	err := server.ListenAndServe()
	if err != nil {
		fmt.Println(err)
	}
}
