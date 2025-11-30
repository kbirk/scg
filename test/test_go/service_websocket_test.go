package test

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/rpc/websocket"
	"github.com/kbirk/scg/test/files/output/basic"
	"github.com/kbirk/scg/test/files/output/pingpong"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPingPong(t *testing.T) {

	server := rpc.NewServer(rpc.ServerConfig{
		Transport: websocket.NewServerTransport(
			websocket.ServerTransportConfig{
				Port: 8000,
			}),
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})
	pingpong.RegisterPingPongServer(server, &pingpongServer{})

	go func() {
		server.ListenAndServe()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: websocket.NewClientTransport(
			websocket.ClientTransportConfig{
				Host: "localhost",
				Port: 8000,
			}),
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})

	middlewareCount := 0
	client.Middleware(func(ctx context.Context, req rpc.Message, next rpc.Handler) (rpc.Message, error) {
		middlewareCount++
		return next(ctx, req)
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

	assert.Equal(t, 11, middlewareCount)

	err := server.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestPingPongConcurrent(t *testing.T) {

	server := rpc.NewServer(rpc.ServerConfig{
		Transport: websocket.NewServerTransport(
			websocket.ServerTransportConfig{
				Port: 8001,
			}),
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})
	pingpong.RegisterPingPongServer(server, &pingpongServer{})

	go func() {
		server.ListenAndServe()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: websocket.NewClientTransport(
			websocket.ClientTransportConfig{
				Host: "localhost",
				Port: 8001,
			}),
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})

	var middlewareCount int32
	client.Middleware(func(ctx context.Context, req rpc.Message, next rpc.Handler) (rpc.Message, error) {
		atomic.AddInt32(&middlewareCount, 1)
		return next(ctx, req)
	})

	c := pingpong.NewPingPongClient(client)

	numGoRoutines := 32
	numIterations := 32
	wg := &sync.WaitGroup{}
	for i := 0; i < numGoRoutines; i++ {
		wg.Add(1)
		go func() {
			count := int32(0)
			for j := 0; j < numIterations; j++ {
				resp, err := c.Ping(context.Background(), &pingpong.PingRequest{
					Ping: pingpong.Ping{
						Count: count,
					},
				})
				require.NoError(t, err)

				assert.Equal(t, count+1, resp.Pong.Count)
				count = resp.Pong.Count

				time.Sleep(50 * time.Millisecond)
			}
			wg.Done()
		}()
	}

	wg.Wait()

	assert.Equal(t, int32(numGoRoutines*numIterations), middlewareCount)

	err := server.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestPingPongTLS(t *testing.T) {

	server := rpc.NewServer(rpc.ServerConfig{
		Transport: websocket.NewServerTransport(websocket.ServerTransportConfig{
			Port:     8002,
			CertFile: "../server.crt",
			KeyFile:  "../server.key",
		}),
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})
	pingpong.RegisterPingPongServer(server, &pingpongServer{})

	go func() {
		server.ListenAndServe()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: websocket.NewClientTransport(websocket.ClientTransportConfig{
			Host: "localhost",
			Port: 8002,
			TLSConfig: &tls.Config{
				InsecureSkipVerify: true, // self signed
			},
		}),
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

func TestPingPongTLSAndAuth(t *testing.T) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: websocket.NewServerTransport(websocket.ServerTransportConfig{
			Port:     8003,
			CertFile: "../server.crt",
			KeyFile:  "../server.key",
		}),
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})
	server.Middleware(authMiddleware)
	pingpong.RegisterPingPongServer(server, &pingpongServer{})

	go func() {
		server.ListenAndServe()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: websocket.NewClientTransport(websocket.ClientTransportConfig{
			Host: "localhost",
			Port: 8003,
			TLSConfig: &tls.Config{
				InsecureSkipVerify: true, // self signed
			},
		}),
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})

	c := pingpong.NewPingPongClient(client)

	count := int32(0)

	md := rpc.NewMetadata()
	md.PutString("token", validToken)

	ctx := rpc.NewContextWithMetadata(context.Background(), md)

	for {
		resp, err := c.Ping(ctx, &pingpong.PingRequest{
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
		Transport: websocket.NewServerTransport(
			websocket.ServerTransportConfig{
				Port: 8004,
			}),
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})
	pingpong.RegisterPingPongServer(server, &pingpongServerFail{})

	go func() {
		server.ListenAndServe()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: websocket.NewClientTransport(
			websocket.ClientTransportConfig{
				Host: "localhost",
				Port: 8004,
			}),
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

func TestPingPongAuthFail(t *testing.T) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: websocket.NewServerTransport(
			websocket.ServerTransportConfig{
				Port: 8005,
			}),
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})
	server.Middleware(authMiddleware)
	pingpong.RegisterPingPongServer(server, &pingpongServerFail{})

	go func() {
		server.ListenAndServe()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: websocket.NewClientTransport(
			websocket.ClientTransportConfig{
				Host: "localhost",
				Port: 8005,
			}),
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})

	c := pingpong.NewPingPongClient(client)

	md := rpc.NewMetadata()
	md.PutString("token", invalidToken)

	ctx := rpc.NewContextWithMetadata(context.Background(), md)

	_, err := c.Ping(ctx, &pingpong.PingRequest{
		Ping: pingpong.Ping{
			Count: 1,
		},
	})
	assert.Error(t, err)
	assert.Equal(t, "invalid token", err.Error())

	err = server.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestPingPongTLSWithGroupsAndAuth(t *testing.T) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: websocket.NewServerTransport(
			websocket.ServerTransportConfig{
				Port:     8006,
				CertFile: "../server.crt",
				KeyFile:  "../server.key",
			}),
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})
	server.Group(func(s *rpc.Server) {
		server.Middleware(authMiddleware)
		pingpong.RegisterPingPongServer(server, &pingpongServer{})
	})

	go func() {
		server.ListenAndServe()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: websocket.NewClientTransport(
			websocket.ClientTransportConfig{
				Host: "localhost",
				Port: 8006,
				TLSConfig: &tls.Config{
					InsecureSkipVerify: true, // self signed
				},
			}),
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})

	c := pingpong.NewPingPongClient(client)

	count := int32(0)

	md := rpc.NewMetadata()
	md.PutString("token", validToken)

	ctx := rpc.NewContextWithMetadata(context.Background(), md)

	for {
		resp, err := c.Ping(ctx, &pingpong.PingRequest{
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

func TestPingPongDuplicateGroupPanic(t *testing.T) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: websocket.NewServerTransport(
			websocket.ServerTransportConfig{
				Port: 8007,
			}),
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})

	defer func() {
		err := recover()
		require.NotNil(t, err)
	}()

	server.Group(func(s *rpc.Server) {
		pingpong.RegisterPingPongServer(server, &pingpongServer{})
	})
	server.Group(func(s *rpc.Server) {
		pingpong.RegisterPingPongServer(server, &pingpongServer{})
	})
}

