package test

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/rpc/tcp"
	"github.com/kbirk/scg/pkg/serialize"
	"github.com/kbirk/scg/test/go/streamimpl"
	"github.com/kbirk/scg/test/scg/generated/pingpong"

	"github.com/stretchr/testify/require"
)

// Service/method IDs for the generated Chat service. These are stable name
// hashes (see the generated pingpong.go). The overrun test below forges raw
// frames with them; if they drift, the server rejects the OPEN, no stream is
// created, no overflow occurs, and the test fails loudly — so they are
// self-checking rather than silently wrong.
const (
	rawChatServiceID     uint64 = 2103619306134415467
	rawChatConnectMethod uint64 = 8813269157622748953
)

// fcChatServer is a test-local Chat implementation with controllable
// consumption, used to drive deterministic flow-control scenarios that the
// shared (always-consuming) streamimpl.ChatServer cannot.
type fcChatServer struct {
	// when set, Connect sends "welcome" and then blocks on release WITHOUT ever
	// consuming, so the server's receive window fills and stays full.
	noConsume bool
	release   chan struct{}
}

func (s *fcChatServer) Connect(stream *pingpong.Chat_ConnectStreamServer) error {
	if err := stream.Send(&pingpong.ChatMessage{Text: "welcome", Seq: 0}); err != nil {
		return err
	}
	if s.noConsume {
		<-s.release // hold the stream open without draining its receive buffer
		return nil
	}
	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}
		if err := stream.Send(&pingpong.ChatMessage{Text: "echo:" + msg.Text, Seq: msg.Seq + 1}); err != nil {
			return err
		}
	}
}

func (s *fcChatServer) Subscribe(req *pingpong.SubscribeRequest, stream *pingpong.Chat_SubscribeStreamServer) error {
	return nil
}

func (s *fcChatServer) Upload(stream *pingpong.Chat_UploadStreamServer) (*pingpong.UploadSummary, error) {
	return &pingpong.UploadSummary{}, nil
}

// TestStreamFlowControlBackpressure pushes many messages through a deliberately
// tiny per-stream window using the BLOCKING Send. The window only admits a few
// messages at a time, so every message relies on the server replenishing credit
// (WINDOW_UPDATE) as its handler consumes. A correct credit loop delivers all
// messages losslessly; a broken one either deadlocks (caught by the stream's
// context deadline) or drops/corrupts messages (wrong sum).
func TestStreamFlowControlBackpressure(t *testing.T) {
	const port = 18780

	server := rpc.NewServer(rpc.ServerConfig{
		Transport:           tcp.NewServerTransport(tcp.ServerTransportConfig{Port: port, NoDelay: true}),
		ErrHandler:          func(err error) {},
		InitialStreamWindow: 200, // bytes — only a handful of small messages fit
	})
	pingpong.RegisterChatServer(server, &streamimpl.ChatServer{})
	go func() { server.ListenAndServe() }()
	defer server.Shutdown(context.Background())
	time.Sleep(150 * time.Millisecond)

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: tcp.NewClientTransport(tcp.ClientTransportConfig{Host: "127.0.0.1", Port: port, NoDelay: true}),
	})
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	chat := pingpong.NewChatClient(client)
	upload, err := chat.Upload(ctx)
	require.NoError(t, err)

	const n = 500
	var want int32
	for i := int32(1); i <= n; i++ {
		require.NoError(t, upload.Send(&pingpong.ChatMessage{Seq: i}), "Send blocked past the deadline — replenishment likely broken")
		want += i
	}
	summary, err := upload.CloseAndRecv()
	require.NoError(t, err)
	require.Equal(t, want, summary.Total, "messages lost or miscounted under flow control")
}

// TestStreamTrySendHappyPath exercises the non-blocking TrySend against a
// normally-consuming server: with credit available it sends immediately.
func TestStreamTrySendHappyPath(t *testing.T) {
	const port = 18781

	server := rpc.NewServer(rpc.ServerConfig{
		Transport:  tcp.NewServerTransport(tcp.ServerTransportConfig{Port: port, NoDelay: true}),
		ErrHandler: func(err error) {},
	})
	pingpong.RegisterChatServer(server, &streamimpl.ChatServer{})
	go func() { server.ListenAndServe() }()
	defer server.Shutdown(context.Background())
	time.Sleep(150 * time.Millisecond)

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: tcp.NewClientTransport(tcp.ClientTransportConfig{Host: "127.0.0.1", Port: port, NoDelay: true}),
	})
	defer client.Close()

	chat := pingpong.NewChatClient(client)
	stream, err := chat.Connect(context.Background())
	require.NoError(t, err)

	welcome, err := stream.Recv()
	require.NoError(t, err)
	require.Equal(t, "welcome", welcome.Text)

	// SETTINGS arrives on connect; TrySend should succeed once credit is known.
	var sent bool
	for i := 0; i < 100 && !sent; i++ {
		sent, err = stream.TrySend(&pingpong.ChatMessage{Text: "hi", Seq: 1})
		require.NoError(t, err)
		if !sent {
			time.Sleep(2 * time.Millisecond)
		}
	}
	require.True(t, sent, "TrySend never succeeded despite available credit")

	echo, err := stream.Recv()
	require.NoError(t, err)
	require.Equal(t, "echo:hi", echo.Text)

	require.NoError(t, stream.CloseSend())
}

