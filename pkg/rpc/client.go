package rpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kbirk/scg/pkg/log"
	"github.com/kbirk/scg/pkg/serialize"
)

type Client struct {
	conf      ClientConfig
	transport ClientTransport

	// mu guards the connection state below. It is never held across network
	// I/O (dial, send): a slow peer must not stall unrelated calls, Close, or
	// the receive goroutine's response dispatch.
	mu            sync.Mutex
	conn          Connection
	connGen       uint64    // bumped on every (re)connect and on Close; guards stale teardown
	dialing       *dialCall // in-flight dial shared by concurrent callers
	requests      map[uint64]chan *serialize.Reader
	streams       map[uint64]*ClientStream
	requestID     uint64
	keepaliveStop chan struct{}

	lastActivity atomic.Int64 // UnixNano of the last frame received (keepalive)

	middlewareMu sync.Mutex                   // serializes writers of middleware
	middleware   atomic.Pointer[[]Middleware] // copy-on-write; read lock-free per call
}

// dialCall is a single in-flight connection attempt that concurrent callers
// wait on instead of dialing in parallel.
type dialCall struct {
	done chan struct{}
	err  error
}

type ClientConfig struct {
	Transport  ClientTransport
	ErrHandler func(error)
	middleware []Middleware
	Logger     log.Logger
	// StreamRecvBufferSize bounds each stream's inbound queue (0 = default).
	StreamRecvBufferSize int
	// KeepaliveInterval, if > 0, enables connection-level keepalive: a PING is
	// sent after this much idle time. KeepaliveTimeout is the max idle time before
	// the connection is declared dead (defaults to 2*KeepaliveInterval; a value
	// not greater than KeepaliveInterval is raised to 2*KeepaliveInterval, since
	// otherwise an idle connection would be declared dead before it is probed).
	KeepaliveInterval time.Duration
	KeepaliveTimeout  time.Duration
}

func NewClient(conf ClientConfig) *Client {
	c := &Client{
		conf:      conf,
		transport: conf.Transport,
		requests:  make(map[uint64]chan *serialize.Reader),
		streams:   make(map[uint64]*ClientStream),
	}
	mw := append([]Middleware(nil), conf.middleware...)
	c.middleware.Store(&mw)
	return c
}

// Middleware appends a client middleware. It is safe to call concurrently with
// in-flight calls; calls that start afterwards see the new middleware.
func (c *Client) Middleware(middleware Middleware) {
	c.middlewareMu.Lock()
	defer c.middlewareMu.Unlock()
	old := *c.middleware.Load()
	next := make([]Middleware, len(old), len(old)+1)
	copy(next, old)
	next = append(next, middleware)
	c.middleware.Store(&next)
}

// GetMiddleware returns the current middleware chain. The returned slice is a
// snapshot and must not be modified.
func (c *Client) GetMiddleware() []Middleware {
	return *c.middleware.Load()
}

// Close closes the current connection and fails in-flight requests and streams.
// It does not wait for an in-progress dial: a dial that completes after Close is
// discarded. The client remains usable; a later call reconnects.
func (c *Client) Close() error {
	c.mu.Lock()
	c.connGen++ // the closed connection's goroutines become stale: no ErrHandler
	// Detach any in-flight dial: it will be discarded when it completes, so
	// calls made after Close must start a fresh dial rather than wait on it.
	c.dialing = nil
	c.stopKeepaliveUnsafe()
	conn := c.conn
	c.conn = nil
	requests := c.requests
	c.requests = make(map[uint64]chan *serialize.Reader)
	streams := c.streams
	c.streams = make(map[uint64]*ClientStream)
	c.mu.Unlock()

	var err error
	if conn != nil {
		err = conn.Close()
	}
	// Each channel is buffered (1) and was removed from the map under the lock,
	// so this is its only sender: the sends cannot block.
	for _, ch := range requests {
		ch <- nil
	}
	for _, s := range streams {
		s.die(ErrConnectionClosed)
	}
	return err
}

// teardown detaches the connection identified by gen (if it is still current)
// and fails its in-flight requests and streams with cause. It reports whether
// it acted.
func (c *Client) teardown(gen uint64, cause error) bool {
	c.mu.Lock()
	if gen != c.connGen {
		// Stale connection; its teardown already happened (or Close ran).
		c.mu.Unlock()
		return false
	}
	c.connGen++
	c.stopKeepaliveUnsafe()
	conn := c.conn
	c.conn = nil
	requests := c.requests
	c.requests = make(map[uint64]chan *serialize.Reader)
	streams := c.streams
	c.streams = make(map[uint64]*ClientStream)
	c.mu.Unlock()

	if conn != nil {
		conn.Close()
	}
	for _, s := range streams {
		s.die(cause)
	}
	for _, ch := range requests {
		ch <- nil
	}
	return true
}

