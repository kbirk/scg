package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/rpc/nats"
	"github.com/kbirk/scg/test/files/output/pingpong"
)

const (
	natsURL = "nats://localhost:4222"
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
		Transport: nats.NewServerTransport(
			nats.ServerTransportConfig{
				URL: natsURL,
			}),
		ErrHandler: func(err error) {
			fmt.Println("Server error handler:", err)
		},
	})
	pingpong.RegisterPingPongServer(server, &pingpongServer{})

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nShutting down server...")
		server.Shutdown(context.Background())
	}()

	fmt.Printf("Starting NATS server on %s\n", natsURL)
	err := server.ListenAndServe()
	if err != nil {
		fmt.Println(err)
	}
}
