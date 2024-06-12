package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/test/files/output/pingpong"
)

const (
	port = 8080
)

var (
	certFile string
	keyFile  string
)

type pingpongServer struct {
}

func (s *pingpongServer) Ping(ctx context.Context, req *pingpong.PingRequest) (*pingpong.PongResponse, error) {
	return &pingpong.PongResponse{
		Pong: pingpong.Pong{
			Count:     req.Ping.Count + 1,
			Timestamp: time.Now(),
		},
	}, nil
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
	pingpong.RegisterPingPongServer(server, &pingpongServer{})

	err := server.ListenAndServeTLS(certFile, keyFile)
	if err != nil {
		fmt.Println(err)
	}
}