// handleError tears down the connection identified by gen and fails its
// in-flight requests/streams, reporting err to the ErrHandler. A stale gen (a
// connection already replaced or closed) is ignored.
func (c *Client) handleError(gen uint64, err error) error {
	if !c.teardown(gen, fmt.Errorf("connection error: %w", err)) {
		return err
	}
	c.logError("Encountered error: " + err.Error())
	if c.conf.ErrHandler != nil {
		c.conf.ErrHandler(err)
	}
	return err
}

// handleConnectionClosed tears down the connection identified by gen after a
// clean close, failing its in-flight requests and streams so they don't hang.
// Unlike handleError it does not log or invoke ErrHandler — a close is normal.
func (c *Client) handleConnectionClosed(gen uint64) {
	c.teardown(gen, ErrConnectionClosed)
}

func (c *Client) logDebug(msg string) {
	if c.conf.Logger != nil {
		c.conf.Logger.Debug(msg)
	}
}

func (c *Client) logInfo(msg string) {
	if c.conf.Logger != nil {
		c.conf.Logger.Info(msg)
	}
}

func (c *Client) logWarn(msg string) {
	if c.conf.Logger != nil {
		c.conf.Logger.Warn(msg)
	}
}

func (c *Client) logError(msg string) {
	if c.conf.Logger != nil {
		c.conf.Logger.Error(msg)
	}
}

func (c *Client) dial(ctx context.Context) (Connection, error) {
	if ct, ok := c.transport.(ContextClientTransport); ok {
		return ct.ConnectContext(ctx)
	}
	return c.transport.Connect()
}

func isContextErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// ensureConnected returns the current connection and its generation, dialing
// if necessary. Concurrent callers share one dial; each waits only as long as
// its own ctx allows. The dial runs without holding c.mu.
func (c *Client) ensureConnected(ctx context.Context) (Connection, uint64, error) {
	for {
		c.mu.Lock()
		if c.conn != nil {
			conn, gen := c.conn, c.connGen
			c.mu.Unlock()
			return conn, gen, nil
		}
		if d := c.dialing; d != nil {
			c.mu.Unlock()
			select {
			case <-d.done:
			case <-ctx.Done():
				return nil, 0, ctx.Err()
			}
			if d.err != nil && !(isContextErr(d.err) && ctx.Err() == nil) {
				return nil, 0, d.err
			}
			// Either connected, or the dialer gave up because of its own
			// context while ours is still live: loop and re-check / re-dial.
			continue
		}
		d := &dialCall{done: make(chan struct{})}
		c.dialing = d
		startGen := c.connGen
		c.mu.Unlock()

		c.logDebug("Connecting to server")
		conn, err := c.dial(ctx)

		c.mu.Lock()
		if c.dialing == d {
			c.dialing = nil
		}
		if err == nil && c.connGen != startGen {
			// Close() ran while we were dialing: discard the new connection.
			err = ErrConnectionClosed
			c.mu.Unlock()
			conn.Close()
		} else if err != nil {
			c.mu.Unlock()
		} else {
			gen := c.attachUnsafe(conn)
			c.mu.Unlock()
			d.err = nil
			close(d.done)
			return conn, gen, nil
		}
		d.err = err
		close(d.done)
		return nil, 0, err
	}
}

// attachUnsafe installs conn as the current connection and starts its receive
// and keepalive goroutines. Caller holds c.mu.
func (c *Client) attachUnsafe(conn Connection) uint64 {
	c.conn = conn
	c.connGen++
	gen := c.connGen
	c.lastActivity.Store(time.Now().UnixNano())

	go c.receiveLoop(conn, gen)

	if c.conf.KeepaliveInterval > 0 {
		stop := make(chan struct{})
		c.keepaliveStop = stop
		interval := c.conf.KeepaliveInterval
		timeout := c.conf.KeepaliveTimeout
		if timeout <= interval {
			timeout = 2 * interval
		}
		go c.keepaliveLoop(conn, gen, interval, timeout, stop)
	}
	return gen
}

