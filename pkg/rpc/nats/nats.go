package nats

import (
	"fmt"
	"sync"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/nats-io/nats.go"
)

// ServerTransport implements ServerTransport for NATS using request/response pattern
type ServerTransport struct {
	URL                string
	MaxSendMessageSize uint32
	MaxRecvMessageSize uint32
	nc                 *nats.Conn
	subs               map[uint64]*nats.Subscription
	connCh             chan rpc.Connection
	mu                 *sync.Mutex
	closed             bool
	// Track persistent connections by reply inbox for streaming
	activeConns map[string]*natsServerConnection
}

type ServerTransportConfig struct {
	URL                string
	MaxSendMessageSize uint32 // Maximum send message size in bytes (0 for no limit)
	MaxRecvMessageSize uint32 // Maximum receive message size in bytes (0 for no limit)
}

func NewServerTransport(config ServerTransportConfig) *ServerTransport {
	return &ServerTransport{
		URL:                config.URL,
		MaxSendMessageSize: config.MaxSendMessageSize,
		MaxRecvMessageSize: config.MaxRecvMessageSize,
		subs:               make(map[uint64]*nats.Subscription),
		connCh:             make(chan rpc.Connection, 100),
		mu:                 &sync.Mutex{},
		activeConns:        make(map[string]*natsServerConnection),
	}
}

func (t *ServerTransport) RegisterService(serviceID uint64, serviceName string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.nc == nil {
		// Store serviceID for later subscription when Listen() is called
		t.subs[serviceID] = nil
		return nil
	}

	return t.subscribeToService(serviceID)
}

func (t *ServerTransport) subscribeToService(serviceID uint64) error {
	subject := fmt.Sprintf("rpc.%d", serviceID)

	sub, err := t.nc.Subscribe(subject, func(msg *nats.Msg) {
		t.mu.Lock()

		// Check if we already have an active connection for this reply inbox
		var conn *natsServerConnection
		if msg.Reply != "" {
			conn = t.activeConns[msg.Reply]
		}

		if conn == nil {
			// Create new connection
			conn = &natsServerConnection{
				msg:                msg,
				request:            msg.Data,
				maxSendMessageSize: t.MaxSendMessageSize,
				maxRecvMessageSize: t.MaxRecvMessageSize,
				nc:                 t.nc,
				replyTo:            msg.Reply,
				closed:             make(chan struct{}),
				responseCh:         make(chan []byte, 100),
			}

			// Track this connection if it has a reply inbox (for streaming)
			if msg.Reply != "" {
				t.activeConns[msg.Reply] = conn

				// Clean up when connection closes
				go func(inbox string) {
					<-conn.closed
					t.mu.Lock()
					delete(t.activeConns, inbox)
					t.mu.Unlock()
				}(msg.Reply)
			}

			closed := t.closed
			t.mu.Unlock()

			if !closed {
				select {
				case t.connCh <- conn:
				default:
					msg.Respond([]byte("server busy"))
					conn.Close()
				}
			}
		} else {
			// Route to existing connection for streaming
			t.mu.Unlock()

			select {
			case conn.responseCh <- msg.Data:
			case <-conn.closed:
				// Connection closed, drop message
			default:
				// Channel full, drop message
			}
		}
	})

	if err != nil {
		return fmt.Errorf("failed to subscribe to subject %s: %w", subject, err)
	}

	t.subs[serviceID] = sub
	return nil
}

func (t *ServerTransport) Listen() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.nc != nil {
		return fmt.Errorf("transport is already listening")
	}

	nc, err := nats.Connect(t.URL)
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	t.nc = nc

	// Subscribe to all registered services
	for serviceID := range t.subs {
		if err := t.subscribeToService(serviceID); err != nil {
			nc.Close()
			return err
		}
	}

	return nil
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

	// Close all active streaming connections
	for _, conn := range t.activeConns {
		conn.Close()
	}
	t.activeConns = make(map[string]*natsServerConnection)

	for _, sub := range t.subs {
		if err := sub.Unsubscribe(); err != nil {
			return err
		}
	}
	t.subs = make(map[uint64]*nats.Subscription)

	if t.nc != nil {
		t.nc.Close()
		t.nc = nil
	}

	return nil
}

type natsServerConnection struct {
	msg                *nats.Msg
	request            []byte
	mu                 sync.Mutex
	received           bool
	maxSendMessageSize uint32
	maxRecvMessageSize uint32
	// For streaming support
	nc            *nats.Conn
	responseCh    chan []byte
	replyTo       string
	closed        chan struct{}
	alreadyClosed bool
}

func (c *natsServerConnection) Send(data []byte, serviceID uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.maxSendMessageSize > 0 && uint32(len(data)) > c.maxSendMessageSize {
		return fmt.Errorf("message size %d exceeds send limit %d", len(data), c.maxSendMessageSize)
	}

	// For streaming, use the persistent reply subject
	if c.replyTo != "" && c.nc != nil {
		return c.nc.Publish(c.replyTo, data)
	}

	// For regular RPC, use one-time response
	return c.msg.Respond(data)
}

