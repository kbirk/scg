package test

import (
	"context"
	"io"
	"runtime"
	"testing"
	"time"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/rpc/tcp"
	"github.com/kbirk/scg/test/go/streamimpl"
	"github.com/kbirk/scg/test/scg/generated/pingpong"

	"github.com/stretchr/testify/require"
)

// TestStreamHandlerNoLeak opens and closes many streams and asserts the
// goroutine count returns to its baseline — i.e. every per-stream server
// handler goroutine exits on a clean close (no leak under churn).
func TestStreamHandlerNoLeak(t *testing.T) {
	const port = 18770

	server := rpc.NewServer(rpc.ServerConfig{
		Transport:  tcp.NewServerTransport(tcp.ServerTransportConfig{Port: port, NoDelay: true}),
		ErrHandler: func(err error) {},
	})
	pingpong.RegisterPingPongServer(server, &pingpongServer{})
	pingpong.RegisterChatServer(server, &streamimpl.ChatServer{})
	go func() { server.ListenAndServe() }()
	defer server.Shutdown(context.Background())
	time.Sleep(150 * time.Millisecond)

	client := rpc.NewClient(rpc.ClientConfig{
		Transport: tcp.NewClientTransport(tcp.ClientTransportConfig{Host: "127.0.0.1", Port: port, NoDelay: true}),
	})
	defer client.Close()

	chat := pingpong.NewChatClient(client)

	oneStream := func() {
		stream, err := chat.Connect(context.Background())
		require.NoError(t, err)
		_, err = stream.Recv() // welcome
		require.NoError(t, err)
		require.NoError(t, stream.Send(&pingpong.ChatMessage{Text: "x", Seq: 1}))
		_, err = stream.Recv() // echo
		require.NoError(t, err)
		require.NoError(t, stream.CloseSend())
		_, err = stream.Recv() // summary
		require.NoError(t, err)
		_, err = stream.Recv() // io.EOF
		require.Equal(t, io.EOF, err)
	}

	// Warm up (establish the connection + its read loop), then record baseline.
	oneStream()
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	const n = 300
	for i := 0; i < n; i++ {
		oneStream()
	}

	runtime.GC()
	time.Sleep(300 * time.Millisecond)
	final := runtime.NumGoroutine()

	t.Logf("goroutines: baseline=%d final=%d after %d streams", baseline, final, n)
	// A per-stream leak would add ~n goroutines; allow a small slack for transients.
	require.LessOrEqual(t, final, baseline+10, "stream handler goroutines leaked")
}
