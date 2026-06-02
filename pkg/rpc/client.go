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
	conf          ClientConfig
	mu            *sync.Mutex
	conn          Connection
	transport     ClientTransport
	requests      map[uint64]chan *serialize.Reader
	streams       map[uint64]*ClientStream
	requestID     uint64
	running       bool
	connGen       uint64       // bumped on each (re)connect; guards stale-connection teardown
	lastActivity  atomic.Int64 // UnixNano of the last frame received (keepalive)
	keepaliveStop chan struct{}
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
	// the connection is declared dead (defaults to 2*KeepaliveInterval).
	KeepaliveInterval time.Duration
	KeepaliveTimeout  time.Duration
}

func NewClient(conf ClientConfig) *Client {
	return &Client{
		conf:      conf,
		transport: conf.Transport,
		mu:        &sync.Mutex{},
		requests:  make(map[uint64]chan *serialize.Reader),
		streams:   make(map[uint64]*ClientStream),
	}
}

func (c *Client) Middleware(middleware Middleware) {
	c.conf.middleware = append(c.conf.middleware, middleware)
}

func (c *Client) GetMiddleware() []Middleware {
	return c.conf.middleware
}

func (c *Client) Close() error {
	c.mu.Lock()
	c.stopKeepaliveUnsafe()
	streams := c.streams
	c.streams = make(map[uint64]*ClientStream)
	var err error
	if c.conn != nil {
		err = c.conn.Close()
		c.conn = nil

		// Notify all pending requests so they don't block forever.
		// The receive goroutine treats a closed connection as a normal exit
		// and won't call handleError, so we must clean up here.
		requests := c.requests
		c.requests = make(map[uint64]chan *serialize.Reader)
		for _, ch := range requests {
			ch <- nil
		}
	}
	c.mu.Unlock()

	for _, s := range streams {
		s.die(errors.New("connection closed"))
	}
	return err
}

// handleError tears down the connection identified by gen and fails its
// in-flight requests/streams. gen guards against a stale goroutine (from a
// connection that has since been replaced by a reconnect) tearing down the new
// connection's state.
func (c *Client) handleError(gen uint64, err error) error {
	c.mu.Lock()
	if gen != c.connGen {
		// Stale connection; its teardown already happened. Don't touch the
		// current connection's requests/streams.
		c.mu.Unlock()
		return err
	}
	c.stopKeepaliveUnsafe()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	requests := c.requests
	c.requests = make(map[uint64]chan *serialize.Reader)
	streams := c.streams
	c.streams = make(map[uint64]*ClientStream)
	c.mu.Unlock()

	c.logError("Encountered error: " + err.Error())
	if c.conf.ErrHandler != nil {
		c.conf.ErrHandler(err)
	}

	// Fail all in-flight streams.
	for _, s := range streams {
		s.die(fmt.Errorf("connection error: %w", err))
	}

	// Notify all pending requests of the error
	go func() {
		for _, ch := range requests {
			ch <- nil
		}
	}()

	return err
}

// handleConnectionClosed tears down the connection identified by gen after a
// clean server-initiated close, failing its in-flight requests and streams so
// they don't hang. Unlike handleError it does not log or invoke ErrHandler — a
// close is normal. gen guards against a stale goroutine from a replaced
// connection tearing down the current one's state.
func (c *Client) handleConnectionClosed(gen uint64) {
	c.mu.Lock()
	if gen != c.connGen {
		c.mu.Unlock()
		return
	}
	c.stopKeepaliveUnsafe()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	requests := c.requests
	c.requests = make(map[uint64]chan *serialize.Reader)
	streams := c.streams
	c.streams = make(map[uint64]*ClientStream)
	c.mu.Unlock()

	// Fail all in-flight streams.
	for _, s := range streams {
		s.die(errors.New("connection closed"))
	}

	// Notify all pending requests so they don't block forever.
	go func() {
		for _, ch := range requests {
			ch <- nil
		}
	}()
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

func (c *Client) connectUnsafe() error {
	if c.conn != nil {
		return nil
	}

	// connect using transport
	c.logDebug("Connecting to server")
	conn, err := c.transport.Connect()
	if err != nil {
		return err
	}
	c.conn = conn
	c.connGen++
	gen := c.connGen

	go func() {
		for {
			c.logDebug("Waiting for message")
			bs, err := conn.Receive()
			if err != nil {
				// A clean server-initiated close is not an error, but the
				// in-flight requests/streams must still be failed so they don't
				// hang (mirrors the C++ client surfacing a clean close).
				if err.Error() == "connection closed" {
					c.logDebug("Connection closed normally")
					c.handleConnectionClosed(gen)
					return
				}
				c.handleError(gen, err)
				return
			}
			c.logDebug("Received message")
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
					// This can happen when a context cancellation cleaned up the request
					// before the response arrived. Just discard the response.
					continue
				}

				ch <- reader

			case StreamPrefix:
				if err := c.handleStreamFrame(reader); err != nil {
					c.handleError(gen, err)
					return
				}

			default:
				c.handleError(gen, fmt.Errorf("unexpected prefix: %v", prefix))
				return
			}
		}
	}()

	// Start connection-level keepalive if configured.
	if c.conf.KeepaliveInterval > 0 {
		c.lastActivity.Store(time.Now().UnixNano())
		stop := make(chan struct{})
		c.keepaliveStop = stop
		interval := c.conf.KeepaliveInterval
		timeout := c.conf.KeepaliveTimeout
		if timeout <= 0 {
			timeout = 2 * interval
		}
		go c.keepaliveLoop(gen, interval, timeout, stop)
	}

	return nil
}