func (c *natsServerConnection) Receive() ([]byte, error) {
	c.mu.Lock()

	// First receive is always the initial request
	if !c.received {
		c.received = true
		request := c.request

		if c.maxRecvMessageSize > 0 && uint32(len(request)) > c.maxRecvMessageSize {
			c.mu.Unlock()
			return nil, fmt.Errorf("message size %d exceeds receive limit %d", len(request), c.maxRecvMessageSize)
		}

		c.mu.Unlock()
		return request, nil
	}

	// For subsequent receives (streaming), wait on response channel
	if c.responseCh == nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("connection closed")
	}

	responseCh := c.responseCh
	closed := c.closed
	c.mu.Unlock()

	select {
	case msg, ok := <-responseCh:
		if !ok {
			return nil, fmt.Errorf("connection closed")
		}

		if c.maxRecvMessageSize > 0 && uint32(len(msg)) > c.maxRecvMessageSize {
			return nil, fmt.Errorf("message size %d exceeds receive limit %d", len(msg), c.maxRecvMessageSize)
		}

		return msg, nil
	case <-closed:
		return nil, fmt.Errorf("connection closed")
	}
}

func (c *natsServerConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.alreadyClosed {
		return nil
	}
	c.alreadyClosed = true

	if c.closed != nil {
		close(c.closed)
	}

	if c.responseCh != nil {
		close(c.responseCh)
	}

	return nil
}

type ClientTransport struct {
	URL                string
	MaxSendMessageSize uint32
	MaxRecvMessageSize uint32
	nc                 *nats.Conn
	mu                 *sync.Mutex
}

type ClientTransportConfig struct {
	URL                string
	MaxSendMessageSize uint32 // Maximum send message size in bytes (0 for no limit)
	MaxRecvMessageSize uint32 // Maximum receive message size in bytes (0 for no limit)
}

func NewClientTransport(config ClientTransportConfig) *ClientTransport {
	return &ClientTransport{
		URL:                config.URL,
		MaxSendMessageSize: config.MaxSendMessageSize,
		MaxRecvMessageSize: config.MaxRecvMessageSize,
		mu:                 &sync.Mutex{},
	}
}

func (t *ClientTransport) Connect() (rpc.Connection, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.nc == nil {
		nc, err := nats.Connect(t.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to NATS: %w", err)
		}
		t.nc = nc
	}

	// Create inbox and subscription for this connection
	inbox := nats.NewInbox()
	responseCh := make(chan *nats.Msg)
	closed := make(chan struct{})

	sub, err := t.nc.Subscribe(inbox, func(msg *nats.Msg) {
		select {
		case responseCh <- msg:
		case <-closed:
			// Connection closed, discard message
		}
	})
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to inbox: %w", err)
	}

	return &natsClientConnection{
		transport:          t,
		inbox:              inbox,
		sub:                sub,
		responseCh:         responseCh,
		closed:             closed,
		maxSendMessageSize: t.MaxSendMessageSize,
		maxRecvMessageSize: t.MaxRecvMessageSize,
	}, nil
}

// natsClientConnection implements Connection for NATS
type natsClientConnection struct {
	transport          *ClientTransport
	inbox              string
	sub                *nats.Subscription
	responseCh         chan *nats.Msg
	closed             chan struct{}
	maxSendMessageSize uint32
	maxRecvMessageSize uint32
}

func (c *natsClientConnection) Send(data []byte, serviceID uint64) error {
	if c.maxSendMessageSize > 0 && uint32(len(data)) > c.maxSendMessageSize {
		return fmt.Errorf("message size %d exceeds send limit %d", len(data), c.maxSendMessageSize)
	}

	subject := fmt.Sprintf("rpc.%d", serviceID)

	msg := &nats.Msg{
		Subject: subject,
		Reply:   c.inbox,
		Data:    data,
	}

	return c.transport.nc.PublishMsg(msg)
}

func (c *natsClientConnection) Receive() ([]byte, error) {
	if c.responseCh == nil {
		return nil, fmt.Errorf("no request sent")
	}

	msg, ok := <-c.responseCh
	if !ok {
		return nil, fmt.Errorf("connection closed")
	}

	if c.maxRecvMessageSize > 0 && uint32(len(msg.Data)) > c.maxRecvMessageSize {
		return nil, fmt.Errorf("message size %d exceeds receive limit %d", len(msg.Data), c.maxRecvMessageSize)
	}

	return msg.Data, nil
}

func (c *natsClientConnection) Close() error {
	// Signal closed first to prevent sends to responseCh
	if c.closed != nil {
		close(c.closed)
	}
	if c.sub != nil {
		c.sub.Unsubscribe()
	}
	if c.responseCh != nil {
		close(c.responseCh)
	}
	return nil
}
