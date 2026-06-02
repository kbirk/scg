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

// Flow control (credit-based, server-authoritative). The server dictates these
// byte windows via the SETTINGS frame on every accepted connection. The
// per-stream window also bounds the receive buffer: a sender that exceeds its
// granted credit overruns the window, which is a protocol violation.
const (
	defaultInitialStreamWindow     = 1 << 20 // 1 MiB per-stream window
	defaultInitialConnectionWindow = 4 << 20 // 4 MiB connection window (phase 3)
)

// initialStreamWindowOrDefault normalizes a configured per-stream window.
func initialStreamWindowOrDefault(n uint64) uint64 {
	if n == 0 {
		return defaultInitialStreamWindow
	}
	return n
}

// initialConnectionWindowOrDefault normalizes a configured connection window.
func initialConnectionWindowOrDefault(n uint64) uint64 {
	if n == 0 {
		return defaultInitialConnectionWindow
	}
	return n
}

// streamMessageCost returns the exact wire byte size of a MESSAGE frame for msg
// on the given stream — the unit of flow-control credit. The receiver derives
// the identical value from the received frame's length (serialize.Reader.Len),
// so both ends agree on cost without re-serializing.
func streamMessageCost(streamID uint64, msg Message) int {
	return serialize.BitsToBytes(
		BitSizePrefix() +
			serialize.BitSizeUInt64(streamID) +
			serialize.BitSizeUInt8(StreamFrameMessage) +
			msg.BitSize())
}

// streamItem is a buffered inbound message tagged with its wire cost so the
// receiver can replenish exactly that many bytes of credit when it is consumed.
type streamItem struct {
	reader *serialize.Reader
	cost   int
}

// streamRecvQueue is a byte-bounded, thread-safe inbound queue: a single
// producer (the connection read loop, via deliver) and a single consumer (the
// caller of recv). It bounds buffered *bytes* (not message count) so a window's
// worth of tiny messages cannot blow memory. Terminal state is signalled on
// recvDone; deliver/recv coordinate via a coalescing notify channel so recv can
// also select on a context/death channel without a condition variable.
type streamRecvQueue struct {
	mu         sync.Mutex
	items      []streamItem
	bytes      int           // currently buffered bytes
	maxBytes   int           // window (overflow => sender exceeded its credit)
	notify     chan struct{} // cap 1; "queue changed"
	recvDone   chan struct{} // closed when the recv direction is terminal
	recvClosed bool
	recvErr    error
	dead       bool
}

func newStreamRecvQueue(maxBytes int) *streamRecvQueue {
	return &streamRecvQueue{
		maxBytes: maxBytes,
		notify:   make(chan struct{}, 1),
		recvDone: make(chan struct{}),
	}
}

func (q *streamRecvQueue) signal() {
	select {
	case q.notify <- struct{}{}:
	default:
	}
}

// deliver appends a message. It returns true if the byte window was exceeded —
// i.e. the peer sent more than its granted credit — in which case the stream is
// now dead and the caller must respond to the violation. Never blocks.
func (q *streamRecvQueue) deliver(item streamItem) (overflowed bool) {
	q.mu.Lock()
	if q.recvClosed {
		q.mu.Unlock()
		return false
	}
	if q.bytes+item.cost > q.maxBytes {
		q.dead = true
		q.recvClosed = true
		q.recvErr = errStreamOverflow
		close(q.recvDone)
		q.mu.Unlock()
		return true
	}
	q.items = append(q.items, item)
	q.bytes += item.cost
	q.mu.Unlock()
	q.signal()
	return false
}

// recv blocks until a message arrives or the stream terminates. Buffered
// messages are always returned before the terminal error.
func (q *streamRecvQueue) recv() (streamItem, error) {
	for {
		q.mu.Lock()
		if len(q.items) > 0 {
			item := q.items[0]
			q.items[0] = streamItem{} // release the reader for GC
			q.items = q.items[1:]
			q.bytes -= item.cost
			q.mu.Unlock()
			return item, nil
		}
		if q.recvClosed {
			err := q.recvErr
			q.mu.Unlock()
			return streamItem{}, err
		}
		q.mu.Unlock()

		select {
		case <-q.notify:
		case <-q.recvDone:
		}
	}
}

// closeRecv marks the recv direction terminal (clean EOF when err == io.EOF).
func (q *streamRecvQueue) closeRecv(err error) {
	q.mu.Lock()
	if q.recvClosed {
		q.mu.Unlock()
		return
	}
	q.recvClosed = true
	q.recvErr = err
	close(q.recvDone)
	q.mu.Unlock()
	q.signal()
}

