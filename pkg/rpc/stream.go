package rpc

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/kbirk/scg/pkg/serialize"
)

// defaultStreamRecvBufferSize bounds the per-stream inbound queue when no size
// is configured. A consumer that cannot keep up cannot grow memory without
// bound: when the buffer overflows the offending stream is terminated with an
// error and the peer is notified, while other streams and the connection read
// loop are never blocked. This is the v1 backpressure policy (windowed flow
// control is a non-goal). Configurable via ClientConfig/ServerConfig.
const defaultStreamRecvBufferSize = 1024

// ErrStreamClosed is returned by Send once the stream has terminated.
var ErrStreamClosed = errors.New("stream closed")

// errStreamOverflow terminates a stream whose bounded receive buffer overflowed.
var errStreamOverflow = errors.New("stream receive buffer overflow")

// streamRecvBufferSizeOrDefault normalizes a configured buffer size.
func streamRecvBufferSizeOrDefault(size int) int {
	if size <= 0 {
		return defaultStreamRecvBufferSize
	}
	return size
}

// recvQueue is a stream's bounded inbound message queue. It grows lazily (a
// stream that exchanges a handful of messages never allocates the full bound)
// while keeping the overflow semantics of a channel with capacity `limit`:
// deliver fails once `limit` messages are buffered. Buffered messages are always
// returned before the terminal status.
type recvQueue struct {
	mu     sync.Mutex
	items  []*serialize.Reader
	head   int
	limit  int
	closed bool          // terminal status set (no further deliveries)
	err    error         // terminal status (io.EOF or an error)
	notify chan struct{} // capacity 1: "items may be available"
	done   chan struct{} // closed when the terminal status is set
}

func newRecvQueue(limit int) recvQueue {
	return recvQueue{
		limit:  streamRecvBufferSizeOrDefault(limit),
		notify: make(chan struct{}, 1),
		done:   make(chan struct{}),
	}
}

func (q *recvQueue) signal() {
	select {
	case q.notify <- struct{}{}:
	default:
	}
}

// push enqueues r. It returns false if the queue is closed (the message is
// dropped) and overflow=true if the bound is reached.
func (q *recvQueue) push(r *serialize.Reader) (accepted bool, overflow bool) {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return false, false
	}
	if len(q.items)-q.head >= q.limit {
		q.mu.Unlock()
		return false, true
	}
	if q.head > 0 && q.head >= len(q.items)/2 && len(q.items) == cap(q.items) {
		// Compact before growing so a consumer that trails the producer does
		// not grow the backing array without bound.
		n := copy(q.items, q.items[q.head:])
		clear(q.items[n:])
		q.items = q.items[:n]
		q.head = 0
	}
	q.items = append(q.items, r)
	q.mu.Unlock()
	q.signal()
	return true, false
}

// pop returns the next buffered message, or the terminal status once the queue
// is closed and drained. ok=false means nothing is available yet.
func (q *recvQueue) pop() (r *serialize.Reader, err error, ok bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.head < len(q.items) {
		r = q.items[q.head]
		q.items[q.head] = nil
		q.head++
		if q.head == len(q.items) {
			q.items = q.items[:0]
			q.head = 0
		} else {
			// More remain: keep the wake-up token for other consumers.
			q.signal()
		}
		return r, nil, true
	}
	if q.closed {
		return nil, q.err, true
	}
	return nil, nil, false
}

// close sets the terminal status if not already set. Returns true if it did.
func (q *recvQueue) close(err error) bool {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return false
	}
	q.closed = true
	q.err = err
	close(q.done)
	q.mu.Unlock()
	return true
}

// emptyStreamMessage is a zero-size sentinel passed through the middleware chain
// when a stream is opened, so that message-oriented middleware (e.g. auth) can
// gate the stream from the OPEN context metadata.
type emptyStreamMessage struct{}

func (e *emptyStreamMessage) BitSize() int                        { return 0 }
func (e *emptyStreamMessage) ToJSON() ([]byte, error)             { return []byte("{}"), nil }
func (e *emptyStreamMessage) FromJSON([]byte) error               { return nil }
func (e *emptyStreamMessage) ToBytes() []byte                     { return []byte{} }
func (e *emptyStreamMessage) FromBytes([]byte) error              { return nil }
func (e *emptyStreamMessage) Serialize(*serialize.Writer)         {}
func (e *emptyStreamMessage) Deserialize(*serialize.Reader) error { return nil }

