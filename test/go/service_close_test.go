package test

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/rpc/tcp"
	"github.com/kbirk/scg/test/scg/generated/pingpong"

	"github.com/stretchr/testify/require"
)

// TestCloseNotifiesPendingRequestOnCleanServerClose verifies the pending-request
// notification in Client.Close().
//
// This exercises the one path where that notification is load-bearing: a clean
// server-side close. When the peer closes cleanly, the client's receive
// goroutine reads io.EOF, maps it to "connection closed", and treats it as a
// NORMAL exit — it returns WITHOUT calling handleError, so the in-flight unary
// request is never failed by the receive loop. Close() must fail it instead.
//
// (Note: a *local* client.Close() over a real transport does NOT exercise this —
// the local close makes Receive return "use of closed network connection", which
// routes through handleError, and handleError independently notifies pending
// requests. So a plain "Close unblocks a call" test passes with or without the
// notification. The clean remote close below is what isolates it.)
func TestCloseNotifiesPendingRequestOnCleanServerClose(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	serverClosed := make(chan struct{})

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		// Read exactly one framed request (4-byte big-endian length + body) so
		// the client's request is fully sent and registered, then close cleanly
		// WITHOUT responding. The clean FIN gives the client an io.EOF.
		header := make([]byte, 4)
		if _, err := io.ReadFull(conn, header); err != nil {
			conn.Close()
			close(serverClosed)
			return
		}
		body := make([]byte, binary.BigEndian.Uint32(header))
		_, _ = io.ReadFull(conn, body)
		conn.Close()
		close(serverClosed)
	}()

	addr := ln.Addr().(*net.TCPAddr)
	client := rpc.NewClient(rpc.ClientConfig{
		Transport: tcp.NewClientTransport(tcp.ClientTransportConfig{
			Host: "127.0.0.1",
			Port: addr.Port,
		}),
		// Keepalive disabled: the request must be unblocked by Close, not by a
		// keepalive timeout.
	})

	svc := pingpong.NewPingPongClient(client)

	// Deadline-less context: the only things that can unblock this call are a
	// response (never sent) or Close() notifying the pending request.
	done := make(chan error, 1)
	go func() {
		_, err := svc.Ping(context.Background(), &pingpong.PingRequest{
			Ping: pingpong.Ping{Count: 1},
		})
		done <- err
	}()

	// Wait until the server has read the request and closed the connection, so
	// the client's receive goroutine has taken the normal-exit path.
	select {
	case <-serverClosed:
	case <-time.After(2 * time.Second):
		t.Fatal("server never accepted/closed the connection")
	}
	// Give the receive goroutine a beat to observe the EOF and exit.
	time.Sleep(100 * time.Millisecond)

	require.NoError(t, client.Close())

	select {
	case err := <-done:
		require.Error(t, err, "in-flight call must return an error once the client is closed")
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not unblock the in-flight request after a clean server-side close")
	}
}
