package websocket

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kbirk/scg/pkg/rpc"
)

const (
	// DefaultHandshakeTimeout bounds the client's websocket (and TLS) handshake
	// when none is configured.
	DefaultHandshakeTimeout = 45 * time.Second

	readBufferSize = 4 << 10
	// The server writes each message as a single frame regardless of buffer
	// size, so a small buffer suffices. The client masks frames through its
	// write buffer, so a larger buffer means fewer frames/syscalls per large
	// message. (gorilla's WriteBufferPool is avoided: it allocates per write.)
	serverWriteBufferSize = 4 << 10
	clientWriteBufferSize = 32 << 10
)

// WebSocketConnection implements the Connection interface for WebSocket
type WebSocketConnection struct {
	conn               *websocket.Conn
	mu                 *sync.Mutex
	maxSendMessageSize uint32
	maxRecvMessageSize uint32
	writeTimeout       time.Duration
	closeOnce          sync.Once
	closeErr           error
	// closed is set before a local Close. gorilla/websocket rewraps net
	// errors (dropping net.ErrClosed from the chain), so I/O errors after a
	// local close are classified by this flag rather than by errors.Is.
	closed atomic.Bool
	// broken is set once a write fails: the failed write may have left a
	// partial frame on the wire, so the socket is closed and every later
	// Send/Receive reports brokenErr.
	broken    atomic.Bool
	brokenErr error // written once under mu before broken is set
}

// closeFrameTimeout bounds the best-effort close frame sent by Close.
const closeFrameTimeout = 250 * time.Millisecond

func newConnection(conn *websocket.Conn, maxSend, maxRecv uint32, writeTimeout time.Duration) *WebSocketConnection {
	// Bound the read at the websocket layer so an oversized frame is rejected
	// during the read instead of being fully buffered before the size check.
	if maxRecv > 0 {
		conn.SetReadLimit(int64(maxRecv))
	}
	return &WebSocketConnection{
		conn:               conn,
		mu:                 &sync.Mutex{},
		maxSendMessageSize: maxSend,
		maxRecvMessageSize: maxRecv,
		writeTimeout:       writeTimeout,
	}
}

func (c *WebSocketConnection) mapErr(err error) error {
	if err == nil {
		return nil
	}
	if c.broken.Load() {
		return c.brokenErr
	}
	if c.closed.Load() {
		return rpc.ErrConnectionClosed
	}
	return mapErr(err)
}

// failWrite marks the connection broken after a write error and closes the
// socket so the peer sees the connection end instead of waiting on a partial
// frame. Caller holds c.mu.
func (c *WebSocketConnection) failWrite(err error) error {
	if c.closed.Load() {
		return rpc.ErrConnectionClosed // closed locally; not a broken stream
	}
	if mapped := mapErr(err); errors.Is(mapped, rpc.ErrConnectionClosed) {
		return mapped
	}
	if !c.broken.Load() {
		c.brokenErr = fmt.Errorf("connection broken by failed write: %w", err)
		c.broken.Store(true)
	}
	c.conn.Close()
	return c.brokenErr
}

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) ||
		errors.Is(err, net.ErrClosed) || errors.Is(err, websocket.ErrCloseSent) {
		return rpc.ErrConnectionClosed
	}
	return err
}

func (c *WebSocketConnection) Send(data []byte, serviceID uint64) error {
	if c.maxSendMessageSize > 0 && uint64(len(data)) > uint64(c.maxSendMessageSize) {
		return rpc.NewMessageSizeError(uint64(len(data)), uint64(c.maxSendMessageSize), "send")
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
	if err := c.conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return c.failWrite(err)
	}
	return nil
}

func (c *WebSocketConnection) Receive() ([]byte, error) {
	_, data, err := c.conn.ReadMessage()
	if err != nil {
		return nil, c.mapErr(err)
	}

	if c.maxRecvMessageSize > 0 && uint64(len(data)) > uint64(c.maxRecvMessageSize) {
		return nil, rpc.NewMessageSizeError(uint64(len(data)), uint64(c.maxRecvMessageSize), "receive")
	}

	return data, nil
}

// Close sends a normal-closure frame (best effort, bounded) so the peer sees a
// clean close rather than an abnormal disconnect, then closes the socket. It is
// safe to call concurrently with Send/Receive and more than once.
func (c *WebSocketConnection) Close() error {
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		_ = c.conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			time.Now().Add(closeFrameTimeout))
		c.closeErr = c.conn.Close()
	})
	return c.closeErr
}

// ForceClose closes the socket immediately, without attempting a close frame.
// It implements rpc.ForceCloser so a server shutdown whose deadline has
// passed is not delayed by peers that have stopped reading. It also unblocks
// a concurrent Close waiting to write its close frame.
func (c *WebSocketConnection) ForceClose() error {
	c.closed.Store(true)
	return c.conn.Close()
}

// ServerTransport implements ServerTransport for WebSocket
type ServerTransport struct {
	Port               int
	CertFile           string
	KeyFile            string
	MaxSendMessageSize uint32
	MaxRecvMessageSize uint32
	WriteTimeout       time.Duration
	server             *http.Server
	addr               net.Addr // bound address (useful when Port is 0)
	upgrader           websocket.Upgrader
	connCh             chan rpc.Connection
	mu                 *sync.Mutex
	closed             bool
}

