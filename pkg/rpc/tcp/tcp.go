package tcp

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"

	"github.com/kbirk/scg/pkg/rpc"
)

// setNoDelay sets the TCP_NODELAY option on a TCP connection
func setNoDelay(conn net.Conn, noDelay bool) error {
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		return tcpConn.SetNoDelay(noDelay)
	}
	return nil
}

// TCPConnection implements the Connection interface for TCP
type TCPConnection struct {
	conn               net.Conn
	mu                 sync.Mutex
	maxSendMessageSize uint32
	maxRecvMessageSize uint32
}

func (c *TCPConnection) Send(data []byte, serviceID uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	length := uint32(len(data))

	if c.maxSendMessageSize > 0 && length > c.maxSendMessageSize {
		return fmt.Errorf("message size %d exceeds send limit %d", length, c.maxSendMessageSize)
	}

	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, length)

	if _, err := c.conn.Write(header); err != nil {
		return err
	}
	if _, err := c.conn.Write(data); err != nil {
		return err
	}
	return nil
}

func (c *TCPConnection) Receive() ([]byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(c.conn, header); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("connection closed")
		}
		return nil, err
	}
	length := binary.BigEndian.Uint32(header)

	if c.maxRecvMessageSize > 0 && length > c.maxRecvMessageSize {
		return nil, fmt.Errorf("message size %d exceeds receive limit %d", length, c.maxRecvMessageSize)
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(c.conn, data); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("connection closed")
		}
		return nil, err
	}
	return data, nil
}

func (c *TCPConnection) Close() error {
	return c.conn.Close()
}

// ServerTransport implements ServerTransport for TCP
type ServerTransport struct {
	Port               int
	NoDelay            bool
	MaxSendMessageSize uint32
	MaxRecvMessageSize uint32
	listener           net.Listener
	connCh             chan rpc.Connection
	mu                 sync.Mutex
	closed             bool
}

type ServerTransportConfig struct {
	Port               int
	NoDelay            bool   // Disable Nagle's algorithm for better latency
	MaxSendMessageSize uint32 // Maximum send message size in bytes (0 for no limit)
	MaxRecvMessageSize uint32 // Maximum receive message size in bytes (0 for no limit)
}

func NewServerTransport(config ServerTransportConfig) *ServerTransport {
	return &ServerTransport{
		Port:               config.Port,
		NoDelay:            config.NoDelay,
		MaxSendMessageSize: config.MaxSendMessageSize,
		MaxRecvMessageSize: config.MaxRecvMessageSize,
		connCh:             make(chan rpc.Connection, 16),
	}
}

func (t *ServerTransport) Listen() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.listener != nil {
		return fmt.Errorf("transport is already listening")
	}

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", t.Port))
	if err != nil {
		return err
	}
	t.listener = l

	go t.acceptLoop()

	return nil
}

func (t *ServerTransport) acceptLoop() {
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

		// Set TCP_NODELAY option
		if err := setNoDelay(conn, t.NoDelay); err != nil {
			conn.Close()
			continue
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

func (t *ServerTransport) Accept() (rpc.Connection, error) {
	conn, ok := <-t.connCh
	if !ok {
		return nil, fmt.Errorf("transport is closed")
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
}

type ClientTransportConfig struct {
	Host               string
	Port               int
	NoDelay            bool   // Disable Nagle's algorithm for better latency
	MaxSendMessageSize uint32 // Maximum send message size in bytes (0 for no limit)
	MaxRecvMessageSize uint32 // Maximum receive message size in bytes (0 for no limit)
}

func NewClientTransport(config ClientTransportConfig) *ClientTransport {
	return &ClientTransport{
		Host:               config.Host,
		Port:               config.Port,
		NoDelay:            config.NoDelay,
		MaxSendMessageSize: config.MaxSendMessageSize,
		MaxRecvMessageSize: config.MaxRecvMessageSize,
	}
}

func (t *ClientTransport) Connect() (rpc.Connection, error) {
	conn, err := net.Dial("tcp", net.JoinHostPort(t.Host, strconv.Itoa(t.Port)))
	if err != nil {
		return nil, err
	}

	// Set TCP_NODELAY option
	if err := setNoDelay(conn, t.NoDelay); err != nil {
		conn.Close()
		return nil, err
	}

	return &TCPConnection{
		conn:               conn,
		maxSendMessageSize: t.MaxSendMessageSize,
		maxRecvMessageSize: t.MaxRecvMessageSize,
	}, nil
}
