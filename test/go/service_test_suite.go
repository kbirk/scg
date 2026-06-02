package test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/rpc/tcp"
	"github.com/kbirk/scg/pkg/rpc/websocket"
	"github.com/kbirk/scg/test/go/chat"
	"github.com/kbirk/scg/test/scg/generated/basic"
	"github.com/kbirk/scg/test/scg/generated/pingpong"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TransportFactory creates server and client transports for testing
type TransportFactory interface {
	// CreateServerTransport creates a new server transport for testing
	// The port/path parameter allows tests to use unique endpoints
	CreateServerTransport(id int) rpc.ServerTransport
	// CreateClientTransport creates a new client transport for testing
	CreateClientTransport(id int) rpc.ClientTransport
	// SupportsMultipleServers indicates whether this transport supports multiple servers on same messaging infrastructure
	SupportsMultipleServers() bool
	// Name returns the transport name for test naming
	Name() string
}

// TestSuiteConfig holds configuration for running the test suite
type TestSuiteConfig struct {
	Factory           TransportFactory
	StartingPort      int  // Starting port/ID for test isolation
	SkipEdgeTests     bool // Skip edge case tests (connection failure, large payload, etc.)
	SkipTLSTests      bool // Skip TLS-specific tests
	SkipGroupTests    bool // Skip server group tests
	UseExternalServer bool // If true, assumes a server is already running externally (for cross-language testing)
	LargePayloadSizes []LargePayloadTestCase
}

// LargePayloadTestCase defines a payload size test case
type LargePayloadTestCase struct {
	Name       string
	Size       int
	ShouldPass bool
}

// DefaultLargePayloadCases returns default large payload test cases
func DefaultLargePayloadCases() []LargePayloadTestCase {
	return []LargePayloadTestCase{
		{"Small 1KB", 1024, true},
		{"Medium 100KB", 100 * 1024, true},
		{"Large 500KB", 500 * 1024, true},
	}
}

// RunTestSuite runs all tests for a given transport
func RunTestSuite(t *testing.T, config TestSuiteConfig) {
	// All tests use the same port since they run sequentially
	// and each test shuts down its server before the next one starts
	port := config.StartingPort

	t.Run("PingPong", func(t *testing.T) {
		runPingPongTest(t, config.Factory, port, config.UseExternalServer)
	})

	t.Run("PingPongConcurrent", func(t *testing.T) {
		runPingPongConcurrentTest(t, config.Factory, port, config.UseExternalServer)
	})

	// Tests that require server control (middleware, auth, etc.) are skipped when using external server
	if !config.UseExternalServer {
		t.Run("PingPongAuthFail", func(t *testing.T) {
			runPingPongAuthFailTest(t, config.Factory, port)
		})

		t.Run("PingPongAuthSuccess", func(t *testing.T) {
			runPingPongAuthSuccessTest(t, config.Factory, port)
		})

		t.Run("Middleware", func(t *testing.T) {
			runMiddlewareTest(t, config.Factory, port)
		})

		t.Run("PingPongFail", func(t *testing.T) {
			runPingPongFailTest(t, config.Factory, port)
		})

		if !config.SkipGroupTests {
			t.Run("ServerGroupsMiddleware", func(t *testing.T) {
				runServerGroupsMiddlewareTest(t, config.Factory, port)
			})

			t.Run("ServerNestedGroupsMiddleware", func(t *testing.T) {
				runServerNestedGroupsMiddlewareTest(t, config.Factory, port)
			})

			t.Run("DuplicateGroupPanic", func(t *testing.T) {
				runDuplicateGroupPanicTest(t, config.Factory, port)
			})
		}

		if config.Factory.SupportsMultipleServers() {
			t.Run("ServiceIsolation", func(t *testing.T) {
				runServiceIsolationTest(t, config.Factory, port)
			})

			t.Run("MultipleServicesOnOneServer", func(t *testing.T) {
				runMultipleServicesOnOneServerTest(t, config.Factory, port)
			})
		}

		if !config.SkipEdgeTests {
			t.Run("GracefulShutdown", func(t *testing.T) {
				runGracefulShutdownTest(t, config.Factory, port)
			})

			t.Run("LargePayload", func(t *testing.T) {
				sizes := config.LargePayloadSizes
				if sizes == nil {
					sizes = DefaultLargePayloadCases()
				}
				runLargePayloadTest(t, config.Factory, port, sizes)
			})

			t.Run("MultipleClients", func(t *testing.T) {
				runMultipleClientsTest(t, config.Factory, port)
			})

			t.Run("RapidConnectionChurn", func(t *testing.T) {
				runRapidConnectionChurnTest(t, config.Factory, port)
			})

			t.Run("MaxMessageSize", func(t *testing.T) {
				runMaxMessageSizeTest(t, config.Factory, port)
			})

			t.Run("ContextTimeout", func(t *testing.T) {
				runContextTimeoutTest(t, config.Factory, port)
			})

			t.Run("ContextTimeoutRecovery", func(t *testing.T) {
				runContextTimeoutRecoveryTest(t, config.Factory, port)
			})

			t.Run("ContextMetadata", func(t *testing.T) {
				runContextMetadataTest(t, config.Factory, port)
			})

			t.Run("StreamServerError", func(t *testing.T) {
				runStreamServerErrorTest(t, config.Factory, port)
			})

			t.Run("StreamConcurrent", func(t *testing.T) {
				runStreamConcurrentTest(t, config.Factory, port)
			})

			t.Run("StreamUnaryCoexist", func(t *testing.T) {
				runStreamUnaryCoexistTest(t, config.Factory, port)
			})

			t.Run("StreamCancel", func(t *testing.T) {
				runStreamCancelTest(t, config.Factory, port)
			})

			t.Run("StreamConnectionDrop", func(t *testing.T) {
				runStreamConnectionDropTest(t, config.Factory, port)
			})

			t.Run("StreamAuthFail", func(t *testing.T) {
				runStreamAuthFailTest(t, config.Factory, port)
			})

			t.Run("StreamConcurrentSendRecv", func(t *testing.T) {
				runStreamConcurrentSendRecvTest(t, config.Factory, port)
			})

			t.Run("StreamSendAfterCloseSend", func(t *testing.T) {
				runStreamSendAfterCloseSendTest(t, config.Factory, port)
			})

			t.Run("StreamLargeMessage", func(t *testing.T) {
				runStreamLargeMessageTest(t, config.Factory, port)
			})

			t.Run("StreamRecvAfterEOF", func(t *testing.T) {
				runStreamRecvAfterEOFTest(t, config.Factory, port)
			})

			t.Run("StreamBackpressure", func(t *testing.T) {
				runStreamBackpressureTest(t, config.Factory, port)
			})

			t.Run("StreamMaxConcurrent", func(t *testing.T) {
				runStreamMaxConcurrentTest(t, config.Factory, port)
			})

			t.Run("StreamKeepalive", func(t *testing.T) {
				runStreamKeepaliveTest(t, config.Factory, port)
			})
		}
	}

	// Streaming tests that run both in-process and against an external
	// (cross-language) server — these validate the stream wire framing interops.
	t.Run("StreamBidi", func(t *testing.T) {
		runStreamBidiTest(t, config.Factory, port, config.UseExternalServer)
	})

	t.Run("StreamServerStreaming", func(t *testing.T) {
		runStreamServerStreamingTest(t, config.Factory, port, config.UseExternalServer)
	})

	t.Run("StreamClientStreaming", func(t *testing.T) {
		runStreamClientStreamingTest(t, config.Factory, port, config.UseExternalServer)
	})

	t.Run("Concurrency", func(t *testing.T) {
		runConcurrencyTest(t, config.Factory, port, config.UseExternalServer)
	})

	t.Run("EmptyPayload", func(t *testing.T) {
		runEmptyPayloadTest(t, config.Factory, port, config.UseExternalServer)
	})

	t.Run("SequentialRequests", func(t *testing.T) {
		runSequentialRequestsTest(t, config.Factory, port, config.UseExternalServer)
	})
}