type testerAServer struct {
}

func (s *testerAServer) Test(ctx context.Context, req *basic.TestRequestA) (*basic.TestResponseA, error) {
	return &basic.TestResponseA{
		A: req.A,
	}, nil
}

type testerBServer struct {
}

func (s *testerBServer) Test(ctx context.Context, req *basic.TestRequestB) (*basic.TestResponseB, error) {
	return &basic.TestResponseB{
		B: req.B,
	}, nil
}

func TestServerGroupsMiddleware(t *testing.T) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: websocket.NewServerTransport(
			websocket.ServerTransportConfig{
				Port: 8008,
			}),
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})

	server.Group(func(server *rpc.Server) {
		server.Middleware(authMiddleware)
		basic.RegisterTesterAServer(server, &testerAServer{})
	})
	server.Group(func(s *rpc.Server) {
		basic.RegisterTesterBServer(server, &testerBServer{})
	})

	go func() {
		server.ListenAndServe()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: websocket.NewClientTransport(
			websocket.ClientTransportConfig{
				Host: "localhost",
				Port: 8008,
			}),
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})

	cA := basic.NewTesterAClient(client)
	cB := basic.NewTesterBClient(client)

	_, err := cA.Test(context.Background(), &basic.TestRequestA{
		A: "A",
	})
	require.Error(t, err)
	assert.Equal(t, "no metadata", err.Error())

	resp, err := cB.Test(context.Background(), &basic.TestRequestB{
		B: "B",
	})
	require.NoError(t, err)
	assert.Equal(t, "B", resp.B)

	err = server.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestServerNestedGroupsMiddleware(t *testing.T) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: websocket.NewServerTransport(
			websocket.ServerTransportConfig{
				Port: 8009,
			}),
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})

	server.Group(func(server *rpc.Server) {

		server.Middleware(authMiddleware)
		basic.RegisterTesterAServer(server, &testerAServer{})

		server.Group(func(s *rpc.Server) {

			server.Middleware(alwaysRejectMiddleware)
			basic.RegisterTesterBServer(server, &testerBServer{})
		})
	})

	go func() {
		server.ListenAndServe()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: websocket.NewClientTransport(
			websocket.ClientTransportConfig{
				Host: "localhost",
				Port: 8009,
			}),
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})

	cA := basic.NewTesterAClient(client)
	cB := basic.NewTesterBClient(client)

	_, err := cA.Test(context.Background(), &basic.TestRequestA{
		A: "A",
	})
	require.Error(t, err)
	assert.Equal(t, "no metadata", err.Error())

	md := rpc.NewMetadata()
	md.PutString("token", validToken)

	ctx := rpc.NewContextWithMetadata(context.Background(), md)

	_, err = cB.Test(ctx, &basic.TestRequestB{
		B: "B",
	})
	require.Error(t, err)
	assert.Equal(t, "rejected", err.Error())

	err = server.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestWebSocketConcurrency(t *testing.T) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: websocket.NewServerTransport(
			websocket.ServerTransportConfig{
				Port: 8004,
			}),
		ErrHandler: func(err error) {
			t.Logf("Server error: %v", err)
		},
	})

	pingpong.RegisterPingPongServer(server, &pingpongServer{})

	go func() {
		server.ListenAndServe()
	}()

	time.Sleep(100 * time.Millisecond)
	defer server.Shutdown(context.Background())

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: websocket.NewClientTransport(
			websocket.ClientTransportConfig{
				Host: "localhost",
				Port: 8004,
			}),
		ErrHandler: func(err error) {
			t.Logf("Client error: %v", err)
		},
	})

	c := pingpong.NewPingPongClient(client)

	// Test concurrent requests from multiple goroutines
	const numGoroutines = 50
	const requestsPerGoroutine = 20

	var wg sync.WaitGroup
	var successCount atomic.Int32
	var errorCount atomic.Int32

	t.Logf("Starting %d goroutines, each sending %d requests", numGoroutines, requestsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		goroutineID := i

		go func(id int) {
			defer wg.Done()

			for j := 0; j < requestsPerGoroutine; j++ {
				// Use unique count value to verify responses match requests
				expectedCount := int32(id*requestsPerGoroutine + j)

				resp, err := c.Ping(context.Background(), &pingpong.PingRequest{
					Ping: pingpong.Ping{
						Count: expectedCount,
						Payload: pingpong.TestPayload{
							ValString: fmt.Sprintf("goroutine-%d-request-%d", id, j),
						},
					},
				})

				if err != nil {
					errorCount.Add(1)
					t.Errorf("Goroutine %d, request %d failed: %v", id, j, err)
					continue
				}

				// Verify we got the correct response for our request
				if resp.Pong.Count != expectedCount+1 {
					errorCount.Add(1)
					t.Errorf("Goroutine %d, request %d: expected count %d, got %d",
						id, j, expectedCount+1, resp.Pong.Count)
					continue
				}

				expectedPayload := fmt.Sprintf("goroutine-%d-request-%d", id, j)
				if resp.Pong.Payload.ValString != expectedPayload {
					errorCount.Add(1)
					t.Errorf("Goroutine %d, request %d: expected payload %q, got %q",
						id, j, expectedPayload, resp.Pong.Payload.ValString)
					continue
				}

				successCount.Add(1)
			}
		}(goroutineID)
	}

	wg.Wait()

	totalRequests := numGoroutines * requestsPerGoroutine
	success := int(successCount.Load())
	errors := int(errorCount.Load())

	t.Logf("Completed: %d successful, %d errors out of %d total requests",
		success, errors, totalRequests)

	assert.Equal(t, totalRequests, success, "All requests should succeed")
	assert.Equal(t, 0, errors, "No errors should occur")
}
