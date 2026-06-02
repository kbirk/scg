package test

import (
	"context"
	"io"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/rpc/tcp"
	wsrpc "github.com/kbirk/scg/pkg/rpc/websocket"
	"github.com/kbirk/scg/pkg/serialize"
	"github.com/kbirk/scg/test/go/chat"
	"github.com/kbirk/scg/test/scg/generated/pingpong"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The adversarial tests (raw/black-hole peers) cannot use the factory-driven
// RunTestSuite, because that builds well-behaved scg servers. Instead they are
// parameterized over a transportProbe so every case runs identically on TCP and
// WebSocket — the two transports chat runs on — keeping coverage uniform.

// rawConn is a hand-built peer that speaks the framing directly: TCP prepends a
// length header per frame; WS sends one scg frame per binary message.
type rawConn interface {
	// sendFrame writes one scg frame (already-serialized bytes, no length header).
	sendFrame(t *testing.T, payload []byte)
	// closedWithin drains anything the peer sends and reports whether the peer
	// closed the connection within d (a read error).
	closedWithin(t *testing.T, d time.Duration) bool
	close()
}

type transportProbe struct {
	name string
	// serverTransport builds a real scg server transport on the given port.
	serverTransport func(port int) rpc.ServerTransport
	// clientTransport builds a real scg client transport to host:port.
	clientTransport func(host string, port int) rpc.ClientTransport
	// startBlackHole starts a peer that accepts and reads but never replies.
	startBlackHole func(t *testing.T) (host string, port int, stop func())
	// dialRaw connects a raw client that can inject frames and observe closure.
	dialRaw func(t *testing.T, host string, port int) rawConn
}

// ---- TCP probe ----

type tcpRawConn struct{ conn net.Conn }

func (c *tcpRawConn) sendFrame(t *testing.T, payload []byte) {
	t.Helper()
	hdr := make([]byte, 4)
	hdr[0] = byte(len(payload) >> 24)
	hdr[1] = byte(len(payload) >> 16)
	hdr[2] = byte(len(payload) >> 8)
	hdr[3] = byte(len(payload))
	_, err := c.conn.Write(hdr)
	require.NoError(t, err)
	_, err = c.conn.Write(payload)
	require.NoError(t, err)
}

func (c *tcpRawConn) closedWithin(t *testing.T, d time.Duration) bool {
	_ = c.conn.SetReadDeadline(time.Now().Add(d))
	buf := make([]byte, 256)
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if _, err := c.conn.Read(buf); err != nil {
			return true
		}
	}
	return false
}

func (c *tcpRawConn) close() { c.conn.Close() }

func tcpProbe() transportProbe {
	return transportProbe{
		name: "tcp",
		serverTransport: func(port int) rpc.ServerTransport {
			return tcp.NewServerTransport(tcp.ServerTransportConfig{Port: port, NoDelay: true})
		},
		clientTransport: func(host string, port int) rpc.ClientTransport {
			return tcp.NewClientTransport(tcp.ClientTransportConfig{Host: host, Port: port, NoDelay: true})
		},
		startBlackHole: func(t *testing.T) (string, int, func()) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			go func() {
				for {
					conn, err := ln.Accept()
					if err != nil {
						return
					}
					go func(c net.Conn) { _, _ = io.Copy(io.Discard, c) }(conn)
				}
			}()
			addr := ln.Addr().(*net.TCPAddr)
			return "127.0.0.1", addr.Port, func() { ln.Close() }
		},
		dialRaw: func(t *testing.T, host string, port int) rawConn {
			conn, err := net.Dial("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
			require.NoError(t, err)
			return &tcpRawConn{conn: conn}
		},
	}
}

// ---- WebSocket probe ----

type wsRawConn struct{ conn *websocket.Conn }

func (c *wsRawConn) sendFrame(t *testing.T, payload []byte) {
	t.Helper()
	require.NoError(t, c.conn.WriteMessage(websocket.BinaryMessage, payload))
}

func (c *wsRawConn) closedWithin(t *testing.T, d time.Duration) bool {
	_ = c.conn.SetReadDeadline(time.Now().Add(d))
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return true
		}
	}
	return false
}

func (c *wsRawConn) close() { c.conn.Close() }