type ServerTransportConfig struct {
	Port               int
	CertFile           string // Optional: for TLS
	KeyFile            string // Optional: for TLS
	MaxSendMessageSize uint32 // Maximum send message size in bytes (0 for no limit)
	MaxRecvMessageSize uint32 // Maximum receive message size in bytes (0 = rpc.DefaultMaxRecvMessageSize)
	// WriteTimeout, if > 0, bounds each message write.
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
		CertFile:           config.CertFile,
		KeyFile:            config.KeyFile,
		MaxSendMessageSize: config.MaxSendMessageSize,
		MaxRecvMessageSize: recvSizeOrDefault(config.MaxRecvMessageSize),
		WriteTimeout:       config.WriteTimeout,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  readBufferSize,
			WriteBufferSize: serverWriteBufferSize,
		},
		connCh: make(chan rpc.Connection, 16), // buffered channel for connections
		mu:     &sync.Mutex{},
	}
}

// Listen binds the port synchronously, so bind and certificate errors are
// returned here, then serves in the background.
func (t *ServerTransport) Listen() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return rpc.ErrTransportClosed
	}
	if t.server != nil {
		return fmt.Errorf("transport is already listening")
	}

	var tlsConfig *tls.Config
	if t.CertFile != "" && t.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(t.CertFile, t.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to load certificate: %w", err)
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
	}

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", t.Port))
	if err != nil {
		return err
	}
	t.addr = l.Addr()
	if tlsConfig != nil {
		l = tls.NewListener(l, tlsConfig)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", t.handleWebSocket)

	t.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 30 * time.Second,
	}
	server := t.server

	go func() {
		// Serve returns http.ErrServerClosed after Close.
		_ = server.Serve(l)
	}()

	return nil
}

func (t *ServerTransport) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := t.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	wsConn := newConnection(conn, t.MaxSendMessageSize, t.MaxRecvMessageSize, t.WriteTimeout)

	// Hold the lock across the non-blocking send so Close cannot close the
	// channel between the closed check and the send.
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		conn.Close()
		return
	}
	select {
	case t.connCh <- wsConn:
	default:
		// Channel is full, close the connection
		conn.Close()
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
		return nil // Already closed
	}

	t.closed = true
	close(t.connCh)

	if t.server != nil {
		return t.server.Close()
	}
	return nil
}

// ClientTransport implements ClientTransport for WebSocket
type ClientTransport struct {
	Host               string
	Port               int
	TLSConfig          *tls.Config
	MaxSendMessageSize uint32
	MaxRecvMessageSize uint32
	WriteTimeout       time.Duration
	HandshakeTimeout   time.Duration
}

type ClientTransportConfig struct {
	Host               string
	Port               int
	TLSConfig          *tls.Config
	MaxSendMessageSize uint32 // Maximum send message size in bytes (0 for no limit)
	MaxRecvMessageSize uint32 // Maximum receive message size in bytes (0 = rpc.DefaultMaxRecvMessageSize)
	// WriteTimeout, if > 0, bounds each message write.
	WriteTimeout time.Duration
	// HandshakeTimeout bounds the websocket (and TLS) handshake
	// (0 = DefaultHandshakeTimeout). The caller's context deadline also applies.
	HandshakeTimeout time.Duration
}

func NewClientTransport(config ClientTransportConfig) *ClientTransport {
	return &ClientTransport{
		Host:               config.Host,
		Port:               config.Port,
		TLSConfig:          config.TLSConfig,
		MaxSendMessageSize: config.MaxSendMessageSize,
		MaxRecvMessageSize: recvSizeOrDefault(config.MaxRecvMessageSize),
		WriteTimeout:       config.WriteTimeout,
		HandshakeTimeout:   config.HandshakeTimeout,
	}
}

func (t *ClientTransport) Connect() (rpc.Connection, error) {
	return t.ConnectContext(context.Background())
}

// ConnectContext dials and upgrades; ctx bounds the dial and handshake, which
// are additionally bounded by HandshakeTimeout.
func (t *ClientTransport) ConnectContext(ctx context.Context) (rpc.Connection, error) {
	scheme := "ws"

	handshakeTimeout := t.HandshakeTimeout
	if handshakeTimeout <= 0 {
		handshakeTimeout = DefaultHandshakeTimeout
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: handshakeTimeout,
		ReadBufferSize:   readBufferSize,
		WriteBufferSize:  clientWriteBufferSize,
	}
	if t.TLSConfig != nil {
		// Configure the Dialer to use SSL/TLS
		dialer.TLSClientConfig = t.TLSConfig
		scheme = "wss"
	}

	u := url.URL{Scheme: scheme, Host: net.JoinHostPort(t.Host, fmt.Sprint(t.Port)), Path: "/rpc"}

	// connect to the WebSocket server
	conn, _, err := dialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return nil, err
	}

	return newConnection(conn, t.MaxSendMessageSize, t.MaxRecvMessageSize, t.WriteTimeout), nil
}
