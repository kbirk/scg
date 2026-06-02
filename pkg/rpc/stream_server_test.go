package rpc

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/kbirk/scg/pkg/serialize"
	"github.com/stretchr/testify/require"
)

// recordingConn is a fake Connection that records the frames written to it, for
// asserting the server's control responses without a real transport.
type recordingConn struct {
	mu     sync.Mutex
	sent   [][]byte
	closed bool
}

func (c *recordingConn) Send(data []byte, serviceID uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	b := make([]byte, len(data))
	copy(b, data)
	c.sent = append(c.sent, b)
	return nil
}

func (c *recordingConn) Receive() ([]byte, error) { return nil, errors.New("connection closed") }

func (c *recordingConn) Close() error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	return nil
}

// closeMessages deserializes every recorded CLOSE frame and returns their status
// messages. Frames are bit-packed, so the payload must be parsed rather than
// byte-searched.
func (c *recordingConn) closeMessages() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var msgs []string
	for _, f := range c.sent {
		r := serialize.NewReader(f)
		var prefix [16]byte
		if DeserializePrefix(&prefix, r) != nil || prefix != StreamPrefix {
			continue
		}
		var streamID uint64
		if serialize.DeserializeUInt64(&streamID, r) != nil {
			continue
		}
		var kind uint8
		if serialize.DeserializeUInt8(&kind, r) != nil || kind != StreamFrameClose {
			continue
		}
		var status uint8
		if serialize.DeserializeUInt8(&status, r) != nil {
			continue
		}
		var msg string
		if serialize.DeserializeString(&msg, r) != nil {
			continue
		}
		msgs = append(msgs, msg)
	}
	return msgs
}

func (c *recordingConn) sentCloseWith(needle string) bool {
	for _, m := range c.closeMessages() {
		if strings.Contains(m, needle) {
			return true
		}
	}
	return false
}

// blockingStreamService is a minimal streaming stub whose handler blocks, so an
// opened stream stays registered for the duration of the test.
type blockingStreamService struct {
	block chan struct{}
}

func (s *blockingStreamService) HandleWrapper(context.Context, []Middleware, uint64, *serialize.Reader) []byte {
	return nil
}

func (s *blockingStreamService) HandleStreamWrapper(ctx context.Context, stream *ServerStream, methodID uint64) error {
	<-s.block
	return nil
}

// openFrameReader builds an OPEN frame and returns a reader positioned just past
// the prefix, matching how handleConnection hands frames to handleStreamFrame.
func openFrameReader(t *testing.T, streamID, serviceID, methodID uint64) *serialize.Reader {
	t.Helper()
	bs := serializeStreamOpen(context.Background(), streamID, serviceID, methodID)
	r := serialize.NewReader(bs)
	var prefix [16]byte
	require.NoError(t, DeserializePrefix(&prefix, r))
	return r
}

// TestServerRejectsDuplicateStreamID verifies a second OPEN reusing a live
// stream id is rejected with a CLOSE(error) rather than orphaning the existing
// stream.
func TestServerRejectsDuplicateStreamID(t *testing.T) {
	const serviceID, methodID = uint64(42), uint64(7)

	svc := &blockingStreamService{block: make(chan struct{})}
	defer close(svc.block)

	server := NewServer(ServerConfig{})
	server.RegisterServer(serviceID, "fake", svc)

	conn := &recordingConn{}
	cs := newConnStreams()
	defer cs.terminateAll(errors.New("test done"))

	// First OPEN registers the stream (its handler blocks, keeping it live).
	server.handleStreamFrame(conn, cs, openFrameReader(t, 1, serviceID, methodID))
	require.NotNil(t, cs.get(1), "first OPEN should register the stream")

	// Second OPEN reusing id 1 must be rejected and must not displace the first.
	server.handleStreamFrame(conn, cs, openFrameReader(t, 1, serviceID, methodID))
	require.NotNil(t, cs.get(1), "duplicate OPEN must not orphan the existing stream")
	require.True(t, conn.sentCloseWith("duplicate stream id"),
		"server should reject a duplicate stream id with a CLOSE(error)")
}

// TestServerEnforcesMaxConcurrentStreams verifies the per-connection cap rejects
// an OPEN beyond the limit with a CLOSE(error).
func TestServerEnforcesMaxConcurrentStreams(t *testing.T) {
	const serviceID, methodID = uint64(42), uint64(7)

	svc := &blockingStreamService{block: make(chan struct{})}
	defer close(svc.block)

	server := NewServer(ServerConfig{MaxConcurrentStreams: 1})
	server.RegisterServer(serviceID, "fake", svc)

	conn := &recordingConn{}
	cs := newConnStreams()
	defer cs.terminateAll(errors.New("test done"))

	server.handleStreamFrame(conn, cs, openFrameReader(t, 1, serviceID, methodID))
	require.NotNil(t, cs.get(1))

	// Second distinct stream exceeds the cap of 1.
	server.handleStreamFrame(conn, cs, openFrameReader(t, 2, serviceID, methodID))
	require.Nil(t, cs.get(2), "stream beyond the cap must not be registered")
	require.True(t, conn.sentCloseWith("max concurrent streams exceeded"),
		"server should reject an over-cap stream with a CLOSE(error)")
}