// streamServerStub is implemented by generated service stubs that contain at
// least one streaming method. The server discovers it via type assertion, so
// unary-only generated code is unaffected. Middleware/auth is gated by the
// server on OPEN before this is invoked, so the stub only dispatches by method.
type streamServerStub interface {
	HandleStreamWrapper(ctx context.Context, stream *ServerStream, methodID uint64) error
}

// connStreams is the per-connection registry of live server streams. The
// connection read loop and each handler goroutine touch it, so it is guarded.
type connStreams struct {
	mu      sync.Mutex
	streams map[uint64]*ServerStream
}

func newConnStreams() *connStreams {
	return &connStreams{streams: make(map[uint64]*ServerStream)}
}

func (c *connStreams) add(id uint64, s *ServerStream) {
	c.mu.Lock()
	c.streams[id] = s
	c.mu.Unlock()
}

func (c *connStreams) get(id uint64) *ServerStream {
	c.mu.Lock()
	s := c.streams[id]
	c.mu.Unlock()
	return s
}

func (c *connStreams) remove(id uint64) {
	c.mu.Lock()
	delete(c.streams, id)
	c.mu.Unlock()
}

func (c *connStreams) count() int {
	c.mu.Lock()
	n := len(c.streams)
	c.mu.Unlock()
	return n
}

func (c *connStreams) terminateAll(err error) {
	c.mu.Lock()
	streams := c.streams
	c.streams = make(map[uint64]*ServerStream)
	c.mu.Unlock()

	for _, s := range streams {
		s.die(err)
	}
}

// ----------------------------------------------------------------------------
// Frame serialization
// ----------------------------------------------------------------------------

func serializeStreamOpen(ctx context.Context, streamID uint64, serviceID uint64, methodID uint64) []byte {
	size := serialize.BitsToBytes(
		BitSizePrefix() +
			serialize.BitSizeUInt64(streamID) +
			serialize.BitSizeUInt8(StreamFrameOpen) +
			BitSizeContext(ctx) +
			serialize.BitSizeUInt64(serviceID) +
			serialize.BitSizeUInt64(methodID))

	writer := serialize.NewWriter(size)
	SerializePrefix(writer, StreamPrefix)
	serialize.SerializeUInt64(writer, streamID)
	serialize.SerializeUInt8(writer, StreamFrameOpen)
	SerializeContext(writer, ctx)
	serialize.SerializeUInt64(writer, serviceID)
	serialize.SerializeUInt64(writer, methodID)
	return writer.Bytes()
}

// sendStreamMessage serializes a MSG frame into a pooled writer and sends it on
// conn. This is the hot path, so it avoids a per-message heap allocation by
// borrowing from the writer pool (conn.Send consumes the bytes synchronously
// before the writer is returned).
func sendStreamMessage(conn Connection, serviceID uint64, streamID uint64, msg Message) error {
	size := serialize.BitsToBytes(
		BitSizePrefix() +
			serialize.BitSizeUInt64(streamID) +
			serialize.BitSizeUInt8(StreamFrameMessage) +
			msg.BitSize())

	writer := getWriter(size)
	defer putWriter(writer)

	SerializePrefix(writer, StreamPrefix)
	serialize.SerializeUInt64(writer, streamID)
	serialize.SerializeUInt8(writer, StreamFrameMessage)
	msg.Serialize(writer)
	return conn.Send(writer.Bytes(), serviceID)
}

func serializeStreamHalfClose(streamID uint64) []byte {
	size := serialize.BitsToBytes(
		BitSizePrefix() +
			serialize.BitSizeUInt64(streamID) +
			serialize.BitSizeUInt8(StreamFrameHalfClose))

	writer := serialize.NewWriter(size)
	SerializePrefix(writer, StreamPrefix)
	serialize.SerializeUInt64(writer, streamID)
	serialize.SerializeUInt8(writer, StreamFrameHalfClose)
	return writer.Bytes()
}

