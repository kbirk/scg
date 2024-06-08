package test

import (
	"context"
	"crypto/tls"
	"fmt"
	"testing"
	"time"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/test/files/output/pingpong"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pingpongServer struct {
}

func (s *pingpongServer) Ping(ctx context.Context, req *pingpong.PingRequest) (*pingpong.PongResponse, error) {
	return &pingpong.PongResponse{
		Pong: pingpong.Pong{
			Count: req.Ping.Count + 1,
		},
	}, nil
}

type pingpongServerFail struct {
}

func (s *pingpongServerFail) Ping(ctx context.Context, req *pingpong.PingRequest) (*pingpong.PongResponse, error) {
	return nil, fmt.Errorf("unable to ping the pong")
}

func TestPingPong(t *testing.T) {

	server := rpc.NewServer(rpc.ServerConfig{
		Port: 8080,
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})
	pingpong.RegisterPingPongServer(server, &pingpongServer{})

	go func() {
		server.ListenAndServe()
	}()

	client := rpc.NewClient(rpc.ClientConfig{
		Host: "localhost",
		Port: 8080,
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})

	c := pingpong.NewPingPongClient(client)

	count := int32(0)

	for {
		resp, err := c.Ping(context.Background(), &pingpong.PingRequest{
			Ping: pingpong.Ping{
				Count: count,
			},
		})
		require.NoError(t, err)

		assert.Equal(t, count+1, resp.Pong.Count)
		count = resp.Pong.Count

		if count > 10 {
			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	err := server.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestPingPongTLS(t *testing.T) {
	server := rpc.NewServer(rpc.ServerConfig{
		Port: 8080,
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})
	pingpong.RegisterPingPongServer(server, &pingpongServer{})

	go func() {
		server.ListenAndServeTLS("../server.crt", "../server.key")
	}()

	client := rpc.NewClient(rpc.ClientConfig{
		Host: "localhost",
		Port: 8080,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true, // self signed
		},
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})

	c := pingpong.NewPingPongClient(client)

	count := int32(0)

	for {
		resp, err := c.Ping(context.Background(), &pingpong.PingRequest{
			Ping: pingpong.Ping{
				Count: count,
			},
		})
		require.NoError(t, err)

		assert.Equal(t, count+1, resp.Pong.Count)
		count = resp.Pong.Count

		if count > 10 {
			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	err := server.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestPingPongFail(t *testing.T) {
	server := rpc.NewServer(rpc.ServerConfig{
		Port: 8080,
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})
	pingpong.RegisterPingPongServer(server, &pingpongServerFail{})

	go func() {
		server.ListenAndServe()
	}()

	client := rpc.NewClient(rpc.ClientConfig{
		Host: "localhost",
		Port: 8080,
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})

	c := pingpong.NewPingPongClient(client)

	_, err := c.Ping(context.Background(), &pingpong.PingRequest{
		Ping: pingpong.Ping{
			Count: 1,
		},
	})
	assert.Error(t, err)
	assert.Equal(t, "unable to ping the pong", err.Error())

	err = server.Shutdown(context.Background())
	require.NoError(t, err)
}