// runPingPongTest tests basic ping-pong functionality
func runPingPongTest(t *testing.T, factory TransportFactory, id int, useExternalServer bool) {
	var server *rpc.Server
	if !useExternalServer {
		server = rpc.NewServer(rpc.ServerConfig{
			Transport: factory.CreateServerTransport(id),
			ErrHandler: func(err error) {
				require.NoError(t, err)
			},
		})
		pingpong.RegisterPingPongServer(server, &pingpongServer{})

		go func() {
			server.ListenAndServe()
		}()

		time.Sleep(100 * time.Millisecond)
	}

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: factory.CreateClientTransport(id),
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

	if !useExternalServer {
		err := server.Shutdown(context.Background())
		require.NoError(t, err)
	}
}

// runPingPongConcurrentTest tests concurrent ping-pong requests
func runPingPongConcurrentTest(t *testing.T, factory TransportFactory, id int, useExternalServer bool) {
	var server *rpc.Server
	if !useExternalServer {
		server = rpc.NewServer(rpc.ServerConfig{
			Transport: factory.CreateServerTransport(id),
			ErrHandler: func(err error) {
				require.NoError(t, err)
			},
		})
		pingpong.RegisterPingPongServer(server, &pingpongServer{})

		go func() {
			server.ListenAndServe()
		}()

		time.Sleep(100 * time.Millisecond)
	}

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: factory.CreateClientTransport(id),
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

	if !useExternalServer {
		err := server.Shutdown(context.Background())
		require.NoError(t, err)
	}
}

// runPingPongAuthFailTest tests auth middleware rejection
func runPingPongAuthFailTest(t *testing.T, factory TransportFactory, id int) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: factory.CreateServerTransport(id),
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})
	server.Middleware(authMiddleware)
	pingpong.RegisterPingPongServer(server, &pingpongServerFail{})

	go func() {
		server.ListenAndServe()
	}()

	time.Sleep(100 * time.Millisecond)

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: factory.CreateClientTransport(id),
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

// runPingPongAuthSuccessTest tests auth middleware success
func runPingPongAuthSuccessTest(t *testing.T, factory TransportFactory, id int) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: factory.CreateServerTransport(id),
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})
	server.Middleware(authMiddleware)
	pingpong.RegisterPingPongServer(server, &pingpongServer{})

	go func() {
		server.ListenAndServe()
	}()

	time.Sleep(100 * time.Millisecond)

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: factory.CreateClientTransport(id),
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})

	c := pingpong.NewPingPongClient(client)

	md := rpc.NewMetadata()
	md.PutString("token", validToken)

	ctx := rpc.NewContextWithMetadata(context.Background(), md)

	count := int32(0)

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

// runMiddlewareTest tests that middleware works on both client and server
func runMiddlewareTest(t *testing.T, factory TransportFactory, id int) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: factory.CreateServerTransport(id),
	})

	serverMiddlewareCount := 0
	server.Middleware(func(ctx context.Context, req rpc.Message, next rpc.Handler) (rpc.Message, error) {
		serverMiddlewareCount++
		return next(ctx, req)
	})

	pingpong.RegisterPingPongServer(server, &pingpongServer{})

	go func() {
		server.ListenAndServe()
	}()

	time.Sleep(100 * time.Millisecond)
	defer server.Shutdown(context.Background())

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: factory.CreateClientTransport(id),
	})

	clientMiddlewareCount := 0
	client.Middleware(func(ctx context.Context, req rpc.Message, next rpc.Handler) (rpc.Message, error) {
		clientMiddlewareCount++
		return next(ctx, req)
	})

	c := pingpong.NewPingPongClient(client)

	_, err := c.Ping(context.Background(), &pingpong.PingRequest{
		Ping: pingpong.Ping{Count: 1},
	})

	require.NoError(t, err)
	assert.Equal(t, 1, serverMiddlewareCount)
	assert.Equal(t, 1, clientMiddlewareCount)
}

// runConcurrencyTest tests high concurrency with request/response verification
func runConcurrencyTest(t *testing.T, factory TransportFactory, id int, useExternalServer bool) {
	var server *rpc.Server
	if !useExternalServer {
		server = rpc.NewServer(rpc.ServerConfig{
			Transport: factory.CreateServerTransport(id),
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
	}

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: factory.CreateClientTransport(id),
		ErrHandler: func(err error) {
			t.Logf("Client error: %v", err)
		},
	})

	c := pingpong.NewPingPongClient(client)

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

// runPingPongFailTest tests server-side error handling
func runPingPongFailTest(t *testing.T, factory TransportFactory, id int) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: factory.CreateServerTransport(id),
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})
	pingpong.RegisterPingPongServer(server, &pingpongServerFail{})

	go func() {
		server.ListenAndServe()
	}()

	time.Sleep(100 * time.Millisecond)

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: factory.CreateClientTransport(id),
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

