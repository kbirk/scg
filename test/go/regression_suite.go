package test

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/test/scg/generated/pingpong"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Suite helpers and transport-agnostic regression scenarios. These live in a
// non-test file because service_test_suite.go (also a non-test file) calls
// them, so the package must build without its _test.go files.

// portFactory is implemented by factories whose transports listen on a local
// TCP port, letting tests wait for the server instead of sleeping.
// The TCP and WebSocket factories (defined in their _test.go files) implement
// it in regression_test.go.
type portFactory interface {
	Port(id int) int
}

// waitForPort blocks until something accepts TCP connections on port.
func waitForPort(t testing.TB, port int) {
	t.Helper()
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			c.Close()
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("server on port %d did not start listening", port)
}

// waitForServer waits until the server for factory/id is accepting
// connections, falling back to a short sleep for factories without a port.
func waitForServer(t testing.TB, factory TransportFactory, id int) {
	t.Helper()
	if pf, ok := factory.(portFactory); ok {
		waitForPort(t, pf.Port(id))
		return
	}
	time.Sleep(100 * time.Millisecond)
}

// requireExternalServer skips a test that needs an externally started server
// (e.g. the C++ test server) unless it is reachable or SCG_EXTERNAL_SERVER is
// set, so `go test ./...` does not hang waiting on a server that isn't there.
func requireExternalServer(t *testing.T, port int) {
	t.Helper()
	if os.Getenv("SCG_EXTERNAL_SERVER") != "" {
		return
	}
	c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 300*time.Millisecond)
	if err != nil {
		t.Skipf("external server not reachable on port %d (set SCG_EXTERNAL_SERVER=1 to force): %v", port, err)
	}
	c.Close()
}

func startPingPongServer(t *testing.T, factory TransportFactory, id int, conf rpc.ServerConfig) *rpc.Server {
	t.Helper()
	conf.Transport = factory.CreateServerTransport(id)
	server := rpc.NewServer(conf)
	pingpong.RegisterPingPongServer(server, &pingpongServer{})
	go server.ListenAndServe()
	waitForServer(t, factory, id)
	return server
}

func sleepCtx(ms int) context.Context {
	md := rpc.NewMetadata()
	md.PutString("sleep", strconv.Itoa(ms))
	return rpc.NewContextWithMetadata(context.Background(), md)
}

// runUnknownServiceTest: a call to a service the server doesn't have must get
// an error response, not hang until the caller's deadline.
func runUnknownServiceTest(t *testing.T, factory TransportFactory, id int) {
	server := startPingPongServer(t, factory, id, rpc.ServerConfig{ErrHandler: func(error) {}})
	defer server.Shutdown(context.Background())

	client := rpc.NewClient(rpc.ClientConfig{Transport: factory.CreateClientTransport(id)})
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	_, err := client.Call(ctx, 0xDEADBEEF, 1, &pingpong.PingRequest{})
	require.Error(t, err)
	assert.NotErrorIs(t, err, context.DeadlineExceeded, "unknown service must be answered, not left to time out")
	assert.Contains(t, err.Error(), "not found")
	assert.Less(t, time.Since(start), 2*time.Second)

	// The connection remains usable afterwards.
	resp, err := pingpong.NewPingPongClient(client).Ping(ctx, &pingpong.PingRequest{Ping: pingpong.Ping{Count: 1}})
	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.Pong.Count)
}

// runClientCloseNoErrHandlerTest: a user-initiated Close is not an error.
func runClientCloseNoErrHandlerTest(t *testing.T, factory TransportFactory, id int) {
	server := startPingPongServer(t, factory, id, rpc.ServerConfig{})
	defer server.Shutdown(context.Background())

	var got atomic.Value
	client := rpc.NewClient(rpc.ClientConfig{
		Transport:  factory.CreateClientTransport(id),
		ErrHandler: func(err error) { got.Store(err.Error()) },
	})
	pp := pingpong.NewPingPongClient(client)
	_, err := pp.Ping(context.Background(), &pingpong.PingRequest{})
	require.NoError(t, err)

	require.NoError(t, client.Close())
	time.Sleep(200 * time.Millisecond)
	assert.Nil(t, got.Load(), "ErrHandler must not be invoked by Close()")

	// The client reconnects transparently after Close.
	_, err = pp.Ping(context.Background(), &pingpong.PingRequest{})
	require.NoError(t, err)
	require.NoError(t, client.Close())
}