func wsProbe() transportProbe {
	return transportProbe{
		name: "ws",
		serverTransport: func(port int) rpc.ServerTransport {
			return wsrpc.NewServerTransport(wsrpc.ServerTransportConfig{Port: port})
		},
		clientTransport: func(host string, port int) rpc.ClientTransport {
			return wsrpc.NewClientTransport(wsrpc.ClientTransportConfig{Host: host, Port: port})
		},
		startBlackHole: func(t *testing.T) (string, int, func()) {
			upgrader := websocket.Upgrader{}
			mux := http.NewServeMux()
			mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					return
				}
				for {
					if _, _, err := conn.ReadMessage(); err != nil {
						return
					}
				}
			})
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			srv := &http.Server{Handler: mux}
			go srv.Serve(ln)
			addr := ln.Addr().(*net.TCPAddr)
			return "127.0.0.1", addr.Port, func() { srv.Close(); ln.Close() }
		},
		dialRaw: func(t *testing.T, host string, port int) rawConn {
			u := "ws://" + net.JoinHostPort(host, strconv.Itoa(port)) + "/rpc"
			conn, _, err := websocket.DefaultDialer.Dial(u, nil)
			require.NoError(t, err)
			return &wsRawConn{conn: conn}
		},
	}
}

func allProbes() []transportProbe { return []transportProbe{tcpProbe(), wsProbe()} }

// ---- Frame builders (transport-independent scg framing) ----

func streamFrame(streamID uint64, frameKind uint8, tail []byte) []byte {
	w := serialize.NewWriter(64)
	rpc.SerializePrefix(w, rpc.StreamPrefix)
	serialize.SerializeUInt64(w, streamID)
	serialize.SerializeUInt8(w, frameKind)
	out := w.Bytes()
	return append(out, tail...)
}

// ---- Tests (each runs on every transport) ----

// TestKeepaliveTimeout: keepalive must detect a dead peer (a black-hole that
// never replies) and fail the in-flight stream, on every transport.
func TestKeepaliveTimeout(t *testing.T) {
	for _, p := range allProbes() {
		p := p
		t.Run(p.name, func(t *testing.T) {
			host, port, stop := p.startBlackHole(t)
			defer stop()

			client := rpc.NewClient(rpc.ClientConfig{
				Transport:         p.clientTransport(host, port),
				KeepaliveInterval: 40 * time.Millisecond,
				KeepaliveTimeout:  150 * time.Millisecond,
			})
			defer client.Close()

			stream, err := pingpong.NewChatClient(client).Connect(context.Background())
			require.NoError(t, err)

			done := make(chan error, 1)
			go func() { _, e := stream.Recv(); done <- e }()
			select {
			case e := <-done:
				require.Error(t, e)
				assert.Contains(t, e.Error(), "keepalive timeout")
			case <-time.After(2 * time.Second):
				t.Fatal("keepalive did not time out the stream")
			}
		})
	}
}

// TestKeepaliveReconnect: keepalive must resume after a reconnect on the same
// client — a second stream (which reconnects) must also time out.
func TestKeepaliveReconnect(t *testing.T) {
	for _, p := range allProbes() {
		p := p
		t.Run(p.name, func(t *testing.T) {
			host, port, stop := p.startBlackHole(t)
			defer stop()

			client := rpc.NewClient(rpc.ClientConfig{
				Transport:         p.clientTransport(host, port),
				KeepaliveInterval: 40 * time.Millisecond,
				KeepaliveTimeout:  150 * time.Millisecond,
			})
			defer client.Close()

			c := pingpong.NewChatClient(client)
			expectTimeout := func(label string) {
				stream, err := c.Connect(context.Background())
				require.NoError(t, err, label)
				done := make(chan error, 1)
				go func() { _, e := stream.Recv(); done <- e }()
				select {
				case e := <-done:
					require.Error(t, e, label)
					assert.Contains(t, e.Error(), "keepalive timeout", label)
				case <-time.After(2 * time.Second):
					t.Fatalf("%s: keepalive did not time out", label)
				}
			}
			expectTimeout("connection 1")
			expectTimeout("connection 2 (reconnect)")
		})
	}
}