// runServerGroupsMiddlewareTest tests server groups with middleware
func runServerGroupsMiddlewareTest(t *testing.T, factory TransportFactory, id int) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: factory.CreateServerTransport(id),
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})

	server.Group(func(server *rpc.Server) {
		server.Middleware(authMiddleware)
		basic.RegisterTesterAServer(server, &suiteTesterAServer{})
	})
	server.Group(func(s *rpc.Server) {
		basic.RegisterTesterBServer(server, &suiteTesterBServer{})
	})

	go func() {
		server.ListenAndServe()
	}()

	time.Sleep(100 * time.Millisecond)

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: factory.CreateClientTransport(id),
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

// runServerNestedGroupsMiddlewareTest tests nested server groups with middleware
func runServerNestedGroupsMiddlewareTest(t *testing.T, factory TransportFactory, id int) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: factory.CreateServerTransport(id),
		ErrHandler: func(err error) {
			require.NoError(t, err)
		},
	})

	server.Group(func(server *rpc.Server) {
		server.Middleware(authMiddleware)
		basic.RegisterTesterAServer(server, &suiteTesterAServer{})

		server.Group(func(s *rpc.Server) {
			server.Middleware(alwaysRejectMiddleware)
			basic.RegisterTesterBServer(server, &suiteTesterBServer{})
		})
	})

	go func() {
		server.ListenAndServe()
	}()

	time.Sleep(100 * time.Millisecond)

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: factory.CreateClientTransport(id),
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

// runDuplicateGroupPanicTest tests that duplicate service registration panics
func runDuplicateGroupPanicTest(t *testing.T, factory TransportFactory, id int) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: factory.CreateServerTransport(id),
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

// runServiceIsolationTest tests that multiple servers with different services correctly route requests
func runServiceIsolationTest(t *testing.T, factory TransportFactory, id int) {
	server1 := rpc.NewServer(rpc.ServerConfig{
		Transport: factory.CreateServerTransport(id),
		ErrHandler: func(err error) {
			t.Logf("Server1 error: %v", err)
		},
	})

	testerAImpl := &suiteTesterAServerImpl{responsePrefix: "ServerA"}
	basic.RegisterTesterAServer(server1, testerAImpl)

	go func() {
		err := server1.ListenAndServe()
		if err != nil {
			t.Logf("Server1 stopped: %v", err)
		}
	}()

	server2 := rpc.NewServer(rpc.ServerConfig{
		Transport: factory.CreateServerTransport(id + 100), // Use different port for server2
		ErrHandler: func(err error) {
			t.Logf("Server2 error: %v", err)
		},
	})

	testerBImpl := &suiteTesterBServerImpl{responsePrefix: "ServerB"}
	basic.RegisterTesterBServer(server2, testerBImpl)

	go func() {
		err := server2.ListenAndServe()
		if err != nil {
			t.Logf("Server2 stopped: %v", err)
		}
	}()

	time.Sleep(200 * time.Millisecond)
	defer server1.Shutdown(context.Background())
	defer server2.Shutdown(context.Background())

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: factory.CreateClientTransport(id),
		ErrHandler: func(err error) {
			t.Logf("Client error: %v", err)
		},
	})

	testerAClient := basic.NewTesterAClient(client)
	testerBClient := basic.NewTesterBClient(client)

	respA, err := testerAClient.Test(context.Background(), &basic.TestRequestA{
		A: "test-a",
	})
	require.NoError(t, err)
	require.NotNil(t, respA)
	assert.Equal(t, "ServerA:test-a", respA.A, "TesterA should be handled by Server1")

	respB, err := testerBClient.Test(context.Background(), &basic.TestRequestB{
		B: "test-b",
	})
	require.NoError(t, err)
	require.NotNil(t, respB)
	assert.Equal(t, "ServerB:test-b", respB.B, "TesterB should be handled by Server2")

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

	assert.Equal(t, 6, testerAImpl.callCount, "TesterA should have received 6 calls")
	assert.Equal(t, 6, testerBImpl.callCount, "TesterB should have received 6 calls")
}

// runMultipleServicesOnOneServerTest tests that a single server can host multiple services
func runMultipleServicesOnOneServerTest(t *testing.T, factory TransportFactory, id int) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: factory.CreateServerTransport(id),
		ErrHandler: func(err error) {
			t.Logf("Server error: %v", err)
		},
	})

	testerAImpl := &suiteTesterAServerImpl{responsePrefix: "Combined"}
	testerBImpl := &suiteTesterBServerImpl{responsePrefix: "Combined"}

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

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: factory.CreateClientTransport(id),
	})

	testerAClient := basic.NewTesterAClient(client)
	testerBClient := basic.NewTesterBClient(client)

	respA, err := testerAClient.Test(context.Background(), &basic.TestRequestA{A: "multi-a"})
	require.NoError(t, err)
	assert.Equal(t, "Combined:multi-a", respA.A)

	respB, err := testerBClient.Test(context.Background(), &basic.TestRequestB{B: "multi-b"})
	require.NoError(t, err)
	assert.Equal(t, "Combined:multi-b", respB.B)

	assert.Equal(t, 1, testerAImpl.callCount)
	assert.Equal(t, 1, testerBImpl.callCount)
}

// runGracefulShutdownTest tests that shutdown properly handles in-flight requests
func runGracefulShutdownTest(t *testing.T, factory TransportFactory, id int) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: factory.CreateServerTransport(id),
	})

	pingpong.RegisterPingPongServer(server, &pingpongServer{})

	go server.ListenAndServe()
	time.Sleep(100 * time.Millisecond)

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: factory.CreateClientTransport(id),
	})

	c := pingpong.NewPingPongClient(client)

	var wg sync.WaitGroup
	var successCount atomic.Int32
	var errorCount atomic.Int32

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

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

	time.Sleep(10 * time.Millisecond)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := server.Shutdown(shutdownCtx)
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Test timed out waiting for client goroutines to finish")
	}

	success := int(successCount.Load())
	errors := int(errorCount.Load())

	t.Logf("After shutdown: %d successful, %d errors", success, errors)
	assert.Equal(t, 10, success+errors, "All requests should complete")
}

// runLargePayloadTest tests handling of large messages
func runLargePayloadTest(t *testing.T, factory TransportFactory, id int, testCases []LargePayloadTestCase) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: factory.CreateServerTransport(id),
	})

	pingpong.RegisterPingPongServer(server, &pingpongServer{})

	go server.ListenAndServe()
	time.Sleep(100 * time.Millisecond)
	defer server.Shutdown(context.Background())

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: factory.CreateClientTransport(id),
	})

	c := pingpong.NewPingPongClient(client)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			largePayload := strings.Repeat("x", tc.Size)

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

			if tc.ShouldPass {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, tc.Size, len(resp.Pong.Payload.ValString))
			} else {
				require.Error(t, err, "Should fail with large payload")
				t.Logf("Large payload correctly rejected: %v", err)
			}
		})
	}
}

