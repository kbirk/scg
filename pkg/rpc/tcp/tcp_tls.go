package tcp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/kbirk/scg/pkg/rpc"
)

// DefaultTLSHandshakeTimeout bounds a TLS handshake when none is configured, so
// a peer that connects and never completes the handshake cannot hold a
// connection (and, on the client, a call) open forever.
const DefaultTLSHandshakeTimeout = 10 * time.Second

func handshakeTimeoutOrDefault(d time.Duration) time.Duration {
	if d <= 0 {
		return DefaultTLSHandshakeTimeout
	}
	return d
}

// ServerTransportTLS implements ServerTransport for TCP with TLS
type ServerTransportTLS struct {
	Port               int
	NoDelay            bool
	CertFile           string
	KeyFile            string
	MaxSendMessageSize uint32
	MaxRecvMessageSize uint32
	WriteTimeout       time.Duration
	HandshakeTimeout   time.Duration
	listener           net.Listener
	tlsConfig          *tls.Config
	connCh             chan rpc.Connection
	mu                 sync.Mutex
	closed             bool
}

type ServerTransportTLSConfig struct {
	Port int
	// NoDelay is retained for compatibility. TCP_NODELAY is always enabled
	// (Go's default); setting this to false no longer re-enables Nagle.
	NoDelay            bool
	CertFile           string // Server certificate file (PEM)
	KeyFile            string // Server private key file (PEM)
	MaxSendMessageSize uint32 // Maximum send message size in bytes (0 for no limit)
	MaxRecvMessageSize uint32 // Maximum receive message size in bytes (0 = rpc.DefaultMaxRecvMessageSize)
	// WriteTimeout, if > 0, bounds each frame write.
	WriteTimeout time.Duration
	// HandshakeTimeout bounds each client's TLS handshake (0 = DefaultTLSHandshakeTimeout).
	HandshakeTimeout time.Duration
}

func NewServerTransportTLS(config ServerTransportTLSConfig) *ServerTransportTLS {
	return &ServerTransportTLS{
		Port:               config.Port,
		NoDelay:            config.NoDelay,
		CertFile:           config.CertFile,
		KeyFile:            config.KeyFile,
		MaxSendMessageSize: config.MaxSendMessageSize,
		MaxRecvMessageSize: recvSizeOrDefault(config.MaxRecvMessageSize),
		WriteTimeout:       config.WriteTimeout,
		HandshakeTimeout:   config.HandshakeTimeout,
		connCh:             make(chan rpc.Connection, 16),
	}
}

func (t *ServerTransportTLS) Listen() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return rpc.ErrTransportClosed
	}
	if t.listener != nil {
		return fmt.Errorf("transport is already listening")
	}

	cert, err := tls.LoadX509KeyPair(t.CertFile, t.KeyFile)
	if err != nil {
		return fmt.Errorf("failed to load certificate: %w", err)
	}

	t.tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", t.Port))
	if err != nil {
		return err
	}
	t.listener = l

	go t.acceptLoop(l)

	return nil
}

func (t *ServerTransportTLS) acceptLoop(l net.Listener) {
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
		// Handshake off the accept loop so a slow or silent client cannot
		// delay other clients.
		go t.handshake(conn)
	}
}

func (t *ServerTransportTLS) handshake(raw net.Conn) {
	tlsConn := tls.Server(raw, t.tlsConfig)
	ctx, cancel := context.WithTimeout(context.Background(), handshakeTimeoutOrDefault(t.HandshakeTimeout))
	err := tlsConn.HandshakeContext(ctx)
	cancel()
	if err != nil {
		raw.Close()
		return
	}
	t.deliver(newTCPConnection(tlsConn, t.MaxSendMessageSize, t.MaxRecvMessageSize, t.WriteTimeout))
}

