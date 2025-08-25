package websocket

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/kbirk/scg/pkg/rpc"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// WebSocketConnection implements the Connection interface for WebSocket
type WebSocketConnection struct {
	conn *websocket.Conn
	mu   *sync.Mutex
}

func (c *WebSocketConnection) Send(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteMessage(websocket.BinaryMessage, data)
}

func (c *WebSocketConnection) Receive() ([]byte, error) {
	_, data, err := c.conn.ReadMessage()
	if err != nil {
		// Check if this is a normal close error
		if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			return nil, fmt.Errorf("connection closed")
		}
	}
	return data, err
}

func (c *WebSocketConnection) Close() error {
	return c.conn.Close()
}

// ServerTransport implements ServerTransport for WebSocket
type ServerTransport struct {
	Port     int
	CertFile string
	KeyFile  string
	server   *http.Server
	connCh   chan rpc.Connection
	mu       *sync.Mutex
	closed   bool
}

type ServerTransportConfig struct {
	Port     int
	CertFile string // Optional: for TLS
	KeyFile  string // Optional: for TLS
}

func NewServerTransport(config ServerTransportConfig) *ServerTransport {
	return &ServerTransport{
		Port:     config.Port,
		CertFile: config.CertFile,
		KeyFile:  config.KeyFile,
		connCh:   make(chan rpc.Connection, 16), // buffered channel for connections
		mu:       &sync.Mutex{},
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
		conn: conn,
		mu:   &sync.Mutex{},
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

	t.closed = true
	close(t.connCh)

	if t.server != nil {
		return t.server.Close()
	}
	return nil
}

// ClientTransport implements ClientTransport for WebSocket
type ClientTransport struct {
	Host      string
	Port      int
	TLSConfig *tls.Config
}

type ClientTransportConfig struct {
	Host      string
	Port      int
	TLSConfig *tls.Config
}

func NewClientTransport(config ClientTransportConfig) *ClientTransport {
	return &ClientTransport{
		Host:      config.Host,
		Port:      config.Port,
		TLSConfig: config.TLSConfig,
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
		conn: conn,
		mu:   &sync.Mutex{},
	}, nil
}
