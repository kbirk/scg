package test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/rpc/tcp"
	"github.com/kbirk/scg/test/files/output/pingpong"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPingPongTCP(t *testing.T) {

	server := rpc.NewServer(rpc.ServerConfig{
		Transport: tcp.NewServerTransport(
			tcp.ServerTransportConfig{
				Port: 9000,
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
		Transport: tcp.NewClientTransport(
			tcp.ClientTransportConfig{
				Host: "localhost",
				Port: 9000,
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

func TestPingPongConcurrentTCP(t *testing.T) {

	server := rpc.NewServer(rpc.ServerConfig{
		Transport: tcp.NewServerTransport(
			tcp.ServerTransportConfig{
				Port: 9001,
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
		Transport: tcp.NewClientTransport(
			tcp.ClientTransportConfig{
				Host: "localhost",
				Port: 9001,
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

func TestPingPongAuthFailTCP(t *testing.T) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: tcp.NewServerTransport(
			tcp.ServerTransportConfig{
				Port: 9005,
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
		Transport: tcp.NewClientTransport(
			tcp.ClientTransportConfig{
				Host: "localhost",
				Port: 9005,
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

func TestTCPConcurrency(t *testing.T) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: tcp.NewServerTransport(
			tcp.ServerTransportConfig{
				Port: 9004,
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
		Transport: tcp.NewClientTransport(
			tcp.ClientTransportConfig{
				Host: "localhost",
				Port: 9004,
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