// die marks the whole stream dead (connection dropped / cancelled).
func (q *streamRecvQueue) die(err error) {
	q.mu.Lock()
	q.dead = true
	if !q.recvClosed {
		q.recvClosed = true
		q.recvErr = err
		close(q.recvDone)
	}
	q.mu.Unlock()
	q.signal()
}

func (q *streamRecvQueue) isDead() bool {
	q.mu.Lock()
	d := q.dead
	q.mu.Unlock()
	return d
}

// emptyStreamMessage is a zero-size sentinel passed through the middleware chain
// when a stream is opened, so that message-oriented middleware (e.g. auth) can
// gate the stream from the OPEN context metadata.
type emptyStreamMessage struct{}

func (e *emptyStreamMessage) BitSize() int                   { return 0 }
func (e *emptyStreamMessage) ToJSON() ([]byte, error)        { return []byte("{}"), nil }
func (e *emptyStreamMessage) FromJSON([]byte) error          { return nil }
func (e *emptyStreamMessage) ToBytes() []byte                { return []byte{} }
func (e *emptyStreamMessage) FromBytes([]byte) error         { return nil }
func (e *emptyStreamMessage) Serialize(*serialize.Writer)    {}
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

// serializeStreamWindowUpdate builds a WINDOW_UPDATE frame granting `increment`
// more bytes of credit to the sender on that stream (or the whole connection
// when streamID == 0).
func serializeStreamWindowUpdate(streamID uint64, increment uint64) []byte {
	size := serialize.BitsToBytes(
		BitSizePrefix() +
			serialize.BitSizeUInt64(streamID) +
			serialize.BitSizeUInt8(StreamFrameWindowUpdate) +
			serialize.BitSizeUInt64(increment))

	writer := serialize.NewWriter(size)
	SerializePrefix(writer, StreamPrefix)
	serialize.SerializeUInt64(writer, streamID)
	serialize.SerializeUInt8(writer, StreamFrameWindowUpdate)
	serialize.SerializeUInt64(writer, increment)
	return writer.Bytes()
}

// serializeStreamSettings builds the server-dictated SETTINGS frame (streamID
// 0). Sent by the server only, as the first frame on each accepted connection.
func serializeStreamSettings(initialStreamWindow uint64, initialConnectionWindow uint64) []byte {
	size := serialize.BitsToBytes(
		BitSizePrefix() +
			serialize.BitSizeUInt64(0) +
			serialize.BitSizeUInt8(StreamFrameSettings) +
			serialize.BitSizeUInt64(initialStreamWindow) +
			serialize.BitSizeUInt64(initialConnectionWindow))

	writer := serialize.NewWriter(size)
	SerializePrefix(writer, StreamPrefix)
	serialize.SerializeUInt64(writer, 0)
	serialize.SerializeUInt8(writer, StreamFrameSettings)
	serialize.SerializeUInt64(writer, initialStreamWindow)
	serialize.SerializeUInt64(writer, initialConnectionWindow)
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
	streamID  uint64
	serviceID uint64
	ctx       context.Context

	recvCh   chan *serialize.Reader
	recvDone chan struct{}

	mu         sync.Mutex
	recvClosed bool  // recv direction is terminal (io.EOF or error in recvErr)
	recvErr    error
	dead       bool // whole stream is dead (Send fails)
	sendClosed bool // local CloseSend has been issued

	// C2S flow control. The effective send credit is
	// (server-dictated initial window) + granted - sent, in bytes. Send blocks
	// (TrySend reports false) when the next message would exceed it.
	sent       int64
	granted    int64
	deadCh     chan struct{} // closed in die(); aborts a blocked Send
	sendNotify chan struct{} // cap 1; signalled when credit is granted
}

func newClientStream(client *Client, ctx context.Context, streamID uint64, serviceID uint64, bufferSize int) *ClientStream {
	return &ClientStream{
		client:     client,
		streamID:   streamID,
		serviceID:  serviceID,
		ctx:        ctx,
		recvCh:     make(chan *serialize.Reader, streamRecvBufferSizeOrDefault(bufferSize)),
		recvDone:   make(chan struct{}),
		deadCh:     make(chan struct{}),
		sendNotify: make(chan struct{}, 1),
	}
}

// addSendCredit grants n more bytes of send credit (a WINDOW_UPDATE from the
// server) and wakes any blocked Send. Called by the client demux.
func (s *ClientStream) addSendCredit(n int64) {
	s.mu.Lock()
	s.granted += n
	s.mu.Unlock()
	select {
	case s.sendNotify <- struct{}{}:
	default:
	}
}

