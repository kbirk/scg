package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/rpc/websocket"
	"github.com/kbirk/scg/test/files/output/pingpong"
)

const (
	port = 8001
)

var (
	certFile string
	keyFile  string
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
	flag.StringVar(&certFile, "cert", "", "TLS certificate file")
	flag.StringVar(&keyFile, "key", "", "TLS key file")
	flag.Parse()

	if certFile == "" || keyFile == "" {
		fmt.Println("Usage: pingpong_server_ws_tls --cert=<cert_file> --key=<key_file>")
		os.Exit(1)
	}

	server := rpc.NewServer(rpc.ServerConfig{
		Transport: websocket.NewServerTransport(
			websocket.ServerTransportConfig{
				Port:     port,
				CertFile: certFile,
				KeyFile:  keyFile,
			}),
		ErrHandler: func(err error) {
			fmt.Println("Server error handler:", err)
		},
	})
	pingpong.RegisterPingPongServer(server, &pingpongServer{})

	fmt.Println("Starting TLS server on port", port)
	err := server.ListenAndServe()
	if err != nil {
		fmt.Println(err)
	}
}