// TestServerKeepaliveDeadClient: the server's keepalive must tear down a
// connection whose client vanished (never replies to PINGs), on every transport.
func TestServerKeepaliveDeadClient(t *testing.T) {
	for i, p := range allProbes() {
		p := p
		port := 18810 + i
		t.Run(p.name, func(t *testing.T) {
			server := rpc.NewServer(rpc.ServerConfig{
				Transport:         p.serverTransport(port),
				ErrHandler:        func(err error) {},
				KeepaliveInterval: 40 * time.Millisecond,
				KeepaliveTimeout:  150 * time.Millisecond,
			})
			pingpong.RegisterPingPongServer(server, &pingpongServer{})
			pingpong.RegisterChatServer(server, &chat.ChatServer{})
			go func() { server.ListenAndServe() }()
			defer server.Shutdown(context.Background())
			time.Sleep(150 * time.Millisecond)

			raw := p.dialRaw(t, "127.0.0.1", port)
			defer raw.close()

			start := time.Now()
			require.True(t, raw.closedWithin(t, 2*time.Second),
				"server did not close the dead client connection")
			require.Less(t, time.Since(start), 1500*time.Millisecond,
				"server took too long to close the dead client connection")
		})
	}
}

// TestMalformedFrames: a server must survive a grab-bag of hostile frames and
// still serve a legitimate client afterward, on every transport.
func TestMalformedFrames(t *testing.T) {
	for i, p := range allProbes() {
		p := p
		port := 18820 + i
		t.Run(p.name, func(t *testing.T) {
			server := rpc.NewServer(rpc.ServerConfig{
				Transport:  p.serverTransport(port),
				ErrHandler: func(err error) {},
			})
			pingpong.RegisterPingPongServer(server, &pingpongServer{})
			pingpong.RegisterChatServer(server, &chat.ChatServer{})
			go func() { server.ListenAndServe() }()
			defer server.Shutdown(context.Background())
			time.Sleep(150 * time.Millisecond)

			raw := p.dialRaw(t, "127.0.0.1", port)
			malformed := [][]byte{
				[]byte("this is not a valid scg frame at all!!"),
				{},
				func() []byte {
					w := serialize.NewWriter(16)
					rpc.SerializePrefix(w, rpc.StreamPrefix)
					return w.Bytes()
				}(),
				// A valid unary request prefix followed by garbage — exercises the
				// unary request path's robustness, not just the stream path.
				func() []byte {
					w := serialize.NewWriter(32)
					rpc.SerializePrefix(w, rpc.RequestPrefix)
					return append(w.Bytes(), []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x00, 0x11, 0x22}...)
				}(),
				streamFrame(7, 0xFF, nil),
				streamFrame(123, rpc.StreamFrameMessage, []byte{0x01, 0x02, 0x03}),
				streamFrame(124, rpc.StreamFrameHalfClose, nil),
				streamFrame(125, rpc.StreamFrameClose, []byte{0x00}),
				streamFrame(200, rpc.StreamFrameOpen, []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x00, 0x11}),
				// Oversized declared context length on an OPEN: must be rejected,
				// not allocated. (count=1 entry, key "", value length ~1 GiB.)
				oversizedOpenFrame(),
				streamFrame(0, rpc.StreamFramePing, nil),
				streamFrame(0, rpc.StreamFramePong, nil),
			}
			for _, m := range malformed {
				raw.sendFrame(t, m)
			}
			raw.close()
			time.Sleep(100 * time.Millisecond)

			// The server must still be alive and serving.
			client := rpc.NewClient(rpc.ClientConfig{Transport: p.clientTransport("127.0.0.1", port)})
			defer client.Close()

			pp := pingpong.NewPingPongClient(client)
			resp, err := pp.Ping(context.Background(), &pingpong.PingRequest{Ping: pingpong.Ping{Count: 41}})
			require.NoError(t, err)
			require.Equal(t, int32(42), resp.Pong.Count)

			stream, err := pingpong.NewChatClient(client).Connect(context.Background())
			require.NoError(t, err)
			welcome, err := stream.Recv()
			require.NoError(t, err)
			require.Equal(t, "welcome", welcome.Text)
		})
	}
}

// oversizedOpenFrame builds an OPEN whose context declares a ~1 GiB metadata
// value but supplies none — exercising the pre-auth allocation guard over the
// wire (server must reject, not allocate).
func oversizedOpenFrame() []byte {
	tail := serialize.NewWriter(32)
	serialize.SerializeUInt32(tail, 1)     // context entry count
	serialize.SerializeString(tail, "k")   // key
	serialize.SerializeUInt32(tail, 1<<30) // value byte length (hostile)
	return streamFrame(200, rpc.StreamFrameOpen, tail.Bytes())
}
