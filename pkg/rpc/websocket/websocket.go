package websocket

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kbirk/scg/pkg/rpc"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// WebSocketConnection implements the Connection interface for WebSocket
type WebSocketConnection struct {
	conn               *websocket.Conn
	mu                 *sync.Mutex
	maxSendMessageSize uint32
	maxRecvMessageSize uint32
}

func (c *WebSocketConnection) Send(data []byte, serviceID uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.maxSendMessageSize > 0 && uint32(len(data)) > c.maxSendMessageSize {
		return fmt.Errorf("message size %d exceeds send limit %d", len(data), c.maxSendMessageSize)
	}

	return c.conn.WriteMessage(websocket.BinaryMessage, data)
}

func (c *WebSocketConnection) Receive() ([]byte, error) {
	_, data, err := c.conn.ReadMessage()
	if err != nil {
		// Check if this is a normal close error
		if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			return nil, fmt.Errorf("connection closed")
		}
		return nil, err
	}

	if c.maxRecvMessageSize > 0 && uint32(len(data)) > c.maxRecvMessageSize {
		return nil, fmt.Errorf("message size %d exceeds receive limit %d", len(data), c.maxRecvMessageSize)
	}

	return data, nil
}

func (c *WebSocketConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Send a proper close frame before closing the connection
	// Use a short deadline to avoid blocking indefinitely
	deadline := time.Now().Add(time.Second)
	err := c.conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		deadline,
	)

	// Close the underlying connection regardless of whether the close frame was sent
	closeErr := c.conn.Close()

	// Return the write error if it occurred, otherwise the close error
	if err != nil {
		return err
	}
	return closeErr
}

// ServerTransport implements ServerTransport for WebSocket
type ServerTransport struct {
	Port               int
	CertFile           string
	KeyFile            string
	MaxSendMessageSize uint32
	MaxRecvMessageSize uint32
	server             *http.Server
	connCh             chan rpc.Connection
	mu                 *sync.Mutex
	closed             bool
}

type ServerTransportConfig struct {
	Port               int
	CertFile           string // Optional: for TLS
	KeyFile            string // Optional: for TLS
	MaxSendMessageSize uint32 // Maximum send message size in bytes (0 for no limit)
	MaxRecvMessageSize uint32 // Maximum receive message size in bytes (0 for no limit)
}

func NewServerTransport(config ServerTransportConfig) *ServerTransport {
	return &ServerTransport{
		Port:               config.Port,
		CertFile:           config.CertFile,
		KeyFile:            config.KeyFile,
		MaxSendMessageSize: config.MaxSendMessageSize,
		MaxRecvMessageSize: config.MaxRecvMessageSize,
		connCh:             make(chan rpc.Connection, 16), // buffered channel for connections
		mu:                 &sync.Mutex{},
	}
}

func (t *ServerTransport) Listen() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.server != nil {
		return fmt.Errorf("transport is already listening")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", t.handleWebSocket)

	t.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", t.Port),
		Handler: mux,
	}

	go func() {
		var err error
		if t.CertFile != "" && t.KeyFile != "" {
			err = t.server.ListenAndServeTLS(t.CertFile, t.KeyFile)
		} else {
			err = t.server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			// Handle error if needed, though we can't return it from this goroutine
		}
	}()

	return nil
}

func (t *ServerTransport) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	wsConn := &WebSocketConnection{
		conn:               conn,
		mu:                 &sync.Mutex{},
		maxSendMessageSize: t.MaxSendMessageSize,
		maxRecvMessageSize: t.MaxRecvMessageSize,
	}

	t.mu.Lock()
	closed := t.closed
	t.mu.Unlock()

	if !closed {
		select {
		case t.connCh <- wsConn:
		default:
			// Channel is full, close the connection
			conn.Close()
		}
	} else {
		conn.Close()
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
}

type ClientTransportConfig struct {
	Host               string
	Port               int
	TLSConfig          *tls.Config
	MaxSendMessageSize uint32 // Maximum send message size in bytes (0 for no limit)
	MaxRecvMessageSize uint32 // Maximum receive message size in bytes (0 for no limit)
}

func NewClientTransport(config ClientTransportConfig) *ClientTransport {
	return &ClientTransport{
		Host:               config.Host,
		Port:               config.Port,
		TLSConfig:          config.TLSConfig,
		MaxSendMessageSize: config.MaxSendMessageSize,
		MaxRecvMessageSize: config.MaxRecvMessageSize,
	}
}

func (t *ClientTransport) Connect() (rpc.Connection, error) {
	scheme := "ws"

	// create dialer
	dialer := websocket.Dialer{}
	if t.TLSConfig != nil {
		// Configure the Dialer to use SSL/TLS
		dialer.TLSClientConfig = t.TLSConfig
		scheme = "wss"
	}

	u := url.URL{Scheme: scheme, Host: fmt.Sprintf("%s:%d", t.Host, t.Port), Path: "/rpc"}

	// connect to the WebSocket server
	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return nil, err
	}

	return &WebSocketConnection{
		conn:               conn,
		mu:                 &sync.Mutex{},
		maxSendMessageSize: t.MaxSendMessageSize,
		maxRecvMessageSize: t.MaxRecvMessageSize,
	}, nil
}
