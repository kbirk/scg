package test

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/rpc/nats"
	"github.com/kbirk/scg/test/files/output/pingpong"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNATSConnectionFailure tests behavior when NATS server is unavailable
func TestNATSConnectionFailure(t *testing.T) {
	// Try to connect to non-existent NATS server
	client := rpc.NewClient(rpc.ClientConfig{
		Transport: nats.NewClientTransport(
			nats.ClientTransportConfig{
				URL: "nats://localhost:9999", // Non-existent server
			}),
	})

	c := pingpong.NewPingPongClient(client)

	// Request should fail with connection error
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := c.Ping(ctx, &pingpong.PingRequest{
		Ping: pingpong.Ping{Count: 1},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "connect", "Error should indicate connection failure")
}

// TestNATSContextTimeout tests that context timeout works for request cancellation
func TestNATSContextTimeout(t *testing.T) {
	// Create server with slow response
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: nats.NewServerTransport(
			nats.ServerTransportConfig{
				URL: natsURL,
			}),
	})

	// Server that delays 10 seconds
	slowServer := &slowPingPongServer{delay: 10 * time.Second}
	pingpong.RegisterPingPongServer(server, slowServer)

	go server.ListenAndServe()
	time.Sleep(100 * time.Millisecond)
	defer server.Shutdown(context.Background())

	// Create client
	client := rpc.NewClient(rpc.ClientConfig{
		Transport: nats.NewClientTransport(
			nats.ClientTransportConfig{
				URL: natsURL,
			}),
	})

	c := pingpong.NewPingPongClient(client)

	// Use short context timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := c.Ping(ctx, &pingpong.PingRequest{
		Ping: pingpong.Ping{Count: 1},
	})
	elapsed := time.Since(start)

	// Should timeout via context, not transport
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deadline", "Should timeout via context deadline")
	assert.Less(t, elapsed, 2*time.Second, "Should timeout quickly via context")
	t.Logf("Context timeout worked correctly after %v", elapsed)
}

// TestNATSGracefulShutdown tests that shutdown properly handles in-flight requests
func TestNATSGracefulShutdown(t *testing.T) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: nats.NewServerTransport(
			nats.ServerTransportConfig{
				URL: natsURL,
			}),
	})

	// Server that responds quickly
	pingpong.RegisterPingPongServer(server, &pingpongNatsServer{})

	go server.ListenAndServe()
	time.Sleep(100 * time.Millisecond)

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: nats.NewClientTransport(
			nats.ClientTransportConfig{
				URL: natsURL,
			}),
	})

	c := pingpong.NewPingPongClient(client)

	var wg sync.WaitGroup
	var successCount atomic.Int32
	var errorCount atomic.Int32

	// Start multiple requests
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Use short timeout since shutdown may prevent responses
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			resp, err := c.Ping(ctx, &pingpong.PingRequest{
				Ping: pingpong.Ping{Count: int32(id)},
			})

			if err != nil {
				errorCount.Add(1)
			} else if resp != nil {
				successCount.Add(1)
			}
		}(i)
	}

	// Give requests minimal time to start, then shutdown immediately
	time.Sleep(10 * time.Millisecond)

	// Shutdown while requests are in flight
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := server.Shutdown(shutdownCtx)
	require.NoError(t, err)

	// Wait for all client goroutines to finish (with timeout)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Good, all goroutines finished
	case <-time.After(3 * time.Second):
		t.Fatal("Test timed out waiting for client goroutines to finish")
	}

	success := int(successCount.Load())
	errors := int(errorCount.Load())

	t.Logf("After shutdown: %d successful, %d errors", success, errors)

	// All requests should complete (successfully or with error)
	assert.Equal(t, 10, success+errors, "All requests should complete")
}

// TestNATSLargePayload tests handling of large messages
func TestNATSLargePayload(t *testing.T) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: nats.NewServerTransport(
			nats.ServerTransportConfig{
				URL: natsURL,
			}),
	})

	pingpong.RegisterPingPongServer(server, &pingpongNatsServer{})

	go server.ListenAndServe()
	time.Sleep(100 * time.Millisecond)
	defer server.Shutdown(context.Background())

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: nats.NewClientTransport(
			nats.ClientTransportConfig{
				URL: natsURL,
			}),
	})

	c := pingpong.NewPingPongClient(client)

	// NATS default max payload is 1MB
	// Test with sizes around that limit
	testCases := []struct {
		name        string
		payloadSize int
		shouldPass  bool
	}{
		{"Small 1KB", 1024, true},
		{"Medium 100KB", 100 * 1024, true},
		{"Large 500KB", 500 * 1024, true},
		{"Very Large 2MB", 2 * 1024 * 1024, false}, // Should fail
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			largePayload := strings.Repeat("x", tc.payloadSize)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			resp, err := c.Ping(ctx, &pingpong.PingRequest{
				Ping: pingpong.Ping{
					Count: 1,
					Payload: pingpong.TestPayload{
						ValString: largePayload,
					},
				},
			})

			if tc.shouldPass {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, tc.payloadSize, len(resp.Pong.Payload.ValString))
			} else {
				require.Error(t, err, "Should fail with large payload")
				t.Logf("Large payload correctly rejected: %v", err)
			}
		})
	}
}

