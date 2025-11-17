package test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/rpc/nats"
	"github.com/kbirk/scg/test/files/output/basic"
	"github.com/kbirk/scg/test/files/output/pingpong"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	natsURL = "nats://localhost:4222"
)

type pingpongNatsServer struct {
}

func (s *pingpongNatsServer) Ping(ctx context.Context, req *pingpong.PingRequest) (*pingpong.PongResponse, error) {
	return &pingpong.PongResponse{
		Pong: pingpong.Pong{
			Count:   req.Ping.Count + 1,
			Payload: req.Ping.Payload,
		},
	}, nil
}

// TestPingPongNATS tests the RPC system using NATS transport
// Note: This test requires a NATS server running on localhost:4222
// You can start one with: docker run -p 4222:4222 nats:latest
// Or run: ./run-nats-tests.sh
func TestPingPongNATS(t *testing.T) {
	t.Log("Creating server...")
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: nats.NewServerTransport(
			nats.ServerTransportConfig{
				URL: natsURL,
			}),
		ErrHandler: func(err error) {
			t.Logf("Server error: %v", err)
		},
	})

	t.Log("Registering service...")
	pingpong.RegisterPingPongServer(server, &pingpongNatsServer{})

	t.Log("Starting server...")
	go func() {
		err := server.ListenAndServe()
		if err != nil {
			t.Logf("Server stopped: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)
	defer server.Shutdown(context.Background())

	t.Log("Creating client...")
	client := rpc.NewClient(rpc.ClientConfig{
		Transport: nats.NewClientTransport(
			nats.ClientTransportConfig{
				URL: natsURL,
			}),
		ErrHandler: func(err error) {
			t.Logf("Client error: %v", err)
		},
	})

	c := pingpong.NewPingPongClient(client)

	t.Log("Sending ping request...")
	// Test basic ping-pong
	resp, err := c.Ping(context.Background(), &pingpong.PingRequest{
		Ping: pingpong.Ping{
			Count: 1,
			Payload: pingpong.TestPayload{
				ValString: "Hello NATS!",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int32(2), resp.Pong.Count)
	assert.Equal(t, "Hello NATS!", resp.Pong.Payload.ValString)

	// Test multiple sequential requests
	for i := int32(1); i <= 5; i++ {
		resp, err := c.Ping(context.Background(), &pingpong.PingRequest{
			Ping: pingpong.Ping{
				Count: i * 10,
			},
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, i*10+1, resp.Pong.Count)
	}
}

// TestNATSMiddleware tests that middleware works with NATS transport
func TestNATSMiddleware(t *testing.T) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: nats.NewServerTransport(
			nats.ServerTransportConfig{
				URL: natsURL,
			}),
	})

	// Add server middleware
	serverMiddlewareCount := 0
	server.Middleware(func(ctx context.Context, req rpc.Message, next rpc.Handler) (rpc.Message, error) {
		serverMiddlewareCount++
		return next(ctx, req)
	})

	pingpong.RegisterPingPongServer(server, &pingpongNatsServer{})

	go func() {
		server.ListenAndServe()
	}()

	time.Sleep(100 * time.Millisecond)
	defer server.Shutdown(context.Background())

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: nats.NewClientTransport(
			nats.ClientTransportConfig{
				URL: natsURL,
			}),
	})

	// Add client middleware
	clientMiddlewareCount := 0
	client.Middleware(func(ctx context.Context, req rpc.Message, next rpc.Handler) (rpc.Message, error) {
		clientMiddlewareCount++
		return next(ctx, req)
	})

	c := pingpong.NewPingPongClient(client)

	// Make a request
	_, err := c.Ping(context.Background(), &pingpong.PingRequest{
		Ping: pingpong.Ping{Count: 1},
	})

	require.NoError(t, err)
	assert.Equal(t, 1, serverMiddlewareCount)
	assert.Equal(t, 1, clientMiddlewareCount)
}

// TestNATSServiceIsolation tests that multiple servers with different services
// correctly route requests based on NATS subjects
func TestNATSServiceIsolation(t *testing.T) {
	// Server 1: Only hosts TesterA service
	server1 := rpc.NewServer(rpc.ServerConfig{
		Transport: nats.NewServerTransport(
			nats.ServerTransportConfig{
				URL: natsURL,
			}),
		ErrHandler: func(err error) {
			t.Logf("Server1 error: %v", err)
		},
	})

	testerAImpl := &testerAServerImpl{responsePrefix: "ServerA"}
	basic.RegisterTesterAServer(server1, testerAImpl)

	go func() {
		err := server1.ListenAndServe()
		if err != nil {
			t.Logf("Server1 stopped: %v", err)
		}
	}()

	// Server 2: Only hosts TesterB service
	server2 := rpc.NewServer(rpc.ServerConfig{
		Transport: nats.NewServerTransport(
			nats.ServerTransportConfig{
				URL: natsURL,
			}),
		ErrHandler: func(err error) {
			t.Logf("Server2 error: %v", err)
		},
	})

	testerBImpl := &testerBServerImpl{responsePrefix: "ServerB"}
	basic.RegisterTesterBServer(server2, testerBImpl)

	go func() {
		err := server2.ListenAndServe()
		if err != nil {
			t.Logf("Server2 stopped: %v", err)
		}
	}()

	// Give servers time to start
	time.Sleep(200 * time.Millisecond)
	defer server1.Shutdown(context.Background())
	defer server2.Shutdown(context.Background())

	// Single client can connect to both services
	client := rpc.NewClient(rpc.ClientConfig{
		Transport: nats.NewClientTransport(
			nats.ClientTransportConfig{
				URL: natsURL,
			}),
		ErrHandler: func(err error) {
			t.Logf("Client error: %v", err)
		},
	})

	testerAClient := basic.NewTesterAClient(client)
	testerBClient := basic.NewTesterBClient(client)

	// Test TesterA - should be handled by Server 1
	respA, err := testerAClient.Test(context.Background(), &basic.TestRequestA{
		A: "test-a",
	})
	require.NoError(t, err)
	require.NotNil(t, respA)
	assert.Equal(t, "ServerA:test-a", respA.A, "TesterA should be handled by Server1")

	// Test TesterB - should be handled by Server 2
	respB, err := testerBClient.Test(context.Background(), &basic.TestRequestB{
		B: "test-b",
	})
	require.NoError(t, err)
	require.NotNil(t, respB)
	assert.Equal(t, "ServerB:test-b", respB.B, "TesterB should be handled by Server2")

	// Run multiple requests to ensure routing is consistent
	for i := 0; i < 5; i++ {
		respA, err := testerAClient.Test(context.Background(), &basic.TestRequestA{
			A: fmt.Sprintf("request-%d", i),
		})
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("ServerA:request-%d", i), respA.A)

		respB, err := testerBClient.Test(context.Background(), &basic.TestRequestB{
			B: fmt.Sprintf("request-%d", i),
		})
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("ServerB:request-%d", i), respB.B)
	}

	// Verify counters to ensure proper isolation
	assert.Equal(t, 6, testerAImpl.callCount, "TesterA should have received 6 calls")
	assert.Equal(t, 6, testerBImpl.callCount, "TesterB should have received 6 calls")
}

