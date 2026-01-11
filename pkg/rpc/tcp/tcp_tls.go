package tcp

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"

	"github.com/kbirk/scg/pkg/rpc"
)

// ServerTransportTLS implements ServerTransport for TCP with TLS
type ServerTransportTLS struct {
	Port               int
	NoDelay            bool
	CertFile           string
	KeyFile            string
	MaxSendMessageSize uint32
	MaxRecvMessageSize uint32
	listener           net.Listener
	connCh             chan rpc.Connection
	mu                 sync.Mutex
	closed             bool
}

type ServerTransportTLSConfig struct {
	Port               int
	NoDelay            bool   // Disable Nagle's algorithm (default: true)
	CertFile           string // Server certificate file (PEM)
	KeyFile            string // Server private key file (PEM)
	MaxSendMessageSize uint32 // Maximum send message size in bytes (0 for no limit)
	MaxRecvMessageSize uint32 // Maximum receive message size in bytes (0 for no limit)
}

func NewServerTransportTLS(config ServerTransportTLSConfig) *ServerTransportTLS {
	return &ServerTransportTLS{
		Port:               config.Port,
		NoDelay:            config.NoDelay,
		CertFile:           config.CertFile,
		KeyFile:            config.KeyFile,
		MaxSendMessageSize: config.MaxSendMessageSize,
		MaxRecvMessageSize: config.MaxRecvMessageSize,
		connCh:             make(chan rpc.Connection, 16),
	}
}

func (t *ServerTransportTLS) Listen() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.listener != nil {
		return fmt.Errorf("transport is already listening")
	}

	cert, err := tls.LoadX509KeyPair(t.CertFile, t.KeyFile)
	if err != nil {
		return fmt.Errorf("failed to load certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	l, err := tls.Listen("tcp", fmt.Sprintf(":%d", t.Port), tlsConfig)
	if err != nil {
		return err
	}
	t.listener = l

	go t.acceptLoop()

	return nil
}

func (t *ServerTransportTLS) acceptLoop() {
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			// Check if closed
			t.mu.Lock()
			if t.closed {
				t.mu.Unlock()
				return
			}
			t.mu.Unlock()
			continue
		}

		// Set TCP_NODELAY option on the underlying TCP connection
		if tlsConn, ok := conn.(*tls.Conn); ok {
			if tcpConn, ok := tlsConn.NetConn().(*net.TCPConn); ok {
				tcpConn.SetNoDelay(t.NoDelay)
			}
		}

		tcpConn := &TCPConnection{
			conn:               conn,
			maxSendMessageSize: t.MaxSendMessageSize,
			maxRecvMessageSize: t.MaxRecvMessageSize,
		}

		t.mu.Lock()
		if !t.closed {
			select {
			case t.connCh <- tcpConn:
			default:
				conn.Close()
			}
		} else {
			conn.Close()
		}
		t.mu.Unlock()
	}
}

func (t *ServerTransportTLS) Accept() (rpc.Connection, error) {
	conn, ok := <-t.connCh
	if !ok {
		return nil, fmt.Errorf("transport is closed")
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
}

type ClientTransportTLSConfig struct {
	Host               string
	Port               int
	NoDelay            bool   // Disable Nagle's algorithm (default: true)
	InsecureSkipVerify bool   // Skip certificate verification (for testing)
	CAFile             string // Optional CA certificate file for verification
	MaxSendMessageSize uint32 // Maximum send message size in bytes (0 for no limit)
	MaxRecvMessageSize uint32 // Maximum receive message size in bytes (0 for no limit)
}

func NewClientTransportTLS(config ClientTransportTLSConfig) *ClientTransportTLS {
	return &ClientTransportTLS{
		Host:               config.Host,
		Port:               config.Port,
		NoDelay:            config.NoDelay,
		InsecureSkipVerify: config.InsecureSkipVerify,
		CAFile:             config.CAFile,
		MaxSendMessageSize: config.MaxSendMessageSize,
		MaxRecvMessageSize: config.MaxRecvMessageSize,
	}
}

func (t *ClientTransportTLS) Connect() (rpc.Connection, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: t.InsecureSkipVerify,
		MinVersion:         tls.VersionTLS12,
	}

	// Load CA certificate if provided
	if t.CAFile != "" {
		caCert, err := os.ReadFile(t.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	conn, err := tls.Dial("tcp", net.JoinHostPort(t.Host, strconv.Itoa(t.Port)), tlsConfig)
	if err != nil {
		return nil, err
	}

	// Set TCP_NODELAY option on the underlying TCP connection
	if tcpConn, ok := conn.NetConn().(*net.TCPConn); ok {
		tcpConn.SetNoDelay(t.NoDelay)
	}

	return &TCPConnection{
		conn:               conn,
		maxSendMessageSize: t.MaxSendMessageSize,
		maxRecvMessageSize: t.MaxRecvMessageSize,
	}, nil
}
