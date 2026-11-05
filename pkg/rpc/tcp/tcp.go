package tcp

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kbirk/scg/pkg/rpc"
)

const (
	// smallFrameMax is the largest payload sent by copying header+payload into
	// one pooled buffer and issuing a single write. Larger payloads are written
	// as header then payload: the extra syscall is negligible next to the copy
	// it avoids.
	smallFrameMax = 32 << 10
	// readBufferSize is the per-connection read buffer: it lets a frame's
	// 4-byte header and a small body arrive in one read syscall.
	readBufferSize = 4 << 10
)

var framePool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 4+1024)
		return &b
	},
}

// applyNoDelay keeps TCP_NODELAY enabled. Go enables it on every TCP connection
// by default, and frames are always written in full, so Nagle's algorithm only
// adds latency. The NoDelay config field is retained for compatibility: true is
// the default behaviour, and false no longer disables TCP_NODELAY (doing so
// stalled each request behind the peer's delayed ACK, ~40ms+ per RPC).
func applyNoDelay(conn net.Conn) {
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
	}
}

// TCPConnection implements the Connection interface for TCP. Each frame is a
// 4-byte big-endian length followed by the payload.
type TCPConnection struct {
	conn               net.Conn
	maxSendMessageSize uint32
	maxRecvMessageSize uint32
	writeTimeout       time.Duration

	mu   sync.Mutex // serializes writers
	whdr [4]byte

	// broken is set once a write fails. The failed write may have left a
	// partial frame on the wire, so the stream is unusable: the socket is
	// closed (the peer sees EOF instead of waiting for the rest of a phantom
	// frame) and every later Send/Receive reports brokenErr.
	broken    atomic.Bool
	brokenErr error // written once under mu before broken is set

	// Receive is called from a single goroutine (the connection's read loop).
	br   *bufio.Reader
	rhdr [4]byte
}

func newTCPConnection(conn net.Conn, maxSend, maxRecv uint32, writeTimeout time.Duration) *TCPConnection {
	return &TCPConnection{
		conn:               conn,
		maxSendMessageSize: maxSend,
		maxRecvMessageSize: maxRecv,
		writeTimeout:       writeTimeout,
		br:                 bufio.NewReaderSize(conn, readBufferSize),
	}
}

// mapErr converts transport-level close conditions into rpc.ErrConnectionClosed.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
		return rpc.ErrConnectionClosed
	}
	return err
}

// failWrite marks the connection broken after a write error and closes the
// socket. Caller holds c.mu.
func (c *TCPConnection) failWrite(err error) error {
	if !c.broken.Load() {
		c.brokenErr = fmt.Errorf("connection broken by failed write: %w", err)
		c.broken.Store(true)
	}
	c.conn.Close()
	return c.brokenErr
}

// readErr maps a read error, reporting the original write failure if the
// connection was closed because a write broke the stream.
func (c *TCPConnection) readErr(err error) error {
	if c.broken.Load() {
		return c.brokenErr
	}
	return mapErr(err)
}

func (c *TCPConnection) Send(data []byte, serviceID uint64) error {
	if uint64(len(data)) > math.MaxUint32 {
		return rpc.NewMessageSizeError(uint64(len(data)), math.MaxUint32, "send")
	}
	length := uint32(len(data))

	if c.maxSendMessageSize > 0 && length > c.maxSendMessageSize {
		return rpc.NewMessageSizeError(uint64(length), uint64(c.maxSendMessageSize), "send")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.broken.Load() {
		return c.brokenErr
	}

	if c.writeTimeout > 0 {
		if err := c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout)); err != nil {
			return c.failWrite(err)
		}
	}

	if len(data) <= smallFrameMax {
		// One write per frame: never split header and payload across writes.
		bp := framePool.Get().(*[]byte)
		buf := (*bp)[:0]
		buf = binary.BigEndian.AppendUint32(buf, length)
		buf = append(buf, data...)
		_, err := c.conn.Write(buf)
		*bp = buf[:0]
		framePool.Put(bp)
		if err != nil {
			return c.writeErr(err)
		}
		return nil
	}

	binary.BigEndian.PutUint32(c.whdr[:], length)
	if _, err := c.conn.Write(c.whdr[:]); err != nil {
		return c.writeErr(err)
	}
	if _, err := c.conn.Write(data); err != nil {
		return c.writeErr(err)
	}
	return nil
}

// writeErr handles a failed write. A connection that was already closed is
// reported as closed; any other failure may have left a partial frame on the
// wire, so the connection is torn down. Caller holds c.mu.
func (c *TCPConnection) writeErr(err error) error {
	if errors.Is(err, net.ErrClosed) {
		return rpc.ErrConnectionClosed
	}
	return c.failWrite(err)
}