// Context returns the context the stream was opened with.
func (s *ClientStream) Context() context.Context {
	return s.ctx
}

// Send writes a message to the server. It is safe to call from any goroutine.
// Under flow control it BLOCKS until the stream has enough send credit (or the
// stream dies / the context is cancelled), so a fast producer cannot outrun a
// slow server — backpressure flows to the caller. It returns an error once the
// stream is dead or after CloseSend. A server half-close does not stop sending.
// Frame-loop callers that must not block should use TrySend.
func (s *ClientStream) Send(msg Message) error {
	cost := int64(streamMessageCost(s.streamID, msg))

	// Block until the server has dictated the initial window (one-time per
	// connection; SETTINGS is the server's first frame so this rarely waits).
	if err := s.client.waitSettings(s.ctx, s.deadCh); err != nil {
		return err
	}
	initWin := s.client.initialStreamWindow()

	for {
		s.mu.Lock()
		if s.dead {
			s.mu.Unlock()
			return ErrStreamClosed
		}
		if s.sendClosed {
			s.mu.Unlock()
			return errors.New("stream send is already closed")
		}
		if initWin+s.granted-s.sent >= cost {
			s.sent += cost
			s.mu.Unlock()
			conn := s.client.connection()
			if conn == nil {
				return ErrStreamClosed
			}
			return sendStreamMessage(conn, s.serviceID, s.streamID, msg)
		}
		s.mu.Unlock()

		select {
		case <-s.sendNotify:
		case <-s.deadCh:
			return ErrStreamClosed
		case <-s.ctx.Done():
			return s.ctx.Err()
		}
	}
}

// TrySend is the non-blocking counterpart to Send for frame-loop callers. It
// sends the message and returns (true, nil) if credit is available; if the
// stream is out of credit it sends nothing and returns (false, nil), so the
// caller can hold the message and retry on a later frame. It returns
// (false, err) only on a terminal condition (stream dead / send closed).
func (s *ClientStream) TrySend(msg Message) (bool, error) {
	cost := int64(streamMessageCost(s.streamID, msg))

	if !s.client.settingsReady() {
		return false, nil // initial window not yet known; retry shortly
	}
	initWin := s.client.initialStreamWindow()

	s.mu.Lock()
	if s.dead {
		s.mu.Unlock()
		return false, ErrStreamClosed
	}
	if s.sendClosed {
		s.mu.Unlock()
		return false, errors.New("stream send is already closed")
	}
	if initWin+s.granted-s.sent < cost {
		s.mu.Unlock()
		return false, nil // out of credit
	}
	s.sent += cost
	s.mu.Unlock()

	conn := s.client.connection()
	if conn == nil {
		return false, ErrStreamClosed
	}
	if err := sendStreamMessage(conn, s.serviceID, s.streamID, msg); err != nil {
		return false, err
	}
	return true, nil
}

