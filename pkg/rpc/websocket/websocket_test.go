package websocket

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/serialize"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func listenLocal(t *testing.T, conf ServerTransportConfig) (*ServerTransport, int) {
	t.Helper()
	conf.Port = 0
	st := NewServerTransport(conf)
	require.NoError(t, st.Listen())
	t.Cleanup(func() { st.Close() })
	return st, st.addr.(*net.TCPAddr).Port
}

func acceptOne(t *testing.T, st *ServerTransport) rpc.Connection {
	t.Helper()
	ch := make(chan rpc.Connection, 1)
	go func() {
		if c, err := st.Accept(); err == nil {
			ch <- c
		}
	}()
	select {
	case c := <-ch:
		t.Cleanup(func() { c.Close() })
		return c
	case <-time.After(3 * time.Second):
		t.Fatal("no connection accepted")
		return nil
	}
}

func dialLocal(t *testing.T, port int, conf ClientTransportConfig) rpc.Connection {
	t.Helper()
	conf.Host = "127.0.0.1"
	conf.Port = port
	c, err := NewClientTransport(conf).ConnectContext(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { c.Close() })
	return c
}

func TestRoundTrip(t *testing.T) {
	st, port := listenLocal(t, ServerTransportConfig{})
	client := dialLocal(t, port, ClientTransportConfig{})
	server := acceptOne(t, st)

	for _, n := range []int{0, 1, 1000, 100 << 10, 2 << 20} {
		payload := bytes.Repeat([]byte{byte(n)}, n)
		require.NoError(t, client.Send(payload, 0))
		got, err := server.Receive()
		require.NoError(t, err)
		require.True(t, bytes.Equal(payload, got), "size %d", n)
	}
}

func TestListenReturnsBindErrors(t *testing.T) {
	l, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer l.Close()

	st := NewServerTransport(ServerTransportConfig{Port: l.Addr().(*net.TCPAddr).Port})
	defer st.Close()
	assert.Error(t, st.Listen(), "Listen on a port in use must return the bind error")

	st2 := NewServerTransport(ServerTransportConfig{Port: 0})
	require.NoError(t, st2.Close())
	assert.ErrorIs(t, st2.Listen(), rpc.ErrTransportClosed)

	st3 := NewServerTransport(ServerTransportConfig{Port: 0, CertFile: "/nonexistent.crt", KeyFile: "/nonexistent.key"})
	assert.Error(t, st3.Listen(), "certificate errors must be reported by Listen")
}

// TestCloseIsCleanForPeer: closing either side surfaces ErrConnectionClosed on
// the other (a close frame is sent), not an abnormal-closure error.
func TestCloseIsCleanForPeer(t *testing.T) {
	st, port := listenLocal(t, ServerTransportConfig{})
	client := dialLocal(t, port, ClientTransportConfig{})
	server := acceptOne(t, st)

	require.NoError(t, server.Close())
	_, err := client.Receive()
	assert.ErrorIs(t, err, rpc.ErrConnectionClosed, "got %v", err)
	_, err = server.Receive()
	assert.ErrorIs(t, err, rpc.ErrConnectionClosed, "local Receive after Close: %v", err)
	assert.NoError(t, server.Close(), "Close is idempotent")

	client2 := dialLocal(t, port, ClientTransportConfig{})
	server2 := acceptOne(t, st)
	require.NoError(t, client2.Close())
	_, err = server2.Receive()
	assert.ErrorIs(t, err, rpc.ErrConnectionClosed, "got %v", err)
}

func TestSizeLimits(t *testing.T) {
	st, port := listenLocal(t, ServerTransportConfig{MaxRecvMessageSize: 64})
	client := dialLocal(t, port, ClientTransportConfig{MaxSendMessageSize: 32})
	server := acceptOne(t, st)

	err := client.Send(make([]byte, 33), 0)
	assert.ErrorIs(t, err, rpc.ErrMessageTooLarge)

	client2 := dialLocal(t, port, ClientTransportConfig{})
	server2 := acceptOne(t, st)
	require.NoError(t, client2.Send(make([]byte, 65), 0))
	_, err = server2.Receive()
	assert.Error(t, err, "a frame over the receive limit must be rejected")
	_ = server
}