func (c *TCPConnection) Receive() ([]byte, error) {
	if _, err := io.ReadFull(c.br, c.rhdr[:]); err != nil {
		return nil, c.readErr(err)
	}
	length := binary.BigEndian.Uint32(c.rhdr[:])

	if c.maxRecvMessageSize > 0 && length > c.maxRecvMessageSize {
		return nil, rpc.NewMessageSizeError(uint64(length), uint64(c.maxRecvMessageSize), "receive")
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(c.br, data); err != nil {
		return nil, c.readErr(err)
	}
	return data, nil
}

func (c *TCPConnection) Close() error {
	return c.conn.Close()
}

// acceptBackoff sleeps after a failed Accept so a persistent error (e.g. the
// process is out of file descriptors) does not spin a core. It returns the
// next delay.
func acceptBackoff(delay time.Duration) time.Duration {
	if delay == 0 {
		delay = 5 * time.Millisecond
	} else {
		delay *= 2
	}
	if delay > time.Second {
		delay = time.Second
	}
	time.Sleep(delay)
	return delay
}

// ServerTransport implements ServerTransport for TCP
type ServerTransport struct {
	Port               int
	NoDelay            bool
	MaxSendMessageSize uint32
	MaxRecvMessageSize uint32
	WriteTimeout       time.Duration
	listener           net.Listener
	connCh             chan rpc.Connection
	mu                 sync.Mutex
	closed             bool
}

type ServerTransportConfig struct {
	Port int
	// NoDelay is retained for compatibility. TCP_NODELAY is always enabled;
	// setting this to false no longer re-enables Nagle's algorithm.
	NoDelay            bool
	MaxSendMessageSize uint32 // Maximum send message size in bytes (0 for no limit)
	MaxRecvMessageSize uint32 // Maximum receive message size in bytes (0 = rpc.DefaultMaxRecvMessageSize)
	// WriteTimeout, if > 0, bounds each frame write; a peer that stops reading
	// then fails the write instead of blocking its senders forever.
	WriteTimeout time.Duration
}

// recvSizeOrDefault applies the library default receive cap when none is
// configured, so transports are bounded against oversized frames out of the box.
func recvSizeOrDefault(v uint32) uint32 {
	if v == 0 {
		return rpc.DefaultMaxRecvMessageSize
	}
	return v
}

func NewServerTransport(config ServerTransportConfig) *ServerTransport {
	return &ServerTransport{
		Port:               config.Port,
		NoDelay:            config.NoDelay,
		MaxSendMessageSize: config.MaxSendMessageSize,
		MaxRecvMessageSize: recvSizeOrDefault(config.MaxRecvMessageSize),
		WriteTimeout:       config.WriteTimeout,
		connCh:             make(chan rpc.Connection, 16),
	}
}

func (t *ServerTransport) Listen() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return rpc.ErrTransportClosed
	}
	if t.listener != nil {
		return fmt.Errorf("transport is already listening")
	}

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", t.Port))
	if err != nil {
		return err
	}
	t.listener = l

	go t.acceptLoop(l)

	return nil
}

func (t *ServerTransport) acceptLoop(l net.Listener) {
	var delay time.Duration
	for {
		conn, err := l.Accept()
		if err != nil {
			t.mu.Lock()
			closed := t.closed
			t.mu.Unlock()
			if closed || errors.Is(err, net.ErrClosed) {
				return
			}
			delay = acceptBackoff(delay)
			continue
		}
		delay = 0

		applyNoDelay(conn)
		t.deliver(newTCPConnection(conn, t.MaxSendMessageSize, t.MaxRecvMessageSize, t.WriteTimeout))
	}
}

// deliver hands an accepted connection to Accept, closing it if the transport
// is closed or the backlog is full. The lock is held across the (non-blocking)
// send so Close cannot close the channel underneath it.
func (t *ServerTransport) deliver(c *TCPConnection) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		c.Close()
		return
	}
	select {
	case t.connCh <- c:
	default:
		c.Close()
	}
}

func (t *ServerTransport) Accept() (rpc.Connection, error) {
	conn, ok := <-t.connCh
	if !ok {
		return nil, rpc.ErrTransportClosed
	}
	return conn, nil
}

func (t *ServerTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true
	close(t.connCh)

	if t.listener != nil {
		return t.listener.Close()
	}
	return nil
}

// ClientTransport implements ClientTransport for TCP
type ClientTransport struct {
	Host               string
	Port               int
	NoDelay            bool
	MaxSendMessageSize uint32
	MaxRecvMessageSize uint32
	WriteTimeout       time.Duration
}

type ClientTransportConfig struct {
	Host string
	Port int
	// NoDelay is retained for compatibility. TCP_NODELAY is always enabled;
	// setting this to false no longer re-enables Nagle's algorithm.
	NoDelay            bool
	MaxSendMessageSize uint32 // Maximum send message size in bytes (0 for no limit)
	MaxRecvMessageSize uint32 // Maximum receive message size in bytes (0 = rpc.DefaultMaxRecvMessageSize)
	// WriteTimeout, if > 0, bounds each frame write.
	WriteTimeout time.Duration
}

func NewClientTransport(config ClientTransportConfig) *ClientTransport {
	return &ClientTransport{
		Host:               config.Host,
		Port:               config.Port,
		NoDelay:            config.NoDelay,
		MaxSendMessageSize: config.MaxSendMessageSize,
		MaxRecvMessageSize: recvSizeOrDefault(config.MaxRecvMessageSize),
		WriteTimeout:       config.WriteTimeout,
	}
}

func (t *ClientTransport) Connect() (rpc.Connection, error) {
	return t.ConnectContext(context.Background())
}

// ConnectContext dials the server; ctx bounds the dial.
func (t *ClientTransport) ConnectContext(ctx context.Context) (rpc.Connection, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(t.Host, strconv.Itoa(t.Port)))
	if err != nil {
		return nil, err
	}

	applyNoDelay(conn)

	return newTCPConnection(conn, t.MaxSendMessageSize, t.MaxRecvMessageSize, t.WriteTimeout), nil
}