// serializeStreamControl builds a connection-level keepalive frame (PING/PONG).
// The stream id is unused (0).
func serializeStreamControl(frameKind uint8) []byte {
	size := serialize.BitsToBytes(
		BitSizePrefix() +
			serialize.BitSizeUInt64(0) +
			serialize.BitSizeUInt8(frameKind))

	writer := serialize.NewWriter(size)
	SerializePrefix(writer, StreamPrefix)
	serialize.SerializeUInt64(writer, 0)
	serialize.SerializeUInt8(writer, frameKind)
	return writer.Bytes()
}

func serializeStreamClose(streamID uint64, status uint8, message string) []byte {
	size := serialize.BitsToBytes(
		BitSizePrefix() +
			serialize.BitSizeUInt64(streamID) +
			serialize.BitSizeUInt8(StreamFrameClose) +
			serialize.BitSizeUInt8(status) +
			serialize.BitSizeString(message))

	writer := serialize.NewWriter(size)
	SerializePrefix(writer, StreamPrefix)
	serialize.SerializeUInt64(writer, streamID)
	serialize.SerializeUInt8(writer, StreamFrameClose)
	serialize.SerializeUInt8(writer, status)
	serialize.SerializeString(writer, message)
	return writer.Bytes()
}

// ----------------------------------------------------------------------------
// ClientStream — the client side of a bidirectional stream.
// ----------------------------------------------------------------------------

type ClientStream struct {
	client    *Client
	conn      Connection // the connection the stream was opened on
	streamID  uint64
	serviceID uint64
	ctx       context.Context

	recv recvQueue

	mu         sync.Mutex
	dead       bool // whole stream is dead (Send fails)
	sendClosed bool // local CloseSend has been issued
}

func newClientStream(client *Client, conn Connection, ctx context.Context, streamID uint64, serviceID uint64, bufferSize int) *ClientStream {
	return &ClientStream{
		client:    client,
		conn:      conn,
		streamID:  streamID,
		serviceID: serviceID,
		ctx:       ctx,
		recv:      newRecvQueue(bufferSize),
	}
}

// Context returns the context the stream was opened with.
func (s *ClientStream) Context() context.Context {
	return s.ctx
}

// Send writes a message to the server. It is safe to call from any goroutine and
// does not block on the peer. It returns an error once the stream is dead or
// after CloseSend. A server half-close does not stop the client from sending.
func (s *ClientStream) Send(msg Message) error {
	s.mu.Lock()
	if s.dead {
		s.mu.Unlock()
		return ErrStreamClosed
	}
	if s.sendClosed {
		s.mu.Unlock()
		return errors.New("stream send is already closed")
	}
	s.mu.Unlock()

	return sendStreamMessage(s.conn, s.serviceID, s.streamID, msg)
}

// Recv blocks until the next message arrives, the stream is cleanly closed
// (io.EOF), or an error terminates it. Buffered messages are always delivered
// before the terminal status.
func (s *ClientStream) Recv() (*serialize.Reader, error) {
	for {
		if err := s.ctx.Err(); err != nil {
			// Caller cancelled; tear the stream down and notify the server.
			// Checked first so a fast producer cannot starve cancellation.
			s.cancel(err)
			return nil, err
		}
		if r, err, ok := s.recv.pop(); ok {
			return r, err
		}
		select {
		case <-s.recv.notify:
		case <-s.recv.done:
		case <-s.ctx.Done():
		}
	}
}

// CloseSend signals that the client is done sending. It may still receive.
func (s *ClientStream) CloseSend() error {
	s.mu.Lock()
	if s.sendClosed || s.dead {
		s.mu.Unlock()
		return nil
	}
	s.sendClosed = true
	s.mu.Unlock()

	return s.conn.Send(serializeStreamHalfClose(s.streamID), s.serviceID)
}

// deliver enqueues an inbound message; called by the client demux on the I/O
// goroutine in arrival order. It returns true if the bounded buffer overflowed,
// in which case the stream is now dead and the caller must notify the peer.
func (s *ClientStream) deliver(r *serialize.Reader) (overflowed bool) {
	if _, overflow := s.recv.push(r); overflow {
		s.die(errStreamOverflow)
		return true
	}
	return false
}

// closeRecv marks the recv direction terminal (server done sending). The client
// may still Send. Idempotent.
func (s *ClientStream) closeRecv(err error) {
	s.recv.close(err)
}