// TestStreamTrySendOutOfCredit verifies TrySend reports backpressure rather than
// blocking: against a server that never consumes, the initial window is quickly
// exhausted and TrySend returns (false, nil) — without sending and without
// blocking — so a frame loop can hold the message and retry later.
func TestStreamTrySendOutOfCredit(t *testing.T) {
	const port = 18782

	impl := &fcChatServer{noConsume: true, release: make(chan struct{})}
	defer close(impl.release)

	server := rpc.NewServer(rpc.ServerConfig{
		Transport:           tcp.NewServerTransport(tcp.ServerTransportConfig{Port: port, NoDelay: true}),
		ErrHandler:          func(err error) {},
		InitialStreamWindow: 300, // small window; never replenished (server doesn't consume)
	})
	pingpong.RegisterChatServer(server, impl)
	go func() { server.ListenAndServe() }()
	defer server.Shutdown(context.Background())
	time.Sleep(150 * time.Millisecond)

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: tcp.NewClientTransport(tcp.ClientTransportConfig{Host: "127.0.0.1", Port: port, NoDelay: true}),
	})
	defer client.Close()

	chat := pingpong.NewChatClient(client)
	stream, err := chat.Connect(context.Background())
	require.NoError(t, err)
	welcome, err := stream.Recv()
	require.NoError(t, err)
	require.Equal(t, "welcome", welcome.Text)

	// Hammer TrySend. The first calls succeed (initial credit), then once the
	// window is exhausted (no replenishment) every call must report false.
	var trues, falses int
	for i := 0; i < 2000; i++ {
		ok, err := stream.TrySend(&pingpong.ChatMessage{Text: "x", Seq: int32(i)})
		require.NoError(t, err)
		if ok {
			trues++
		} else {
			falses++
		}
	}
	require.Greater(t, trues, 0, "no TrySend succeeded — initial credit not granted")
	require.Greater(t, falses, 0, "TrySend never reported backpressure — credit not enforced")
}

// rawStreamFrame builds a length-delimited stream frame body (prefix + streamID
// + frameKind + tail) for the raw-socket overrun test.
func rawStreamFrame(streamID uint64, frameKind uint8, tail []byte) []byte {
	w := serialize.NewWriter(64)
	rpc.SerializePrefix(w, rpc.StreamPrefix)
	serialize.SerializeUInt64(w, streamID)
	serialize.SerializeUInt8(w, frameKind)
	return append(w.Bytes(), tail...)
}

// TestStreamOverrunClosesConnection is the security test: a non-compliant client
// that sends past its granted credit must have its connection torn down by the
// server, and the server itself must survive (other clients keep working).
func TestStreamOverrunClosesConnection(t *testing.T) {
	const port = 18783

	impl := &fcChatServer{noConsume: true, release: make(chan struct{})}
	defer close(impl.release)

	server := rpc.NewServer(rpc.ServerConfig{
		Transport:           tcp.NewServerTransport(tcp.ServerTransportConfig{Port: port, NoDelay: true}),
		ErrHandler:          func(err error) {},
		InitialStreamWindow: 200, // small window; the server never drains it
	})
	pingpong.RegisterChatServer(server, impl)
	pingpong.RegisterPingPongServer(server, &pingpongServer{})
	go func() { server.ListenAndServe() }()
	defer server.Shutdown(context.Background())
	time.Sleep(150 * time.Millisecond)

	raw, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(port))
	require.NoError(t, err)

	// Open a real Chat.Connect stream over the raw socket.
	openTail := func() []byte {
		w := serialize.NewWriter(64)
		rpc.SerializeContext(w, context.Background())
		serialize.SerializeUInt64(w, rawChatServiceID)
		serialize.SerializeUInt64(w, rawChatConnectMethod)
		return w.Bytes()
	}()
	writeTCPFrame(t, raw, rawStreamFrame(1, rpc.StreamFrameOpen, openTail))

	// Build a single message and flood well past the 200-byte window. The server
	// never consumes (noConsume), so buffered bytes only grow → overrun. The
	// server tears the connection down mid-flood, so a write failure here is the
	// expected outcome (it confirms the close) — as is a subsequent read EOF.
	msg := &pingpong.ChatMessage{Text: "this-is-a-reasonably-sized-chat-message", Seq: 7}
	mw := serialize.NewWriter(64)
	msg.Serialize(mw)
	frame := rawStreamFrame(1, rpc.StreamFrameMessage, mw.Bytes())

	closed := false
	for i := 0; i < 200 && !closed; i++ {
		hdr := []byte{byte(len(frame) >> 24), byte(len(frame) >> 16), byte(len(frame) >> 8), byte(len(frame))}
		if _, werr := raw.Write(append(hdr, frame...)); werr != nil {
			closed = true // server reset the connection — the overrun was caught
		}
		time.Sleep(time.Millisecond)
	}
	if !closed {
		// Writes all buffered before the reset propagated; confirm via a read.
		_ = raw.SetReadDeadline(time.Now().Add(3 * time.Second))
		buf := make([]byte, 256)
		for {
			if _, rerr := raw.Read(buf); rerr != nil {
				closed = true // EOF / connection reset — server tore the connection down
				break
			}
		}
	}
	require.True(t, closed, "server did not close the connection after a flow-control overrun")
	raw.Close()

	// The server itself must still be serving other clients.
	client := rpc.NewClient(rpc.ClientConfig{
		Transport: tcp.NewClientTransport(tcp.ClientTransportConfig{Host: "127.0.0.1", Port: port, NoDelay: true}),
	})
	defer client.Close()
	pp := pingpong.NewPingPongClient(client)
	resp, err := pp.Ping(context.Background(), &pingpong.PingRequest{Ping: pingpong.Ping{Count: 41}})
	require.NoError(t, err)
	require.Equal(t, int32(42), resp.Pong.Count)
}
