package test

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/rpc/tcp"
	"github.com/kbirk/scg/test/scg/generated/pingpong"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStreamKeepaliveTimeout verifies that keepalive detects a half-open / dead
// peer: the client connects to a black-hole server that accepts and reads but
// never responds, so no PONG (or any frame) ever arrives and the keepalive
// timeout fails the in-flight stream.
func TestStreamKeepaliveTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			// Read and discard everything; never write back.
			go func(c net.Conn) {
				_, _ = io.Copy(io.Discard, c)
			}(conn)
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)
	client := rpc.NewClient(rpc.ClientConfig{
		Transport: tcp.NewClientTransport(tcp.ClientTransportConfig{
			Host: "127.0.0.1",
			Port: addr.Port,
		}),
		KeepaliveInterval: 40 * time.Millisecond,
		KeepaliveTimeout:  150 * time.Millisecond,
	})
	defer client.Close()

	c := pingpong.NewChatClient(client)
	stream, err := c.Connect(context.Background())
	require.NoError(t, err)

	// The black-hole never sends the welcome (or anything); keepalive should time
	// out and fail the stream within a few intervals.
	done := make(chan error, 1)
	go func() {
		_, err := stream.Recv()
		done <- err
	}()

	select {
	case err := <-done:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "keepalive timeout")
	case <-time.After(2 * time.Second):
		t.Fatal("keepalive did not time out the stream")
	}
}

// TestStreamKeepaliveReconnect verifies keepalive resumes after a reconnect on
// the same client: connection 1 times out, and a second stream (which triggers a
// reconnect) must ALSO time out — proving keepalive was re-established and that a
// stale connection's teardown doesn't kill the new connection's stream.
func TestStreamKeepaliveReconnect(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				_, _ = io.Copy(io.Discard, c)
			}(conn)
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)
	client := rpc.NewClient(rpc.ClientConfig{
		Transport: tcp.NewClientTransport(tcp.ClientTransportConfig{
			Host: "127.0.0.1",
			Port: addr.Port,
		}),
		KeepaliveInterval: 40 * time.Millisecond,
		KeepaliveTimeout:  150 * time.Millisecond,
	})
	defer client.Close()

	c := pingpong.NewChatClient(client)

	expectTimeout := func(label string) {
		stream, err := c.Connect(context.Background())
		require.NoError(t, err, label)
		done := make(chan error, 1)
		go func() {
			_, e := stream.Recv()
			done <- e
		}()
		select {
		case e := <-done:
			require.Error(t, e, label)
			assert.Contains(t, e.Error(), "keepalive timeout", label)
		case <-time.After(2 * time.Second):
			t.Fatalf("%s: keepalive did not time out the stream", label)
		}
	}

	expectTimeout("connection 1")
	expectTimeout("connection 2 (reconnect)")
}