// runMultipleClientsTest tests multiple clients connecting to same service
func runMultipleClientsTest(t *testing.T, factory TransportFactory, id int) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: factory.CreateServerTransport(id),
	})

	counterServer := &counterPingPongServerGeneric{}
	pingpong.RegisterPingPongServer(server, counterServer)

	go server.ListenAndServe()
	time.Sleep(100 * time.Millisecond)
	defer server.Shutdown(context.Background())

	const numClients = 10
	const requestsPerClient = 20

	var wg sync.WaitGroup
	var totalSuccess atomic.Int32

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			client := rpc.NewClient(rpc.ClientConfig{
				Transport: factory.CreateClientTransport(id),
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

// runRapidConnectionChurnTest tests creating and destroying connections rapidly
func runRapidConnectionChurnTest(t *testing.T, factory TransportFactory, id int) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: factory.CreateServerTransport(id),
	})

	pingpong.RegisterPingPongServer(server, &pingpongServer{})

	go server.ListenAndServe()
	time.Sleep(100 * time.Millisecond)
	defer server.Shutdown(context.Background())

	const numIterations = 50

	for i := 0; i < numIterations; i++ {
		client := rpc.NewClient(rpc.ClientConfig{
			Transport: factory.CreateClientTransport(id),
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
	}

	t.Log("All rapid connection iterations completed successfully")
}

// runEmptyPayloadTest tests handling of minimal/empty payloads
func runEmptyPayloadTest(t *testing.T, factory TransportFactory, id int, useExternalServer bool) {
	var server *rpc.Server
	if !useExternalServer {
		server = rpc.NewServer(rpc.ServerConfig{
			Transport: factory.CreateServerTransport(id),
		})

		pingpong.RegisterPingPongServer(server, &pingpongServer{})

		go server.ListenAndServe()
		time.Sleep(100 * time.Millisecond)
		defer server.Shutdown(context.Background())
	}

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: factory.CreateClientTransport(id),
	})

	c := pingpong.NewPingPongClient(client)

	// Test with minimal payload (just count, no payload data)
	resp, err := c.Ping(context.Background(), &pingpong.PingRequest{
		Ping: pingpong.Ping{
			Count: 0,
			// Leave payload at default/zero values
		},
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int32(1), resp.Pong.Count)

	// Test with explicit empty payload struct
	resp, err = c.Ping(context.Background(), &pingpong.PingRequest{
		Ping: pingpong.Ping{
			Count:   42,
			Payload: pingpong.TestPayload{},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int32(43), resp.Pong.Count)
}

// runSequentialRequestsTest tests many sequential requests from the same client
func runSequentialRequestsTest(t *testing.T, factory TransportFactory, id int, useExternalServer bool) {
	var server *rpc.Server
	if !useExternalServer {
		server = rpc.NewServer(rpc.ServerConfig{
			Transport: factory.CreateServerTransport(id),
		})

		pingpong.RegisterPingPongServer(server, &pingpongServer{})

		go server.ListenAndServe()
		time.Sleep(100 * time.Millisecond)
		defer server.Shutdown(context.Background())
	}

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: factory.CreateClientTransport(id),
	})

	c := pingpong.NewPingPongClient(client)

	const numRequests = 100

	for i := 0; i < numRequests; i++ {
		resp, err := c.Ping(context.Background(), &pingpong.PingRequest{
			Ping: pingpong.Ping{
				Count: int32(i),
			},
		})

		require.NoError(t, err, "Request %d should succeed", i)
		require.NotNil(t, resp)
		assert.Equal(t, int32(i+1), resp.Pong.Count)
	}

	t.Logf("All %d sequential requests completed successfully", numRequests)
}

// runMaxMessageSizeTest tests that messages exceeding the limit are rejected
func runMaxMessageSizeTest(t *testing.T, factory TransportFactory, id int) {
	// Create a factory wrapper that sets MaxMessageSize
	limitedFactory := &LimitedTransportFactory{
		delegate:           factory,
		maxSendMessageSize: 1024, // 1KB limit
		maxRecvMessageSize: 1024, // 1KB limit
	}

	server := rpc.NewServer(rpc.ServerConfig{
		Transport: limitedFactory.CreateServerTransport(id),
		ErrHandler: func(err error) {
			// Expected error on server side
		},
	})
	pingpong.RegisterPingPongServer(server, &pingpongServer{})

	go func() {
		server.ListenAndServe()
	}()

	time.Sleep(100 * time.Millisecond)
	defer server.Shutdown(context.Background())

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: limitedFactory.CreateClientTransport(id),
		ErrHandler: func(err error) {
			// Expected error on client side
		},
	})

	c := pingpong.NewPingPongClient(client)

	// 1. Send small message (should succeed)
	_, err := c.Ping(context.Background(), &pingpong.PingRequest{
		Ping: pingpong.Ping{
			Count: 1,
			Payload: pingpong.TestPayload{
				ValString: strings.Repeat("a", 100), // 100 bytes
			},
		},
	})
	require.NoError(t, err)

	// 2. Send large message (should fail)
	// Payload > 1024 bytes
	_, err = c.Ping(context.Background(), &pingpong.PingRequest{
		Ping: pingpong.Ping{
			Count: 2,
			Payload: pingpong.TestPayload{
				ValString: strings.Repeat("a", 2000), // 2000 bytes
			},
		},
	})
	require.Error(t, err)
	// The error message might vary depending on whether the client or server detected it first
	// but the connection should be closed
}

func runContextTimeoutTest(t *testing.T, factory TransportFactory, port int) {
	// Start server
	serverTransport := factory.CreateServerTransport(port)
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: serverTransport,
	})

	pingPongSvc := &pingpongServer{}
	pingpong.RegisterPingPongServer(server, pingPongSvc)

	go func() {
		if err := server.ListenAndServe(); err != nil {
			// Ignore transport closed error which happens on shutdown
			if err.Error() != "transport is closed" {
				t.Logf("Server error: %v", err)
			}
		}
	}()
	defer server.Shutdown(context.Background())

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Create client
	clientTransport := factory.CreateClientTransport(port)
	client := rpc.NewClient(rpc.ClientConfig{
		Transport: clientTransport,
	})
	defer client.Close()

	svc := pingpong.NewPingPongClient(client)

	// 1. Test successful call with long deadline
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &pingpong.PingRequest{
		Ping: pingpong.Ping{
			Count: 1,
			Payload: pingpong.TestPayload{
				ValString: "hello",
			},
		},
	}

	_, err := svc.Ping(ctx, req)
	require.NoError(t, err, "Call with long deadline should succeed")

	// 2. Test timeout
	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()

	// Add sleep metadata
	md := rpc.NewMetadata()
	md.PutString("sleep", "500")
	ctx2 = rpc.NewContextWithMetadata(ctx2, md)

	req2 := &pingpong.PingRequest{
		Ping: pingpong.Ping{
			Count: 2,
			Payload: pingpong.TestPayload{
				ValString: "timeout",
			},
		},
	}

	_, err = svc.Ping(ctx2, req2)
	require.Error(t, err, "Call should timeout")
	assert.Contains(t, err.Error(), "context deadline exceeded", "Error should be context deadline exceeded")
}