// keepaliveLoop periodically probes the connection with a PING when idle and
// fails the connection if no frame arrives within the timeout window. It exits
// when stop is closed (on disconnect) or the connection send fails.
func (c *Client) keepaliveLoop(gen uint64, interval, timeout time.Duration, stop chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

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
			if idle >= interval {
				if err := c.sendStreamFrame(serializeStreamControl(StreamFramePing), 0); err != nil {
					return
				}
			}
		}
	}
}

// stopKeepaliveUnsafe stops the keepalive loop (caller holds mu_).
func (c *Client) stopKeepaliveUnsafe() {
	if c.keepaliveStop != nil {
		close(c.keepaliveStop)
		c.keepaliveStop = nil
	}
}

func (c *Client) sendMessage(ctx context.Context, serviceID uint64, methodID uint64, msg Message) (uint64, chan *serialize.Reader, error) {
	// Get next request ID
	c.mu.Lock()
	requestID := c.requestID
	c.requestID++
	c.mu.Unlock()

	// Serialize message
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
	bs := writer.Bytes()

	// Ensure connection and register request
	c.mu.Lock()
	err := c.connectUnsafe()
	if err != nil {
		c.mu.Unlock()
		return 0, nil, err
	}

	// With a buffered channel of size 1, a late send succeeds without blocking
	ch := make(chan *serialize.Reader, 1)
	c.requests[requestID] = ch

	// Send message
	err = c.conn.Send(bs, serviceID)
	if err != nil {
		delete(c.requests, requestID)
		gen := c.connGen
		c.mu.Unlock()
		return 0, nil, c.handleError(gen, err)
	}

	c.mu.Unlock()
	return requestID, ch, nil
}

func (c *Client) receiveMessage(ctx context.Context, requestID uint64, ch chan *serialize.Reader) (*serialize.Reader, error) {
	select {
	case reader := <-ch:
		if reader == nil {
			return nil, errors.New("channel closed")
		}

		var responseType uint8
		serialize.DeserializeUInt8(&responseType, reader)

		if responseType == MessageResponse {
			return reader, nil
		}

		var errMsg string
		serialize.DeserializeString(&errMsg, reader)
		return nil, errors.New(errMsg)
	case <-ctx.Done():
		// Context cancelled or timed out — clean up the request entry so the
		// receive goroutine doesn't block trying to send on the orphaned channel.
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
	c.mu.Lock()

	if err := c.connectUnsafe(); err != nil {
		c.mu.Unlock()
		return nil, err
	}

	streamID := c.requestID
	c.requestID++

	stream := newClientStream(c, ctx, streamID, serviceID, c.conf.StreamRecvBufferSize)
	c.streams[streamID] = stream

	err := c.conn.Send(serializeStreamOpen(ctx, streamID, serviceID, methodID), serviceID)
	if err != nil {
		delete(c.streams, streamID)
		gen := c.connGen
		c.mu.Unlock()
		return nil, c.handleError(gen, err)
	}

	c.mu.Unlock()
	return stream, nil
}

// connection returns the current connection (nil if not connected).
func (c *Client) connection() Connection {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	return conn
}

// sendStreamFrame writes a pre-serialized stream frame to the connection.
func (c *Client) sendStreamFrame(bs []byte, serviceID uint64) error {
	conn := c.connection()
	if conn == nil {
		return ErrStreamClosed
	}
	return conn.Send(bs, serviceID)
}

func (c *Client) removeStream(streamID uint64) {
	c.mu.Lock()
	delete(c.streams, streamID)
	c.mu.Unlock()
}

// handleStreamFrame routes an inbound stream frame to the correct ClientStream.
// Runs on the client's single receive goroutine, preserving per-stream order.
func (c *Client) handleStreamFrame(reader *serialize.Reader) error {
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
		return c.sendStreamFrame(serializeStreamControl(StreamFramePong), 0)
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
			_ = c.sendStreamFrame(serializeStreamClose(streamID, StreamStatusError, errStreamOverflow.Error()), stream.serviceID)
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
