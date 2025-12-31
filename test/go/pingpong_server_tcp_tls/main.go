package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/rpc/tcp"
	"github.com/kbirk/scg/test/scg/generated/pingpong"
)

const (
	port = 9002
)

var (
	certFile string
	keyFile  string
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
		fmt.Println("Usage: pingpong_server_tcp_tls --cert=<cert_file> --key=<key_file>")
		os.Exit(1)
	}

	server := rpc.NewServer(rpc.ServerConfig{
		Transport: tcp.NewServerTransportTLS(
			tcp.ServerTransportTLSConfig{
				Port:     port,
				CertFile: certFile,
				KeyFile:  keyFile,
			}),
		ErrHandler: func(err error) {
			fmt.Println("Server error handler:", err)
		},
	})
	pingpong.RegisterPingPongServer(server, &pingpongServer{})

	fmt.Println("Starting TCP TLS server on port", port)
	err := server.ListenAndServe()
	if err != nil {
		fmt.Println(err)
	}
}