func (c *Client) receiveLoop(conn Connection, gen uint64) {
	var pongInFlight atomic.Bool
	for {
		bs, err := conn.Receive()
		if err != nil {
			// A clean close is not an error, but the in-flight requests/streams
			// must still be failed so they don't hang.
			if IsConnectionClosed(err) {
				c.logDebug("Connection closed normally")
				c.handleConnectionClosed(gen)
				return
			}
			c.handleError(gen, err)
			return
		}
		c.lastActivity.Store(time.Now().UnixNano())

		reader := serialize.NewReader(bs)

		var prefix [16]byte
		err = DeserializePrefix(&prefix, reader)
		if err != nil {
			c.handleError(gen, err)
			return
		}

		switch prefix {
		case ResponsePrefix:
			var requestID uint64
			err = serialize.DeserializeUInt64(&requestID, reader)
			if err != nil {
				c.handleError(gen, err)
				return
			}

			c.mu.Lock()
			ch, ok := c.requests[requestID]
			delete(c.requests, requestID)
			c.mu.Unlock()

			if !ok {
				// A context cancellation cleaned up the request before the
				// response arrived. Discard the response.
				continue
			}

			ch <- reader

		case StreamPrefix:
			if err := c.handleStreamFrame(conn, reader, &pongInFlight); err != nil {
				c.handleError(gen, err)
				return
			}

		default:
			c.handleError(gen, fmt.Errorf("unexpected prefix: %v", prefix))
			return
		}
	}
}

// sendAsync sends a small control frame without blocking the caller, keeping at
// most one such send in flight per flag. A stuck peer therefore cannot block
// the receive or keepalive goroutine (the keepalive can still detect the dead
// connection and close it, which unblocks the stuck write).
func sendAsync(conn Connection, frame []byte, inFlight *atomic.Bool) {
	if !inFlight.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer inFlight.Store(false)
		_ = conn.Send(frame, 0)
	}()
}

// keepaliveLoop periodically probes the connection with a PING when idle and
// fails the connection if no frame arrives within the timeout window. It exits
// when stop is closed (on disconnect).
func (c *Client) keepaliveLoop(conn Connection, gen uint64, interval, timeout time.Duration, stop chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var pingInFlight atomic.Bool
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			idle := time.Since(time.Unix(0, c.lastActivity.Load()))
			if idle > timeout {
				c.handleError(gen, fmt.Errorf("keepalive timeout: no activity for %s", idle))
				return
			}
			// Probe once idle for half an interval (not a full one): the check
			// only runs once per tick, so waiting a full interval could leave an
			// idle connection unprobed for almost two intervals, and tick jitter
			// would then trip the 2x-interval timeout on a healthy peer.
			if idle >= interval/2 {
				sendAsync(conn, serializeStreamControl(StreamFramePing), &pingInFlight)
			}
		}
	}
}

// stopKeepaliveUnsafe stops the keepalive loop (caller holds mu).
func (c *Client) stopKeepaliveUnsafe() {
	if c.keepaliveStop != nil {
		close(c.keepaliveStop)
		c.keepaliveStop = nil
	}
}

func (c *Client) sendMessage(ctx context.Context, serviceID uint64, methodID uint64, msg Message) (uint64, chan *serialize.Reader, error) {
	conn, gen, err := c.ensureConnected(ctx)
	if err != nil {
		return 0, nil, err
	}

	// Register the request before sending so the response cannot be missed.
	// With a buffered channel of size 1, a late send succeeds without blocking.
	ch := make(chan *serialize.Reader, 1)
	c.mu.Lock()
	if c.connGen != gen {
		// The connection was torn down between connecting and registering.
		c.mu.Unlock()
		return 0, nil, ErrConnectionClosed
	}
	requestID := c.requestID
	c.requestID++
	c.requests[requestID] = ch
	c.mu.Unlock()

	size := int(serialize.BitsToBytes(
		BitSizePrefix() +
			BitSizeContext(ctx) +
			serialize.BitSizeUInt64(requestID) +
			serialize.BitSizeUInt64(serviceID) +
			serialize.BitSizeUInt64(methodID) +
			msg.BitSize()))

	writer := getWriter(size)
	defer putWriter(writer)

	SerializePrefix(writer, RequestPrefix)
	SerializeContext(writer, ctx)
	serialize.SerializeUInt64(writer, requestID)
	serialize.SerializeUInt64(writer, serviceID)
	serialize.SerializeUInt64(writer, methodID)
	msg.Serialize(writer)

	// Send without holding c.mu; the connection serializes its own writes.
	if err := conn.Send(writer.Bytes(), serviceID); err != nil {
		c.mu.Lock()
		delete(c.requests, requestID)
		c.mu.Unlock()
		if IsConnectionClosed(err) {
			c.handleConnectionClosed(gen)
			return 0, nil, err
		}
		if errors.Is(err, ErrMessageTooLarge) {
			// A local size check, not a connection failure.
			return 0, nil, err
		}
		return 0, nil, c.handleError(gen, err)
	}
	return requestID, ch, nil
}