// Recv blocks until the next message arrives, the stream is cleanly closed
// (io.EOF), or an error terminates it. Buffered messages are always delivered
// before the terminal status.
func (s *ClientStream) Recv() (*serialize.Reader, error) {
	select {
	case r := <-s.recvCh:
		return r, nil
	case <-s.recvDone:
		select {
		case r := <-s.recvCh:
			return r, nil
		default:
			s.mu.Lock()
			err := s.recvErr
			s.mu.Unlock()
			return nil, err
		}
	case <-s.ctx.Done():
		// Caller cancelled; tear the stream down and notify the server.
		s.cancel(s.ctx.Err())
		return nil, s.ctx.Err()
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

	// Wake a concurrently-blocked Send so it observes the close promptly.
	select {
	case s.sendNotify <- struct{}{}:
	default:
	}

	return s.client.sendStreamFrame(serializeStreamHalfClose(s.streamID), s.serviceID)
}

// deliver enqueues an inbound message; called by the client demux on the I/O
// goroutine in arrival order. It returns true if the bounded buffer overflowed,
// in which case the stream is now dead and the caller must notify the peer.
func (s *ClientStream) deliver(r *serialize.Reader) (overflowed bool) {
	s.mu.Lock()
	if s.recvClosed {
		s.mu.Unlock()
		return false
	}
	s.mu.Unlock()

	select {
	case s.recvCh <- r:
		return false
	default:
		s.die(errStreamOverflow)
		return true
	}
}

// closeRecv marks the recv direction terminal (server done sending). The client
// may still Send. Idempotent.
func (s *ClientStream) closeRecv(err error) {
	s.mu.Lock()
	if s.recvClosed {
		s.mu.Unlock()
		return
	}
	s.recvClosed = true
	s.recvErr = err
	close(s.recvDone)
	s.mu.Unlock()
}

// die marks the whole stream dead: Send/TrySend fail, a blocked Send unblocks,
// and a pending Recv unblocks with err (unless the recv direction already ended
// cleanly). Idempotent.
func (s *ClientStream) die(err error) {
	s.mu.Lock()
	wasDead := s.dead
	s.dead = true
	if !s.recvClosed {
		s.recvClosed = true
		s.recvErr = err
		close(s.recvDone)
	}
	if !wasDead {
		close(s.deadCh)
	}
	s.mu.Unlock()

	if !wasDead {
		select {
		case s.sendNotify <- struct{}{}:
		default:
		}
	}
}

// cancel kills the stream locally and best-effort notifies the server.
func (s *ClientStream) cancel(err error) {
	s.mu.Lock()
	already := s.dead
	s.mu.Unlock()

	s.die(err)
	s.client.removeStream(s.streamID)
	if !already {
		_ = s.client.sendStreamFrame(serializeStreamClose(s.streamID, StreamStatusError, "stream cancelled by client"), s.serviceID)
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

	queue *streamRecvQueue

	// C2S flow control: as the handler consumes client messages it accrues freed
	// bytes and grants them back as a batched WINDOW_UPDATE once half the window
	// is reclaimed, replenishing the client's send credit. Guarded by replenishMu.
	window      int64
	threshold   int64
	replenishMu sync.Mutex
	pendingGrant int64
}

func newServerStream(conn Connection, ctx context.Context, streamID uint64, serviceID uint64, window uint64) *ServerStream {
	w := int64(initialStreamWindowOrDefault(window))
	return &ServerStream{
		conn:      conn,
		streamID:  streamID,
		serviceID: serviceID,
		ctx:       ctx,
		queue:     newStreamRecvQueue(int(w)),
		window:    w,
		threshold: w / 2,
	}
}

// Context returns the context the stream was opened with (carries the OPEN
// metadata, e.g. the authenticated identity).
func (s *ServerStream) Context() context.Context {
	return s.ctx
}

// Recv blocks for the next client message. It returns io.EOF when the client
// half-closes, or an error if the client cancelled / the connection dropped.
// Consuming a message frees its bytes, replenishing the client's send credit.
func (s *ServerStream) Recv() (*serialize.Reader, error) {
	item, err := s.queue.recv()
	if err != nil {
		return nil, err
	}
	s.replenish(item.cost)
	return item.reader, nil
}

// replenish accrues freed bytes and grants them back to the client as a batched
// WINDOW_UPDATE once the threshold is crossed (HTTP/2-style), rather than one
// control frame per message.
func (s *ServerStream) replenish(cost int) {
	s.replenishMu.Lock()
	s.pendingGrant += int64(cost)
	if s.pendingGrant < s.threshold {
		s.replenishMu.Unlock()
		return
	}
	grant := s.pendingGrant
	s.pendingGrant = 0
	s.replenishMu.Unlock()

	_ = s.conn.Send(serializeStreamWindowUpdate(s.streamID, uint64(grant)), s.serviceID)
}

// Send pushes a message to the client. It remains valid after the client
// half-closes (Recv returns io.EOF); it fails only once the stream is dead.
func (s *ServerStream) Send(msg Message) error {
	if s.queue.isDead() {
		return ErrStreamClosed
	}
	return sendStreamMessage(s.conn, s.serviceID, s.streamID, msg)
}

// deliver enqueues an inbound message tagged with its wire cost. Returns true if
// the client exceeded its granted credit (byte window overrun) — a protocol
// violation; the caller closes the connection.
func (s *ServerStream) deliver(r *serialize.Reader, cost int) (overflowed bool) {
	return s.queue.deliver(streamItem{reader: r, cost: cost})
}

// closeRecv marks the recv direction terminal. The handler may still Send.
func (s *ServerStream) closeRecv(err error) {
	s.queue.closeRecv(err)
}

// die marks the whole stream dead (connection dropped / client cancelled).
func (s *ServerStream) die(err error) {
	s.queue.die(err)
}

// halfClose is called when the client signals it is done sending; the handler's
// Recv returns io.EOF but it may still Send responses.
func (s *ServerStream) halfClose() {
	s.queue.closeRecv(io.EOF)
}