// deliver hands an accepted connection to Accept, closing it if the transport
// is closed or the backlog is full. The lock is held across the (non-blocking)
// send so Close cannot close the channel underneath it.
func (t *ServerTransportTLS) deliver(c *TCPConnection) {
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

func (t *ServerTransportTLS) Accept() (rpc.Connection, error) {
	conn, ok := <-t.connCh
	if !ok {
		return nil, rpc.ErrTransportClosed
	}
	return conn, nil
}

func (t *ServerTransportTLS) Close() error {
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

// ClientTransportTLS implements ClientTransport for TCP with TLS
type ClientTransportTLS struct {
	Host               string
	Port               int
	NoDelay            bool
	InsecureSkipVerify bool
	CAFile             string
	MaxSendMessageSize uint32
	MaxRecvMessageSize uint32
	WriteTimeout       time.Duration
	HandshakeTimeout   time.Duration
}

type ClientTransportTLSConfig struct {
	Host string
	Port int
	// NoDelay is retained for compatibility. TCP_NODELAY is always enabled
	// (Go's default); setting this to false no longer re-enables Nagle.
	NoDelay            bool
	InsecureSkipVerify bool   // Skip certificate verification (for testing)
	CAFile             string // Optional CA certificate file for verification
	MaxSendMessageSize uint32 // Maximum send message size in bytes (0 for no limit)
	MaxRecvMessageSize uint32 // Maximum receive message size in bytes (0 = rpc.DefaultMaxRecvMessageSize)
	// WriteTimeout, if > 0, bounds each frame write.
	WriteTimeout time.Duration
	// HandshakeTimeout bounds the TLS handshake (0 = DefaultTLSHandshakeTimeout).
	// The caller's context deadline, if earlier, also applies.
	HandshakeTimeout time.Duration
}

func NewClientTransportTLS(config ClientTransportTLSConfig) *ClientTransportTLS {
	return &ClientTransportTLS{
		Host:               config.Host,
		Port:               config.Port,
		NoDelay:            config.NoDelay,
		InsecureSkipVerify: config.InsecureSkipVerify,
		CAFile:             config.CAFile,
		MaxSendMessageSize: config.MaxSendMessageSize,
		MaxRecvMessageSize: recvSizeOrDefault(config.MaxRecvMessageSize),
		WriteTimeout:       config.WriteTimeout,
		HandshakeTimeout:   config.HandshakeTimeout,
	}
}

// rootCAs loads the configured CA file. It is read on every connect (connects
// are rare), so a transient read failure is not cached and a rotated CA file
// takes effect on the next reconnect.
func (t *ClientTransportTLS) rootCAs() (*x509.CertPool, error) {
	if t.CAFile == "" {
		return nil, nil
	}
	caCert, err := os.ReadFile(t.CAFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA file: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}
	return pool, nil
}

func (t *ClientTransportTLS) Connect() (rpc.Connection, error) {
	return t.ConnectContext(context.Background())
}

// ConnectContext dials and completes the TLS handshake; ctx bounds both, and
// the handshake is additionally bounded by HandshakeTimeout.
func (t *ClientTransportTLS) ConnectContext(ctx context.Context) (rpc.Connection, error) {
	roots, err := t.rootCAs()
	if err != nil {
		return nil, err
	}
	tlsConfig := &tls.Config{
		InsecureSkipVerify: t.InsecureSkipVerify,
		MinVersion:         tls.VersionTLS12,
		ServerName:         t.Host,
		RootCAs:            roots,
	}

	var d net.Dialer
	raw, err := d.DialContext(ctx, "tcp", net.JoinHostPort(t.Host, strconv.Itoa(t.Port)))
	if err != nil {
		return nil, err
	}
	applyNoDelay(raw)

	hsCtx, cancel := context.WithTimeout(ctx, handshakeTimeoutOrDefault(t.HandshakeTimeout))
	defer cancel()
	conn := tls.Client(raw, tlsConfig)
	if err := conn.HandshakeContext(hsCtx); err != nil {
		raw.Close()
		return nil, err
	}

	return newTCPConnection(conn, t.MaxSendMessageSize, t.MaxRecvMessageSize, t.WriteTimeout), nil
}