// runContextTimeoutRecoveryTest verifies that after a context timeout, the client
// can still make successful calls. This catches bugs where a timed-out request
// leaves orphaned state that deadlocks the receive goroutine or disconnects on
// the late response.
func runContextTimeoutRecoveryTest(t *testing.T, factory TransportFactory, port int) {
	serverTransport := factory.CreateServerTransport(port)
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: serverTransport,
	})

	pingPongSvc := &pingpongServer{}
	pingpong.RegisterPingPongServer(server, pingPongSvc)

	go func() {
		if err := server.ListenAndServe(); err != nil {
			if err.Error() != "transport is closed" {
				t.Logf("Server error: %v", err)
			}
		}
	}()
	defer server.Shutdown(context.Background())

	time.Sleep(100 * time.Millisecond)

	clientTransport := factory.CreateClientTransport(port)
	client := rpc.NewClient(rpc.ClientConfig{
		Transport: clientTransport,
	})
	defer client.Close()

	svc := pingpong.NewPingPongClient(client)

	// Call 1: Force a timeout. Server sleeps 500ms, client deadline is 100ms.
	ctx1, cancel1 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel1()

	md := rpc.NewMetadata()
	md.PutString("sleep", "500")
	ctx1 = rpc.NewContextWithMetadata(ctx1, md)

	_, err := svc.Ping(ctx1, &pingpong.PingRequest{
		Ping: pingpong.Ping{Count: 1},
	})
	require.Error(t, err, "Call 1 should timeout")
	assert.Contains(t, err.Error(), "context deadline exceeded")

	// Wait for the server to finish processing and send the late response.
	time.Sleep(600 * time.Millisecond)

	// Call 2: Should succeed. If the client is broken by the orphaned response,
	// this will timeout or error.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel2()

	resp, err := svc.Ping(ctx2, &pingpong.PingRequest{
		Ping: pingpong.Ping{Count: 42},
	})
	require.NoError(t, err, "Call 2 should succeed after timeout recovery")
	assert.Equal(t, int32(43), resp.Pong.Count)
}

func runContextMetadataTest(t *testing.T, factory TransportFactory, port int) {
	// Start server
	serverTransport := factory.CreateServerTransport(port)
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: serverTransport,
	})

	pingPongSvc := &pingpongServer{}
	pingpong.RegisterPingPongServer(server, pingPongSvc)

	go func() {
		if err := server.ListenAndServe(); err != nil {
			if err.Error() != "transport is closed" {
				t.Logf("Server error: %v", err)
			}
		}
	}()
	defer server.Shutdown(context.Background())

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Create client
	clientTransport := factory.CreateClientTransport(port)
	client := rpc.NewClient(rpc.ClientConfig{
		Transport: clientTransport,
	})
	defer client.Close()

	svc := pingpong.NewPingPongClient(client)

	// Test with context metadata
	md := rpc.NewMetadata()
	md.PutString("key1", "value1")
	md.PutString("key2", "value2")
	md.PutString("token", "1234")
	ctx := rpc.NewContextWithMetadata(context.Background(), md)

	req := &pingpong.PingRequest{
		Ping: pingpong.Ping{
			Count: 42,
		},
	}

	res, err := svc.Ping(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, int32(43), res.Pong.Count)
}

// LimitedTransportFactory wraps a TransportFactory to inject MaxMessageSize
type LimitedTransportFactory struct {
	delegate           TransportFactory
	maxSendMessageSize uint32
	maxRecvMessageSize uint32
}

func (f *LimitedTransportFactory) CreateServerTransport(id int) rpc.ServerTransport {
	t := f.delegate.CreateServerTransport(id)
	// Use reflection or type assertion to set MaxMessageSize if possible
	// Since we modified the structs, we can try type assertion
	switch v := t.(type) {
	case *tcp.ServerTransport:
		v.MaxSendMessageSize = f.maxSendMessageSize
		v.MaxRecvMessageSize = f.maxRecvMessageSize
	case *tcp.ServerTransportTLS:
		v.MaxSendMessageSize = f.maxSendMessageSize
		v.MaxRecvMessageSize = f.maxRecvMessageSize
	case *websocket.ServerTransport:
		v.MaxSendMessageSize = f.maxSendMessageSize
		v.MaxRecvMessageSize = f.maxRecvMessageSize
	}
	return t
}

func (f *LimitedTransportFactory) CreateClientTransport(id int) rpc.ClientTransport {
	t := f.delegate.CreateClientTransport(id)
	switch v := t.(type) {
	case *tcp.ClientTransport:
		v.MaxSendMessageSize = f.maxSendMessageSize
		v.MaxRecvMessageSize = f.maxRecvMessageSize
	case *tcp.ClientTransportTLS:
		v.MaxSendMessageSize = f.maxSendMessageSize
		v.MaxRecvMessageSize = f.maxRecvMessageSize
	case *websocket.ClientTransport:
		v.MaxSendMessageSize = f.maxSendMessageSize
		v.MaxRecvMessageSize = f.maxRecvMessageSize
	}
	return t
}

func (f *LimitedTransportFactory) SupportsMultipleServers() bool {
	return f.delegate.SupportsMultipleServers()
}

func (f *LimitedTransportFactory) Name() string {
	return f.delegate.Name() + "Limited"
}

// Helper server implementations

type suiteTesterAServer struct {
}

func (s *suiteTesterAServer) Test(ctx context.Context, req *basic.TestRequestA) (*basic.TestResponseA, error) {
	return &basic.TestResponseA{
		A: req.A,
	}, nil
}

type suiteTesterBServer struct {
}

func (s *suiteTesterBServer) Test(ctx context.Context, req *basic.TestRequestB) (*basic.TestResponseB, error) {
	return &basic.TestResponseB{
		B: req.B,
	}, nil
}

type suiteTesterAServerImpl struct {
	responsePrefix string
	callCount      int
	mu             sync.Mutex
}