func (c *Client) receiveMessage(ctx context.Context, requestID uint64, ch chan *serialize.Reader) (*serialize.Reader, error) {
	select {
	case reader := <-ch:
		if reader == nil {
			return nil, errors.New("channel closed")
		}

		var responseType uint8
		if err := serialize.DeserializeUInt8(&responseType, reader); err != nil {
			return nil, fmt.Errorf("malformed response: %w", err)
		}

		if responseType == MessageResponse {
			return reader, nil
		}

		var errMsg string
		if err := serialize.DeserializeString(&errMsg, reader); err != nil {
			return nil, fmt.Errorf("malformed error response: %w", err)
		}
		return nil, errors.New(errMsg)
	case <-ctx.Done():
		// Context cancelled or timed out — clean up the request entry so the
		// receive goroutine doesn't deliver to the orphaned channel.
		c.mu.Lock()
		delete(c.requests, requestID)
		c.mu.Unlock()
		return nil, ctx.Err()
	}
}

func (c *Client) Call(ctx context.Context, serviceID uint64, methodID uint64, msg Message) (*serialize.Reader, error) {
	requestID, ch, err := c.sendMessage(ctx, serviceID, methodID, msg)
	if err != nil {
		return nil, err
	}

	return c.receiveMessage(ctx, requestID, ch)
}

// OpenStream opens a bidirectional stream against the given service/method. The
// returned ClientStream is registered with the client demux before the OPEN
// frame is sent, so no inbound frame can be missed.
func (c *Client) OpenStream(ctx context.Context, serviceID uint64, methodID uint64) (*ClientStream, error) {
	conn, gen, err := c.ensureConnected(ctx)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	if c.connGen != gen {
		c.mu.Unlock()
		return nil, ErrConnectionClosed
	}
	streamID := c.requestID
	c.requestID++
	stream := newClientStream(c, conn, ctx, streamID, serviceID, c.conf.StreamRecvBufferSize)
	c.streams[streamID] = stream
	c.mu.Unlock()

	if err := conn.Send(serializeStreamOpen(ctx, streamID, serviceID, methodID), serviceID); err != nil {
		c.removeStream(streamID)
		stream.die(err)
		if IsConnectionClosed(err) {
			c.handleConnectionClosed(gen)
			return nil, err
		}
		return nil, c.handleError(gen, err)
	}
	return stream, nil
}

func (c *Client) removeStream(streamID uint64) {
	c.mu.Lock()
	delete(c.streams, streamID)
	c.mu.Unlock()
}

// handleStreamFrame routes an inbound stream frame to the correct ClientStream.
// Runs on the connection's single receive goroutine, preserving per-stream order.
func (c *Client) handleStreamFrame(conn Connection, reader *serialize.Reader, pongInFlight *atomic.Bool) error {
	var streamID uint64
	if err := serialize.DeserializeUInt64(&streamID, reader); err != nil {
		return err
	}

	var frameKind uint8
	if err := serialize.DeserializeUInt8(&frameKind, reader); err != nil {
		return err
	}

	// Connection-level keepalive frames are not associated with a stream.
	if frameKind == StreamFramePing {
		sendAsync(conn, serializeStreamControl(StreamFramePong), pongInFlight)
		return nil
	}
	if frameKind == StreamFramePong {
		return nil // liveness already recorded via lastActivity
	}

	c.mu.Lock()
	stream, ok := c.streams[streamID]
	c.mu.Unlock()

	if !ok {
		// Unknown stream (already closed/cancelled locally) — discard.
		return nil
	}

	switch frameKind {
	case StreamFrameMessage:
		if stream.deliver(reader) {
			// Bounded buffer overflowed: notify the server and drop the stream.
			_ = conn.Send(serializeStreamClose(streamID, StreamStatusError, errStreamOverflow.Error()), stream.serviceID)
			c.removeStream(streamID)
		}

	case StreamFrameHalfClose:
		// Server is done sending; surface a clean EOF on Recv. The client may
		// still Send until it CloseSends or the stream is fully closed.
		stream.closeRecv(io.EOF)

	case StreamFrameClose:
		var status uint8
		if err := serialize.DeserializeUInt8(&status, reader); err != nil {
			return err
		}
		var message string
		if err := serialize.DeserializeString(&message, reader); err != nil {
			return err
		}
		if status == StreamStatusOK {
			stream.die(io.EOF)
		} else {
			if message == "" {
				message = "stream closed with error"
			}
			stream.die(errors.New(message))
		}
		c.removeStream(streamID)

	default:
		return fmt.Errorf("unknown stream frame kind: %d", frameKind)
	}

	return nil
}
