package rpc

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kbirk/scg/pkg/serialize"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This reproduces the buffered-response-channel half of commit 469bb52 ("More
// fixes"): the per-request response channel must be buffered (size 1) so a late
// delivery from the receive goroutine does not block forever when the caller has
// already bailed out on ctx cancellation. A blocked send wedges the single
// receive goroutine and kills the whole connection.
//
// It lives here (rather than in the integration harness) because reliably
// hitting the delivery-vs-cancellation race needs the precise timing control of
// an in-memory Connection; over a real transport it could only be reproduced
// probabilistically. The companion Close()-notification edge case is covered by
// TestCloseNotifiesPendingRequestOnCleanServerClose in the test/go harness.

// edgeMockConn is a hand-controlled Connection. Send captures outgoing frames on
// `sent`; the test feeds responses in on `recv`. Receive returns the
// "connection closed" error on Close, exactly like the real transports, which is
// what makes the receive goroutine treat it as a normal exit.
type edgeMockConn struct {
	sent   chan []byte
	recv   chan []byte
	closed chan struct{}
	once   sync.Once
}

func newEdgeMockConn() *edgeMockConn {
	return &edgeMockConn{
		sent:   make(chan []byte, 4096),
		recv:   make(chan []byte, 4096),
		closed: make(chan struct{}),
	}
}

func (m *edgeMockConn) Send(data []byte, serviceID uint64) error {
	b := make([]byte, len(data))
	copy(b, data)
	select {
	case m.sent <- b:
		return nil
	case <-m.closed:
		return errors.New("connection closed")
	}
}

func (m *edgeMockConn) Receive() ([]byte, error) {
	select {
	case b := <-m.recv:
		return b, nil
	case <-m.closed:
		return nil, errors.New("connection closed")
	}
}

func (m *edgeMockConn) Close() error {
	m.once.Do(func() { close(m.closed) })
	return nil
}

type edgeMockTransport struct{ conn *edgeMockConn }

func (t *edgeMockTransport) Connect() (Connection, error) { return t.conn, nil }

// edgeMsg is a no-op Message; the tests only care about the request/response
// plumbing, not payload contents.
type edgeMsg struct{}

func (edgeMsg) BitSize() int                       { return 0 }
func (edgeMsg) ToJSON() ([]byte, error)            { return nil, nil }
func (edgeMsg) FromJSON([]byte) error              { return nil }
func (edgeMsg) ToBytes() []byte                    { return nil }
func (edgeMsg) FromBytes([]byte) error             { return nil }
func (edgeMsg) Serialize(*serialize.Writer)        {}
func (edgeMsg) Deserialize(*serialize.Reader) error { return nil }

// parseRequestID extracts the request ID from a captured outgoing request frame:
// prefix(16) + context + requestID + serviceID + methodID + msg.
func parseRequestID(t *testing.T, frame []byte) uint64 {
	t.Helper()
	reader := serialize.NewReader(frame)
	var prefix [16]byte
	require.NoError(t, DeserializePrefix(&prefix, reader))
	var cctx context.Context
	require.NoError(t, DeserializeContext(&cctx, reader))
	var requestID uint64
	require.NoError(t, serialize.DeserializeUInt64(&requestID, reader))
	return requestID
}

// buildResponse crafts a successful response frame for the given request ID.
func buildResponse(requestID uint64) []byte {
	w := serialize.NewWriter(64)
	SerializePrefix(w, ResponsePrefix)
	serialize.SerializeUInt64(w, requestID)
	serialize.SerializeUInt8(w, MessageResponse)
	return w.Bytes()
}

// TestLateResponseDoesNotWedgeReceiveLoop reproduces the buffered
// response channel. Each iteration races a response delivery against ctx
// cancellation: if a delivery lands in the window after the caller has left,
// an UNBUFFERED channel blocks the single receive goroutine forever. The final
// clean Call then never gets its response. With the buffered channel the late
// send is absorbed and the client keeps working.
//
// The race is timing-dependent, so we run many iterations to hit the window.
func TestLateResponseDoesNotWedgeReceiveLoop(t *testing.T) {
	conn := newEdgeMockConn()
	client := NewClient(ClientConfig{Transport: &edgeMockTransport{conn: conn}})
	defer client.Close()

	const iterations = 1500
	for i := 0; i < iterations; i++ {
		ctx, cancel := context.WithCancel(context.Background())

		callDone := make(chan struct{})
		go func() {
			_, _ = client.Call(ctx, 1, 2, edgeMsg{})
			close(callDone)
		}()

		frame := <-conn.sent
		resp := buildResponse(parseRequestID(t, frame))

		// Deliver the response and cancel the context as close to simultaneously
		// as possible, so delivery races the caller's exit.
		go func() { conn.recv <- resp }()
		cancel()

		<-callDone
	}

	// If the receive goroutine got wedged by a late send above, it can no longer
	// process responses, so this clean call will never complete.
	type result struct {
		err error
	}
	res := make(chan result, 1)
	go func() {
		_, err := client.Call(context.Background(), 1, 2, edgeMsg{})
		res <- result{err}
	}()

	frame := <-conn.sent
	conn.recv <- buildResponse(parseRequestID(t, frame))

	select {
	case r := <-res:
		assert.NoError(t, r.err, "final call should succeed")
	case <-time.After(3 * time.Second):
		t.Fatal("receive goroutine is wedged: a late send on an unbuffered request " +
			"channel blocked it, so the client can no longer process responses")
	}
}