func (s *suiteTesterAServerImpl) Test(ctx context.Context, req *basic.TestRequestA) (*basic.TestResponseA, error) {
	s.mu.Lock()
	s.callCount++
	s.mu.Unlock()
	return &basic.TestResponseA{
		A: fmt.Sprintf("%s:%s", s.responsePrefix, req.A),
	}, nil
}

type suiteTesterBServerImpl struct {
	responsePrefix string
	callCount      int
	mu             sync.Mutex
}

func (s *suiteTesterBServerImpl) Test(ctx context.Context, req *basic.TestRequestB) (*basic.TestResponseB, error) {
	s.mu.Lock()
	s.callCount++
	s.mu.Unlock()
	return &basic.TestResponseB{
		B: fmt.Sprintf("%s:%s", s.responsePrefix, req.B),
	}, nil
}

type counterPingPongServerGeneric struct {
	callCount atomic.Int32
}

func (s *counterPingPongServerGeneric) Ping(ctx context.Context, req *pingpong.PingRequest) (*pingpong.PongResponse, error) {
	s.callCount.Add(1)
	return &pingpong.PongResponse{
		Pong: pingpong.Pong{
			Count:   req.Ping.Count + 1,
			Payload: req.Ping.Payload,
		},
	}, nil
}

// ============================================================================
// Streaming tests
// ============================================================================

// The Chat streaming service implementation the streaming tests assert against
// lives in the shared chat package so the standalone cross-language test
// servers behave identically. See chat.ChatServer.

// newStreamingServer starts an in-process server hosting both the unary PingPong
// service and the streaming Chat service.
func newStreamingServer(t *testing.T, factory TransportFactory, id int, middleware ...rpc.Middleware) *rpc.Server {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: factory.CreateServerTransport(id),
		// Streams surface handler errors to the client via Recv, not via the
		// server error handler, so just log here.
		ErrHandler: func(err error) {
			t.Logf("server error: %v", err)
		},
	})
	for _, m := range middleware {
		server.Middleware(m)
	}
	pingpong.RegisterPingPongServer(server, &pingpongServer{})
	pingpong.RegisterChatServer(server, &chat.ChatServer{})

	go func() {
		server.ListenAndServe()
	}()

	time.Sleep(100 * time.Millisecond)
	return server
}

// runStreamBidiTest exercises the full bidi lifecycle: server push on open,
// client send + server echo, half-close, final summary, then a clean io.EOF.
// Runs in-process and against an external (cross-language) server.
func runStreamBidiTest(t *testing.T, factory TransportFactory, id int, useExternalServer bool) {
	if !useExternalServer {
		server := newStreamingServer(t, factory, id)
		defer server.Shutdown(context.Background())
	}

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: factory.CreateClientTransport(id),
	})
	defer client.Close()

	c := pingpong.NewChatClient(client)

	stream, err := c.Connect(context.Background())
	require.NoError(t, err)

	// Server pushes "welcome" before the client sends anything.
	welcome, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, "welcome", welcome.Text)

	const n = 10
	for i := 0; i < n; i++ {
		err := stream.Send(&pingpong.ChatMessage{Text: fmt.Sprintf("m%d", i), Seq: int32(i)})
		require.NoError(t, err)

		echo, err := stream.Recv()
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("echo:m%d", i), echo.Text)
		assert.Equal(t, int32(i+1), echo.Seq)
	}

	// Half-close: done sending, still receiving.
	require.NoError(t, stream.CloseSend())

	summary, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, "summary", summary.Text)
	assert.Equal(t, int32(n), summary.Seq)

	// Clean close surfaces as io.EOF.
	_, err = stream.Recv()
	assert.Equal(t, io.EOF, err)
}

// runStreamServerErrorTest verifies a server-side handler error surfaces on the
// client's next Recv.
func runStreamServerErrorTest(t *testing.T, factory TransportFactory, id int) {
	server := newStreamingServer(t, factory, id)
	defer server.Shutdown(context.Background())

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: factory.CreateClientTransport(id),
	})
	defer client.Close()

	c := pingpong.NewChatClient(client)

	stream, err := c.Connect(context.Background())
	require.NoError(t, err)

	// Drain the welcome push.
	_, err = stream.Recv()
	require.NoError(t, err)

	require.NoError(t, stream.Send(&pingpong.ChatMessage{Text: "fail", Seq: 1}))

	_, err = stream.Recv()
	require.Error(t, err)
	assert.Equal(t, "requested failure", err.Error())
}

// runStreamConcurrentTest opens multiple independent streams on one connection
// and verifies messages are not cross-delivered.
func runStreamConcurrentTest(t *testing.T, factory TransportFactory, id int) {
	server := newStreamingServer(t, factory, id)
	defer server.Shutdown(context.Background())

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: factory.CreateClientTransport(id),
	})
	defer client.Close()

	c := pingpong.NewChatClient(client)

	const numStreams = 8
	const msgsPerStream = 20

	var wg sync.WaitGroup
	var errCount atomic.Int32

	for s := 0; s < numStreams; s++ {
		wg.Add(1)
		go func(streamIdx int) {
			defer wg.Done()

			stream, err := c.Connect(context.Background())
			if err != nil {
				errCount.Add(1)
				return
			}

			welcome, err := stream.Recv()
			if err != nil || welcome.Text != "welcome" {
				errCount.Add(1)
				return
			}

			for i := 0; i < msgsPerStream; i++ {
				text := fmt.Sprintf("s%d-m%d", streamIdx, i)
				if err := stream.Send(&pingpong.ChatMessage{Text: text, Seq: int32(i)}); err != nil {
					errCount.Add(1)
					return
				}
				echo, err := stream.Recv()
				if err != nil || echo.Text != "echo:"+text || echo.Seq != int32(i+1) {
					errCount.Add(1)
					return
				}
			}

			if err := stream.CloseSend(); err != nil {
				errCount.Add(1)
				return
			}
			if _, err := stream.Recv(); err != nil { // summary
				errCount.Add(1)
				return
			}
			if _, err := stream.Recv(); err != io.EOF {
				errCount.Add(1)
				return
			}
		}(s)
	}

	wg.Wait()
	assert.Equal(t, int32(0), errCount.Load(), "all concurrent streams should complete cleanly")
}