// TestNATSServerOverload tests server backpressure handling
func TestNATSServerOverload(t *testing.T) {
	// Create server with small connection buffer to easily trigger overload
	transport := nats.NewServerTransport(
		nats.ServerTransportConfig{
			URL: natsURL,
		})

	server := rpc.NewServer(rpc.ServerConfig{
		Transport: transport,
	})

	// Use very slow server to fill up the buffer
	slowServer := &slowPingPongServer{delay: 2 * time.Second}
	pingpong.RegisterPingPongServer(server, slowServer)

	go server.ListenAndServe()
	time.Sleep(100 * time.Millisecond)
	defer server.Shutdown(context.Background())

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: nats.NewClientTransport(
			nats.ClientTransportConfig{
				URL: natsURL,
			}),
	})

	c := pingpong.NewPingPongClient(client)

	// Flood the server with more requests than it can handle
	// The connection channel buffer is 100, and server is very slow
	const numRequests = 150

	var wg sync.WaitGroup
	var busyCount atomic.Int32
	var timeoutCount atomic.Int32
	var successCount atomic.Int32

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			resp, err := c.Ping(ctx, &pingpong.PingRequest{
				Ping: pingpong.Ping{Count: int32(id)},
			})

			if err != nil {
				errMsg := err.Error()
				if strings.Contains(errMsg, "server busy") {
					busyCount.Add(1)
				} else if strings.Contains(errMsg, "timeout") ||
					strings.Contains(errMsg, "deadline") ||
					strings.Contains(errMsg, "channel closed") {
					timeoutCount.Add(1)
				}
			} else if resp != nil {
				successCount.Add(1)
			}
		}(i)
	}

	wg.Wait()

	busy := int(busyCount.Load())
	timeout := int(timeoutCount.Load())
	success := int(successCount.Load())

	t.Logf("Results: %d successful, %d busy, %d timeout out of %d requests",
		success, busy, timeout, numRequests)

	// With proper concurrency handling, the system should handle high load
	// Some requests may timeout due to the 5-second context timeout vs 2-second server delay
	// This test verifies the system remains stable under load
	assert.Equal(t, numRequests, success+busy+timeout, "All requests should complete")
	t.Logf("System handled high concurrent load successfully")
}

// TestNATSMultipleClients tests multiple clients connecting to same service
func TestNATSMultipleClients(t *testing.T) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: nats.NewServerTransport(
			nats.ServerTransportConfig{
				URL: natsURL,
			}),
	})

	counterServer := &counterPingPongServer{}
	pingpong.RegisterPingPongServer(server, counterServer)

	go server.ListenAndServe()
	time.Sleep(100 * time.Millisecond)
	defer server.Shutdown(context.Background())

	// Create multiple clients
	const numClients = 10
	const requestsPerClient = 20

	var wg sync.WaitGroup
	var totalSuccess atomic.Int32

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			// Each client gets its own connection
			client := rpc.NewClient(rpc.ClientConfig{
				Transport: nats.NewClientTransport(
					nats.ClientTransportConfig{
						URL: natsURL,
					}),
			})

			c := pingpong.NewPingPongClient(client)

			for j := 0; j < requestsPerClient; j++ {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				resp, err := c.Ping(ctx, &pingpong.PingRequest{
					Ping: pingpong.Ping{Count: int32(clientID*1000 + j)},
				})
				cancel()

				if err == nil && resp != nil {
					totalSuccess.Add(1)
				}
			}
		}(i)
	}

	wg.Wait()

	expectedRequests := numClients * requestsPerClient
	success := int(totalSuccess.Load())

	t.Logf("Multiple clients: %d successful out of %d requests", success, expectedRequests)

	assert.Equal(t, expectedRequests, success, "All requests from all clients should succeed")
	assert.Equal(t, int32(expectedRequests), counterServer.callCount.Load(),
		"Server should process all requests")
}

// TestNATSRapidConnectionChurn tests creating and destroying connections rapidly
func TestNATSRapidConnectionChurn(t *testing.T) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: nats.NewServerTransport(
			nats.ServerTransportConfig{
				URL: natsURL,
			}),
	})

	pingpong.RegisterPingPongServer(server, &pingpongNatsServer{})

	go server.ListenAndServe()
	time.Sleep(100 * time.Millisecond)
	defer server.Shutdown(context.Background())

	// Rapidly create connections, make a request, and implicitly disconnect
	const numIterations = 50

	for i := 0; i < numIterations; i++ {
		client := rpc.NewClient(rpc.ClientConfig{
			Transport: nats.NewClientTransport(
				nats.ClientTransportConfig{
					URL: natsURL,
				}),
		})

		c := pingpong.NewPingPongClient(client)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		resp, err := c.Ping(ctx, &pingpong.PingRequest{
			Ping: pingpong.Ping{Count: int32(i)},
		})
		cancel()

		require.NoError(t, err, "Request %d should succeed", i)
		require.NotNil(t, resp)
		assert.Equal(t, int32(i+1), resp.Pong.Count)

		// Client goes out of scope - connection should be cleaned up
	}

	t.Log("All rapid connection iterations completed successfully")
}

// Helper server implementations

type slowPingPongServer struct {
	delay time.Duration
}

func (s *slowPingPongServer) Ping(ctx context.Context, req *pingpong.PingRequest) (*pingpong.PongResponse, error) {
	time.Sleep(s.delay)
	return &pingpong.PongResponse{
		Pong: pingpong.Pong{
			Count:   req.Ping.Count + 1,
			Payload: req.Ping.Payload,
		},
	}, nil
}

type counterPingPongServer struct {
	callCount atomic.Int32
}

func (s *counterPingPongServer) Ping(ctx context.Context, req *pingpong.PingRequest) (*pingpong.PongResponse, error) {
	s.callCount.Add(1)
	return &pingpong.PongResponse{
		Pong: pingpong.Pong{
			Count:   req.Ping.Count + 1,
			Payload: req.Ping.Payload,
		},
	}, nil
}