// runShutdownClosesConnectionsTest: after Shutdown an existing connection is
// no longer served, and Shutdown returns promptly.
func runShutdownClosesConnectionsTest(t *testing.T, factory TransportFactory, id int) {
	server := startPingPongServer(t, factory, id, rpc.ServerConfig{})

	var errHandlerCalls atomic.Int32
	client := rpc.NewClient(rpc.ClientConfig{
		Transport:  factory.CreateClientTransport(id),
		ErrHandler: func(error) { errHandlerCalls.Add(1) },
	})
	defer client.Close()
	pp := pingpong.NewPingPongClient(client)
	_, err := pp.Ping(context.Background(), &pingpong.PingRequest{})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	require.NoError(t, server.Shutdown(ctx))
	assert.Less(t, time.Since(start), 2*time.Second)

	// The server closes the connection cleanly: the idle client observes a
	// normal close, not an error. (A send racing the close may legitimately
	// fail, so this is checked before issuing another call.)
	time.Sleep(200 * time.Millisecond)
	assert.Zero(t, errHandlerCalls.Load(), "a server shutdown is a clean close, not a client error")

	callCtx, callCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer callCancel()
	_, err = pp.Ping(callCtx, &pingpong.PingRequest{})
	require.Error(t, err, "a shut-down server must not keep serving existing connections")
	assert.NotErrorIs(t, err, context.DeadlineExceeded, "the call should fail fast, not hang")
}

// runShutdownDrainsInFlightTest: an in-flight call completes during a graceful
// Shutdown, and Shutdown waits for it.
func runShutdownDrainsInFlightTest(t *testing.T, factory TransportFactory, id int) {
	server := startPingPongServer(t, factory, id, rpc.ServerConfig{})

	client := rpc.NewClient(rpc.ClientConfig{Transport: factory.CreateClientTransport(id)})
	defer client.Close()
	pp := pingpong.NewPingPongClient(client)
	_, err := pp.Ping(context.Background(), &pingpong.PingRequest{}) // connect
	require.NoError(t, err)

	result := make(chan error, 1)
	go func() {
		resp, err := pp.Ping(sleepCtx(400), &pingpong.PingRequest{Ping: pingpong.Ping{Count: 9}})
		if err == nil && resp.Pong.Count != 10 {
			err = fmt.Errorf("unexpected count %d", resp.Pong.Count)
		}
		result <- err
	}()
	time.Sleep(100 * time.Millisecond) // the call is now in the handler

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	require.NoError(t, server.Shutdown(ctx))
	assert.GreaterOrEqual(t, time.Since(start), 200*time.Millisecond, "Shutdown should wait for the in-flight handler")

	select {
	case err := <-result:
		require.NoError(t, err, "the in-flight call should complete during a graceful shutdown")
	case <-time.After(3 * time.Second):
		t.Fatal("in-flight call did not complete")
	}
}

// runMiddlewareAddedLaterTest: middleware registered after the first request
// applies to subsequent requests (no stale cached chain).
func runMiddlewareAddedLaterTest(t *testing.T, factory TransportFactory, id int) {
	server := startPingPongServer(t, factory, id, rpc.ServerConfig{})
	defer server.Shutdown(context.Background())

	client := rpc.NewClient(rpc.ClientConfig{Transport: factory.CreateClientTransport(id)})
	defer client.Close()
	pp := pingpong.NewPingPongClient(client)

	_, err := pp.Ping(context.Background(), &pingpong.PingRequest{})
	require.NoError(t, err)

	server.Middleware(alwaysRejectMiddleware)

	_, err = pp.Ping(context.Background(), &pingpong.PingRequest{})
	require.Error(t, err, "middleware added after the first request must apply")
	assert.Contains(t, err.Error(), "rejected")
}

// runUnaryContextCancelledOnDisconnectTest: a unary handler's context is
// cancelled when its client disconnects.
func runUnaryContextCancelledOnDisconnectTest(t *testing.T, factory TransportFactory, id int) {
	entered := make(chan struct{})
	cancelled := make(chan struct{})
	server := rpc.NewServer(rpc.ServerConfig{Transport: factory.CreateServerTransport(id)})
	server.Middleware(func(ctx context.Context, req rpc.Message, next rpc.Handler) (rpc.Message, error) {
		close(entered)
		select {
		case <-ctx.Done():
			close(cancelled)
		case <-time.After(5 * time.Second):
		}
		return nil, ctx.Err()
	})
	pingpong.RegisterPingPongServer(server, &pingpongServer{})
	go server.ListenAndServe()
	waitForServer(t, factory, id)
	defer server.Shutdown(context.Background())

	client := rpc.NewClient(rpc.ClientConfig{Transport: factory.CreateClientTransport(id)})
	go pingpong.NewPingPongClient(client).Ping(context.Background(), &pingpong.PingRequest{})

	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("handler not reached")
	}
	client.Close()
	select {
	case <-cancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("handler context was not cancelled after the client disconnected")
	}
}