// runStreamUnaryCoexistTest verifies unary calls and a stream coexist on the
// same connection concurrently.
func runStreamUnaryCoexistTest(t *testing.T, factory TransportFactory, id int) {
	server := newStreamingServer(t, factory, id)
	defer server.Shutdown(context.Background())

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: factory.CreateClientTransport(id),
	})
	defer client.Close()

	pingClient := pingpong.NewPingPongClient(client)
	chatClient := pingpong.NewChatClient(client)

	stream, err := chatClient.Connect(context.Background())
	require.NoError(t, err)

	welcome, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, "welcome", welcome.Text)

	// Interleave a unary call with stream traffic.
	for i := 0; i < 5; i++ {
		resp, err := pingClient.Ping(context.Background(), &pingpong.PingRequest{
			Ping: pingpong.Ping{Count: int32(i)},
		})
		require.NoError(t, err)
		assert.Equal(t, int32(i+1), resp.Pong.Count)

		require.NoError(t, stream.Send(&pingpong.ChatMessage{Text: fmt.Sprintf("x%d", i), Seq: int32(i)}))
		echo, err := stream.Recv()
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("echo:x%d", i), echo.Text)
	}

	require.NoError(t, stream.CloseSend())
	_, err = stream.Recv() // summary
	require.NoError(t, err)
	_, err = stream.Recv()
	assert.Equal(t, io.EOF, err)
}

// runStreamCancelTest verifies cancelling the caller context terminates the
// client side of the stream.
func runStreamCancelTest(t *testing.T, factory TransportFactory, id int) {
	server := newStreamingServer(t, factory, id)
	defer server.Shutdown(context.Background())

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: factory.CreateClientTransport(id),
	})
	defer client.Close()

	c := pingpong.NewChatClient(client)

	ctx, cancel := context.WithCancel(context.Background())
	stream, err := c.Connect(ctx)
	require.NoError(t, err)

	_, err = stream.Recv() // welcome
	require.NoError(t, err)

	cancel()

	_, err = stream.Recv()
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// runStreamConnectionDropTest verifies that closing the client connection fails
// in-flight streams with an error.
func runStreamConnectionDropTest(t *testing.T, factory TransportFactory, id int) {
	server := newStreamingServer(t, factory, id)
	defer server.Shutdown(context.Background())

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: factory.CreateClientTransport(id),
	})

	c := pingpong.NewChatClient(client)

	stream, err := c.Connect(context.Background())
	require.NoError(t, err)

	_, err = stream.Recv() // welcome
	require.NoError(t, err)

	// Drop the connection out from under the live stream.
	require.NoError(t, client.Close())

	_, err = stream.Recv()
	require.Error(t, err)
}

// runStreamAuthFailTest verifies stream OPEN is gated by server middleware
// (auth is validated once on open).
func runStreamAuthFailTest(t *testing.T, factory TransportFactory, id int) {
	server := newStreamingServer(t, factory, id, authMiddleware)
	defer server.Shutdown(context.Background())

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: factory.CreateClientTransport(id),
	})
	defer client.Close()

	c := pingpong.NewChatClient(client)

	// No token in context -> auth middleware rejects on OPEN.
	stream, err := c.Connect(context.Background())
	require.NoError(t, err)

	_, err = stream.Recv()
	require.Error(t, err)
	assert.Equal(t, "no metadata", err.Error())

	// With a valid token the stream opens and the welcome push arrives.
	md := rpc.NewMetadata()
	md.PutString("token", validToken)
	ctx := rpc.NewContextWithMetadata(context.Background(), md)

	stream2, err := c.Connect(ctx)
	require.NoError(t, err)
	welcome, err := stream2.Recv()
	require.NoError(t, err)
	assert.Equal(t, "welcome", welcome.Text)
}

// runStreamConcurrentSendRecvTest sends from one goroutine while receiving on
// another on the same stream, exercising both paths concurrently (run -race).
func runStreamConcurrentSendRecvTest(t *testing.T, factory TransportFactory, id int) {
	server := newStreamingServer(t, factory, id)
	defer server.Shutdown(context.Background())

	client := rpc.NewClient(rpc.ClientConfig{Transport: factory.CreateClientTransport(id)})
	defer client.Close()

	c := pingpong.NewChatClient(client)
	stream, err := c.Connect(context.Background())
	require.NoError(t, err)

	const n = 200
	go func() {
		for i := 0; i < n; i++ {
			_ = stream.Send(&pingpong.ChatMessage{Text: fmt.Sprintf("m%d", i), Seq: int32(i)})
		}
		_ = stream.CloseSend()
	}()

	echoes := 0
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if msg.Text != "welcome" && msg.Text != "summary" {
			echoes++
		}
	}
	assert.Equal(t, n, echoes)
}

// runStreamSendAfterCloseSendTest verifies Send after CloseSend returns an error.
func runStreamSendAfterCloseSendTest(t *testing.T, factory TransportFactory, id int) {
	server := newStreamingServer(t, factory, id)
	defer server.Shutdown(context.Background())

	client := rpc.NewClient(rpc.ClientConfig{Transport: factory.CreateClientTransport(id)})
	defer client.Close()

	c := pingpong.NewChatClient(client)
	stream, err := c.Connect(context.Background())
	require.NoError(t, err)

	_, err = stream.Recv() // welcome
	require.NoError(t, err)

	require.NoError(t, stream.CloseSend())
	err = stream.Send(&pingpong.ChatMessage{Text: "late"})
	require.Error(t, err)
}

// runStreamLargeMessageTest verifies a large payload round-trips over a stream.
func runStreamLargeMessageTest(t *testing.T, factory TransportFactory, id int) {
	server := newStreamingServer(t, factory, id)
	defer server.Shutdown(context.Background())

	client := rpc.NewClient(rpc.ClientConfig{Transport: factory.CreateClientTransport(id)})
	defer client.Close()

	c := pingpong.NewChatClient(client)
	stream, err := c.Connect(context.Background())
	require.NoError(t, err)

	_, err = stream.Recv() // welcome
	require.NoError(t, err)

	big := strings.Repeat("x", 256*1024)
	require.NoError(t, stream.Send(&pingpong.ChatMessage{Text: big, Seq: 1}))

	echo, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, "echo:"+big, echo.Text)
}

// runStreamRecvAfterEOFTest verifies the terminal state is sticky: once a stream
// is cleanly closed, further Recv calls keep returning io.EOF.
func runStreamRecvAfterEOFTest(t *testing.T, factory TransportFactory, id int) {
	server := newStreamingServer(t, factory, id)
	defer server.Shutdown(context.Background())

	client := rpc.NewClient(rpc.ClientConfig{Transport: factory.CreateClientTransport(id)})
	defer client.Close()

	c := pingpong.NewChatClient(client)
	stream, err := c.Connect(context.Background())
	require.NoError(t, err)

	_, err = stream.Recv() // welcome
	require.NoError(t, err)
	require.NoError(t, stream.CloseSend())
	_, err = stream.Recv() // summary
	require.NoError(t, err)

	_, err = stream.Recv()
	assert.Equal(t, io.EOF, err)
	// Terminal is sticky.
	_, err = stream.Recv()
	assert.Equal(t, io.EOF, err)
}