// TestNATSMultipleServicesOnOneServer tests that a single server can host multiple services
func TestNATSMultipleServicesOnOneServer(t *testing.T) {
	// Single server hosting both TesterA and TesterB
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: nats.NewServerTransport(
			nats.ServerTransportConfig{
				URL: natsURL,
			}),
		ErrHandler: func(err error) {
			t.Logf("Server error: %v", err)
		},
	})

	testerAImpl := &testerAServerImpl{responsePrefix: "Combined"}
	testerBImpl := &testerBServerImpl{responsePrefix: "Combined"}

	basic.RegisterTesterAServer(server, testerAImpl)
	basic.RegisterTesterBServer(server, testerBImpl)

	go func() {
		err := server.ListenAndServe()
		if err != nil {
			t.Logf("Server stopped: %v", err)
		}
	}()

	time.Sleep(200 * time.Millisecond)
	defer server.Shutdown(context.Background())

	// Single client can connect to both services on the same server
	client := rpc.NewClient(rpc.ClientConfig{
		Transport: nats.NewClientTransport(
			nats.ClientTransportConfig{
				URL: natsURL,
			}),
	})

	testerAClient := basic.NewTesterAClient(client)
	testerBClient := basic.NewTesterBClient(client)

	// Both should work from the same server
	respA, err := testerAClient.Test(context.Background(), &basic.TestRequestA{A: "multi-a"})
	require.NoError(t, err)
	assert.Equal(t, "Combined:multi-a", respA.A)

	respB, err := testerBClient.Test(context.Background(), &basic.TestRequestB{B: "multi-b"})
	require.NoError(t, err)
	assert.Equal(t, "Combined:multi-b", respB.B)

	assert.Equal(t, 1, testerAImpl.callCount)
	assert.Equal(t, 1, testerBImpl.callCount)
}

// Test server implementations with call tracking
type testerAServerImpl struct {
	responsePrefix string
	callCount      int
	mu             sync.Mutex
}

func (s *testerAServerImpl) Test(ctx context.Context, req *basic.TestRequestA) (*basic.TestResponseA, error) {
	s.mu.Lock()
	s.callCount++
	s.mu.Unlock()
	return &basic.TestResponseA{
		A: fmt.Sprintf("%s:%s", s.responsePrefix, req.A),
	}, nil
}

type testerBServerImpl struct {
	responsePrefix string
	callCount      int
	mu             sync.Mutex
}

func (s *testerBServerImpl) Test(ctx context.Context, req *basic.TestRequestB) (*basic.TestResponseB, error) {
	s.mu.Lock()
	s.callCount++
	s.mu.Unlock()
	return &basic.TestResponseB{
		B: fmt.Sprintf("%s:%s", s.responsePrefix, req.B),
	}, nil
}

func TestNATSConcurrency(t *testing.T) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: nats.NewServerTransport(
			nats.ServerTransportConfig{
				URL: natsURL,
			}),
		ErrHandler: func(err error) {
			t.Logf("Server error: %v", err)
		},
	})

	pingpong.RegisterPingPongServer(server, &pingpongNatsServer{})

	go func() {
		err := server.ListenAndServe()
		if err != nil {
			t.Logf("Server stopped: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	defer server.Shutdown(context.Background())

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: nats.NewClientTransport(
			nats.ClientTransportConfig{
				URL: natsURL,
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