// TestAcceptCloseRace: connections arriving while the transport closes must
// never hit a send on the closed accept channel (run with -race).
func TestAcceptCloseRace(t *testing.T) {
	var mu sync.Mutex
	var logged strings.Builder
	prev := log.Writer()
	log.SetOutput(writerFunc(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return logged.Write(p)
	}))
	defer log.SetOutput(prev)

	for iter := 0; iter < 30; iter++ {
		st := NewServerTransport(ServerTransportConfig{Port: 0})
		require.NoError(t, st.Listen())
		port := st.addr.(*net.TCPAddr).Port
		var wg sync.WaitGroup
		for c := 0; c < 20; c++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ct := NewClientTransport(ClientTransportConfig{Host: "127.0.0.1", Port: port, HandshakeTimeout: time.Second})
				if conn, err := ct.ConnectContext(context.Background()); err == nil {
					conn.Close()
				}
			}()
		}
		time.Sleep(time.Duration(iter%4) * time.Millisecond)
		st.Close()
		wg.Wait()
	}
	mu.Lock()
	defer mu.Unlock()
	assert.NotContains(t, logged.String(), "panic", "the HTTP handler panicked:\n%s", logged.String())
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func TestConnectContextHonoursDeadline(t *testing.T) {
	// A server that accepts TCP but never answers the HTTP upgrade.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			defer c.Close()
		}
	}()
	ct := NewClientTransport(ClientTransportConfig{Host: "127.0.0.1", Port: l.Addr().(*net.TCPAddr).Port})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err = ct.ConnectContext(ctx)
	require.Error(t, err)
	assert.Less(t, time.Since(start), 2*time.Second)
}

// TestWriteFailureClosesConnection: as for TCP, a failed write (here a
// WriteTimeout to a peer that is not reading) must tear the connection down
// so the peer is not left waiting on a partial frame.
func TestWriteFailureClosesConnection(t *testing.T) {
	st, port := listenLocal(t, ServerTransportConfig{MaxRecvMessageSize: 64 << 20})
	client := dialLocal(t, port, ClientTransportConfig{WriteTimeout: 100 * time.Millisecond})
	server := acceptOne(t, st) // not reading yet

	err := client.Send(make([]byte, 16<<20), 0)
	require.Error(t, err)

	peerErr := make(chan error, 1)
	go func() {
		for {
			if _, err := server.Receive(); err != nil {
				peerErr <- err
				return
			}
		}
	}()
	assert.Error(t, client.Send([]byte("next"), 0), "sends after a failed write must fail")
	select {
	case err := <-peerErr:
		assert.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("the peer is stuck waiting for the rest of a truncated frame")
	}
}

// bigMsg is a Message carrying a large opaque payload.
type bigMsg struct{ n int }

func (m *bigMsg) BitSize() int            { return serialize.BitSizeString(strings.Repeat("x", m.n)) }
func (m *bigMsg) ToJSON() ([]byte, error) { return nil, nil }
func (m *bigMsg) FromJSON([]byte) error   { return nil }
func (m *bigMsg) ToBytes() []byte         { return nil }
func (m *bigMsg) FromBytes([]byte) error  { return nil }
func (m *bigMsg) Serialize(w *serialize.Writer) {
	serialize.SerializeString(w, strings.Repeat("x", m.n))
}
func (m *bigMsg) Deserialize(r *serialize.Reader) error { return nil }

// bigStub answers every request with an 8 MiB response.
type bigStub struct{}

func (bigStub) HandleWrapper(ctx context.Context, mw []rpc.Middleware, requestID uint64, r *serialize.Reader) []byte {
	return rpc.RespondWithMessage(requestID, &bigMsg{n: 8 << 20})
}

func (t *ServerTransport) boundAddr() net.Addr {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.addr
}

// TestShutdownBoundedWithStuckPeers: peers that stop reading leave the
// server's writes (and therefore any graceful close frame) stuck. Shutdown
// must still return within its context deadline (plus a small epsilon):
// connections are closed concurrently and force-closed once ctx expires.
func TestShutdownBoundedWithStuckPeers(t *testing.T) {
	st := NewServerTransport(ServerTransportConfig{Port: 0})
	server := rpc.NewServer(rpc.ServerConfig{Transport: st, ErrHandler: func(error) {}})
	server.RegisterServer(1, "big", bigStub{})
	go server.ListenAndServe()
	var port int
	for i := 0; i < 500 && port == 0; i++ {
		if a := st.boundAddr(); a != nil {
			port = a.(*net.TCPAddr).Port
		}
		time.Sleep(2 * time.Millisecond)
	}
	require.NotZero(t, port)

	const peers = 8
	for i := 0; i < peers; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://127.0.0.1:%d/rpc", port), nil)
		require.NoError(t, err)
		defer conn.Close()
		w := serialize.NewWriter(64)
		rpc.SerializePrefix(w, rpc.RequestPrefix)
		rpc.SerializeContext(w, context.Background())
		serialize.SerializeUInt64(w, uint64(i)) // request id
		serialize.SerializeUInt64(w, 1)         // service id
		serialize.SerializeUInt64(w, 1)         // method id
		require.NoError(t, conn.WriteMessage(websocket.BinaryMessage, w.Bytes()))
		// ...and never read: the server's 8 MiB response write gets stuck.
	}
	time.Sleep(300 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := server.Shutdown(ctx)
	elapsed := time.Since(start)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, elapsed, 500*time.Millisecond, "Shutdown must be bounded by its context, took %s", elapsed)
}
