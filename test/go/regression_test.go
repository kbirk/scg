package test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/rpc/tcp"
	"github.com/kbirk/scg/pkg/rpc/websocket"
	"github.com/kbirk/scg/test/scg/generated/pingpong"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResponseExceedsServerSendLimit: a response the server cannot send (over
// its MaxSendMessageSize) is replaced by an error response rather than leaving
// the caller waiting forever.
func TestResponseExceedsServerSendLimit(t *testing.T) {
	cases := []struct {
		name   string
		port   int
		server func(port int) rpc.ServerTransport
		client func(port int) rpc.ClientTransport
	}{
		{"tcp", 18850,
			func(p int) rpc.ServerTransport {
				return tcp.NewServerTransport(tcp.ServerTransportConfig{Port: p, MaxSendMessageSize: 512})
			},
			func(p int) rpc.ClientTransport {
				return tcp.NewClientTransport(tcp.ClientTransportConfig{Host: "127.0.0.1", Port: p})
			}},
		{"ws", 18851,
			func(p int) rpc.ServerTransport {
				return websocket.NewServerTransport(websocket.ServerTransportConfig{Port: p, MaxSendMessageSize: 512})
			},
			func(p int) rpc.ClientTransport {
				return websocket.NewClientTransport(websocket.ClientTransportConfig{Host: "127.0.0.1", Port: p})
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := rpc.NewServer(rpc.ServerConfig{Transport: tc.server(tc.port), ErrHandler: func(error) {}})
			pingpong.RegisterPingPongServer(server, &pingpongServer{})
			go server.ListenAndServe()
			waitForPort(t, tc.port)
			defer server.Shutdown(context.Background())

			client := rpc.NewClient(rpc.ClientConfig{Transport: tc.client(tc.port)})
			defer client.Close()
			pp := pingpong.NewPingPongClient(client)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			// The echoed payload makes the response exceed the 512-byte limit.
			big := pingpong.PingRequest{Ping: pingpong.Ping{Payload: pingpong.TestPayload{ValString: string(make([]byte, 4096))}}}
			_, err := pp.Ping(ctx, &big)
			require.Error(t, err)
			assert.NotErrorIs(t, err, context.DeadlineExceeded, "the caller must get an error response, not a hang")
			assert.Contains(t, err.Error(), "exceeds send limit")

			// Small responses still work on the same connection.
			_, err = pp.Ping(ctx, &pingpong.PingRequest{})
			require.NoError(t, err)
		})
	}
}

// TestConcurrentCallsAcrossReconnect hammers one client from many goroutines
// while its connection is repeatedly closed, exercising the dial singleflight,
// generation-guarded teardown and request registry under -race. Every call
// must either succeed or fail promptly; none may hang.
func TestConcurrentCallsAcrossReconnect(t *testing.T) {
	const port = 18852
	server := rpc.NewServer(rpc.ServerConfig{Transport: tcp.NewServerTransport(tcp.ServerTransportConfig{Port: port})})
	pingpong.RegisterPingPongServer(server, &pingpongServer{})
	go server.ListenAndServe()
	waitForPort(t, port)
	defer server.Shutdown(context.Background())

	client := rpc.NewClient(rpc.ClientConfig{
		Transport:  tcp.NewClientTransport(tcp.ClientTransportConfig{Host: "127.0.0.1", Port: port}),
		ErrHandler: func(error) {},
	})
	defer client.Close()
	pp := pingpong.NewPingPongClient(client)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	var ok, failed atomic.Int64
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				resp, err := pp.Ping(ctx, &pingpong.PingRequest{Ping: pingpong.Ping{Count: int32(i)}})
				cancel()
				if err != nil {
					if errors.Is(err, context.DeadlineExceeded) {
						t.Errorf("call hung until its deadline: %v", err)
						return
					}
					failed.Add(1)
					continue
				}
				if resp.Pong.Count != int32(i)+1 {
					t.Errorf("goroutine %d: got count %d want %d", g, resp.Pong.Count, i+1)
					return
				}
				ok.Add(1)
			}
		}(g)
	}
	for i := 0; i < 20; i++ {
		time.Sleep(15 * time.Millisecond)
		client.Close()
	}
	close(stop)
	wg.Wait()
	t.Logf("ok=%d failed(during close)=%d", ok.Load(), failed.Load())
	assert.Greater(t, ok.Load(), int64(100))
}
