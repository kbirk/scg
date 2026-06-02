package test

import (
	"context"
	"encoding/binary"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/rpc/tcp"
	"github.com/kbirk/scg/pkg/serialize"
	"github.com/kbirk/scg/test/go/chat"
	"github.com/kbirk/scg/test/scg/generated/pingpong"

	"github.com/stretchr/testify/require"
)

// writeTCPFrame writes a single length-delimited frame (matching the TCP
// transport's framing) onto a raw connection.
func writeTCPFrame(t *testing.T, conn net.Conn, payload []byte) {
	t.Helper()
	hdr := make([]byte, 4)
	binary.BigEndian.PutUint32(hdr, uint32(len(payload)))
	_, err := conn.Write(hdr)
	require.NoError(t, err)
	_, err = conn.Write(payload)
	require.NoError(t, err)
}

func streamPrefixBytes(streamID uint64, frameKind uint8, tail []byte) []byte {
	w := serialize.NewWriter(64)
	rpc.SerializePrefix(w, rpc.StreamPrefix)
	serialize.SerializeUInt64(w, streamID)
	serialize.SerializeUInt8(w, frameKind)
	out := w.Bytes()
	return append(out, tail...)
}

// TestServerSurvivesMalformedFrames feeds a server a series of malformed /
// hostile frames over a raw connection and verifies the server neither crashes
// nor wedges: a legitimate client can still connect and make calls afterward.
func TestServerSurvivesMalformedFrames(t *testing.T) {
	const port = 18750

	server := rpc.NewServer(rpc.ServerConfig{
		Transport:  tcp.NewServerTransport(tcp.ServerTransportConfig{Port: port}),
		ErrHandler: func(err error) { /* malformed input is expected to log errors */ },
	})
	pingpong.RegisterPingPongServer(server, &pingpongServer{})
	pingpong.RegisterChatServer(server, &chat.ChatServer{})
	go func() { server.ListenAndServe() }()
	defer server.Shutdown(context.Background())
	time.Sleep(150 * time.Millisecond)

	raw, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(port))
	require.NoError(t, err)

	// A grab-bag of malformed frames. None should crash, deadlock, or leak.
	malformed := [][]byte{
		// Junk with no recognizable prefix.
		[]byte("this is not a valid scg frame at all!!"),
		// Empty frame.
		{},
		// Valid stream prefix but truncated (no stream id / kind).
		func() []byte { w := serialize.NewWriter(16); rpc.SerializePrefix(w, rpc.StreamPrefix); return w.Bytes() }(),
		// Unknown frame kind.
		streamPrefixBytes(7, 0xFF, nil),
		// MSG / HALF_CLOSE / CLOSE for a stream that was never opened.
		streamPrefixBytes(123, rpc.StreamFrameMessage, []byte{0x01, 0x02, 0x03}),
		streamPrefixBytes(124, rpc.StreamFrameHalfClose, nil),
		streamPrefixBytes(125, rpc.StreamFrameClose, []byte{0x00}),
		// OPEN with garbage context / serviceID / methodID tail.
		streamPrefixBytes(200, rpc.StreamFrameOpen, []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x00, 0x11}),
		// PING / PONG (connection-level) — server must tolerate.
		streamPrefixBytes(0, rpc.StreamFramePing, nil),
		streamPrefixBytes(0, rpc.StreamFramePong, nil),
		// A request prefix followed by garbage.
		func() []byte { w := serialize.NewWriter(16); rpc.SerializePrefix(w, rpc.RequestPrefix); return append(w.Bytes(), 0xFF, 0xFF, 0xFF) }(),
	}

	for _, m := range malformed {
		writeTCPFrame(t, raw, m)
	}
	raw.Close()

	// Give the server a moment to process the garbage.
	time.Sleep(100 * time.Millisecond)

	// The server must still be alive and serving.
	client := rpc.NewClient(rpc.ClientConfig{
		Transport: tcp.NewClientTransport(tcp.ClientTransportConfig{Host: "127.0.0.1", Port: port}),
	})
	defer client.Close()

	pp := pingpong.NewPingPongClient(client)
	resp, err := pp.Ping(context.Background(), &pingpong.PingRequest{Ping: pingpong.Ping{Count: 41}})
	require.NoError(t, err)
	require.Equal(t, int32(42), resp.Pong.Count)

	// And a streaming call still works after the abuse.
	chat := pingpong.NewChatClient(client)
	stream, err := chat.Connect(context.Background())
	require.NoError(t, err)
	welcome, err := stream.Recv()
	require.NoError(t, err)
	require.Equal(t, "welcome", welcome.Text)
}