// die marks the whole stream dead: Send fails and a pending Recv unblocks with
// err (unless the recv direction already ended cleanly). Idempotent.
func (s *ClientStream) die(err error) {
	s.mu.Lock()
	s.dead = true
	s.mu.Unlock()
	s.recv.close(err)
}

// cancel kills the stream locally and best-effort notifies the server.
func (s *ClientStream) cancel(err error) {
	s.mu.Lock()
	already := s.dead
	s.mu.Unlock()

	s.die(err)
	s.client.removeStream(s.streamID)
	if !already {
		_ = s.conn.Send(serializeStreamClose(s.streamID, StreamStatusError, "stream cancelled by client"), s.serviceID)
	}
}

// ----------------------------------------------------------------------------
// ServerStream — the server side of a bidirectional stream.
// ----------------------------------------------------------------------------

type ServerStream struct {
	conn      Connection
	streamID  uint64
	serviceID uint64
	ctx       context.Context
	cancel    context.CancelCauseFunc

	recv recvQueue

	mu   sync.Mutex
	dead bool // whole stream is dead (Send fails)
}

func newServerStream(conn Connection, ctx context.Context, streamID uint64, serviceID uint64, bufferSize int) *ServerStream {
	// Wrap the OPEN context so the stream's death (client cancel / connection
	// drop) cancels it. A push-only handler — one that only Sends and never
	// Recvs (e.g. a chat presence pump fed by a broadcast channel) — would
	// otherwise not observe the client going away until its next Send; it can now
	// select on Context().Done() instead. Metadata on the OPEN context (the
	// authenticated identity) is preserved.
	ctx, cancel := context.WithCancelCause(ctx)
	return &ServerStream{
		conn:      conn,
		streamID:  streamID,
		serviceID: serviceID,
		ctx:       ctx,
		cancel:    cancel,
		recv:      newRecvQueue(bufferSize),
	}
}

// Context returns a context that carries the OPEN metadata (e.g. the
// authenticated identity) and is cancelled when the stream dies — so a handler
// can watch Context().Done() to react to client cancellation or connection loss.
func (s *ServerStream) Context() context.Context {
	return s.ctx
}

// Recv blocks for the next client message. It returns io.EOF when the client
// half-closes, or an error if the client cancelled / the connection dropped.
func (s *ServerStream) Recv() (*serialize.Reader, error) {
	for {
		if r, err, ok := s.recv.pop(); ok {
			return r, err
		}
		select {
		case <-s.recv.notify:
		case <-s.recv.done:
		}
	}
}

// Send pushes a message to the client. It remains valid after the client
// half-closes (Recv returns io.EOF); it fails only once the stream is dead.
func (s *ServerStream) Send(msg Message) error {
	s.mu.Lock()
	if s.dead {
		s.mu.Unlock()
		return ErrStreamClosed
	}
	s.mu.Unlock()

	return sendStreamMessage(s.conn, s.serviceID, s.streamID, msg)
}

// deliver enqueues an inbound message. Returns true if the bounded buffer
// overflowed, in which case the stream is now dead and the caller must notify
// the peer.
func (s *ServerStream) deliver(r *serialize.Reader) (overflowed bool) {
	if _, overflow := s.recv.push(r); overflow {
		s.die(errStreamOverflow)
		return true
	}
	return false
}

// closeRecv marks the recv direction terminal. The handler may still Send.
func (s *ServerStream) closeRecv(err error) {
	s.recv.close(err)
}

// die marks the whole stream dead (connection dropped / client cancelled) and
// cancels the handler's context with the cause.
func (s *ServerStream) die(err error) {
	s.mu.Lock()
	s.dead = true
	s.mu.Unlock()
	s.recv.close(err)
	s.cancel(err)
}

// markDone marks the stream dead once its handler has returned so a goroutine
// the handler leaked cannot keep sending frames after the terminal CLOSE.
func (s *ServerStream) markDone() {
	s.mu.Lock()
	s.dead = true
	s.mu.Unlock()
}

// halfClose is called when the client signals it is done sending; the handler's
// Recv returns io.EOF but it may still Send responses.
func (s *ServerStream) halfClose() {
	s.closeRecv(io.EOF)
}