// runStreamBackpressureTest verifies that a slow reader whose bounded buffer
// overflows has its stream terminated with an overflow error (and not the
// connection or other streams).
func runStreamBackpressureTest(t *testing.T, factory TransportFactory, id int) {
	server := newStreamingServer(t, factory, id)
	defer server.Shutdown(context.Background())

	// Tiny receive buffer so a flood overflows quickly.
	client := rpc.NewClient(rpc.ClientConfig{
		Transport:            factory.CreateClientTransport(id),
		StreamRecvBufferSize: 4,
	})
	defer client.Close()

	c := pingpong.NewChatClient(client)
	stream, err := c.Connect(context.Background())
	require.NoError(t, err)

	// Trigger a server flood, then deliberately don't read for a moment so the
	// bounded buffer overflows.
	require.NoError(t, stream.Send(&pingpong.ChatMessage{Text: "flood"}))
	time.Sleep(250 * time.Millisecond)

	var lastErr error
	for i := 0; i < 200; i++ {
		if _, err := stream.Recv(); err != nil {
			lastErr = err
			break
		}
	}
	require.Error(t, lastErr)
	assert.Contains(t, lastErr.Error(), "overflow")

	// The connection is still usable for new streams/calls.
	stream2, err := c.Connect(context.Background())
	require.NoError(t, err)
	welcome, err := stream2.Recv()
	require.NoError(t, err)
	assert.Equal(t, "welcome", welcome.Text)
}

// runStreamMaxConcurrentTest verifies the per-connection stream cap rejects
// streams beyond the limit while existing streams keep working.
func runStreamMaxConcurrentTest(t *testing.T, factory TransportFactory, id int) {
	server := rpc.NewServer(rpc.ServerConfig{
		Transport:            factory.CreateServerTransport(id),
		ErrHandler:           func(err error) { t.Logf("server error: %v", err) },
		MaxConcurrentStreams: 2,
	})
	pingpong.RegisterPingPongServer(server, &pingpongServer{})
	pingpong.RegisterChatServer(server, &chat.ChatServer{})
	go func() { server.ListenAndServe() }()
	time.Sleep(100 * time.Millisecond)
	defer server.Shutdown(context.Background())

	client := rpc.NewClient(rpc.ClientConfig{Transport: factory.CreateClientTransport(id)})
	defer client.Close()

	c := pingpong.NewChatClient(client)

	// Two streams open and stay open (handlers block on Recv).
	s1, err := c.Connect(context.Background())
	require.NoError(t, err)
	w1, err := s1.Recv()
	require.NoError(t, err)
	assert.Equal(t, "welcome", w1.Text)

	s2, err := c.Connect(context.Background())
	require.NoError(t, err)
	w2, err := s2.Recv()
	require.NoError(t, err)
	assert.Equal(t, "welcome", w2.Text)

	// Third exceeds the cap and is rejected on open.
	s3, err := c.Connect(context.Background())
	require.NoError(t, err)
	_, err = s3.Recv()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max concurrent streams")

	// Freeing a slot lets a new stream open.
	require.NoError(t, s1.CloseSend())
	_, _ = s1.Recv() // summary
	_, _ = s1.Recv() // EOF
	time.Sleep(50 * time.Millisecond)

	s4, err := c.Connect(context.Background())
	require.NoError(t, err)
	w4, err := s4.Recv()
	require.NoError(t, err)
	assert.Equal(t, "welcome", w4.Text)
}

// runStreamServerStreamingTest exercises the server-streaming form: the client
// sends a single request and reads a stream of responses.
// Runs in-process and against an external (cross-language) server.
func runStreamServerStreamingTest(t *testing.T, factory TransportFactory, id int, useExternalServer bool) {
	if !useExternalServer {
		server := newStreamingServer(t, factory, id)
		defer server.Shutdown(context.Background())
	}

	client := rpc.NewClient(rpc.ClientConfig{Transport: factory.CreateClientTransport(id)})
	defer client.Close()

	c := pingpong.NewChatClient(client)
	stream, err := c.Subscribe(context.Background(), &pingpong.SubscribeRequest{Count: 5})
	require.NoError(t, err)

	var got []int32
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		got = append(got, msg.Seq)
	}
	require.Equal(t, []int32{0, 1, 2, 3, 4}, got)
}

// runStreamClientStreamingTest exercises the client-streaming form: the client
// sends a stream of requests and reads a single response.
// Runs in-process and against an external (cross-language) server.
func runStreamClientStreamingTest(t *testing.T, factory TransportFactory, id int, useExternalServer bool) {
	if !useExternalServer {
		server := newStreamingServer(t, factory, id)
		defer server.Shutdown(context.Background())
	}

	client := rpc.NewClient(rpc.ClientConfig{Transport: factory.CreateClientTransport(id)})
	defer client.Close()

	c := pingpong.NewChatClient(client)
	stream, err := c.Upload(context.Background())
	require.NoError(t, err)

	var sum int32
	for i := int32(1); i <= 5; i++ {
		require.NoError(t, stream.Send(&pingpong.ChatMessage{Seq: i}))
		sum += i
	}

	resp, err := stream.CloseAndRecv()
	require.NoError(t, err)
	assert.Equal(t, sum, resp.Total)
}

// runStreamKeepaliveTest verifies that connection-level keepalive keeps an idle
// stream healthy (PING/PONG flow without disrupting the stream).
func runStreamKeepaliveTest(t *testing.T, factory TransportFactory, id int) {
	server := newStreamingServer(t, factory, id)
	defer server.Shutdown(context.Background())

	client := rpc.NewClient(rpc.ClientConfig{
		Transport:         factory.CreateClientTransport(id),
		KeepaliveInterval: 40 * time.Millisecond,
		KeepaliveTimeout:  500 * time.Millisecond,
	})
	defer client.Close()

	c := pingpong.NewChatClient(client)
	stream, err := c.Connect(context.Background())
	require.NoError(t, err)

	welcome, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, "welcome", welcome.Text)

	// Idle well beyond the keepalive interval; pings keep the connection alive.
	time.Sleep(250 * time.Millisecond)

	// The stream is still usable.
	require.NoError(t, stream.Send(&pingpong.ChatMessage{Text: "after-idle", Seq: 1}))
	echo, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, "echo:after-idle", echo.Text)
}
