package nats

import (
	"fmt"
	"sync"
	"time"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/nats-io/nats.go"
)

// NATSConnection implements the Connection interface for NATS
type NATSConnection struct {
	nc      *nats.Conn
	sub     *nats.Subscription
	subject string
	inbox   string
	mu      *sync.Mutex
}

func (c *NATSConnection) Send(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.nc == nil {
		return fmt.Errorf("connection is closed")
	}

	return c.nc.Publish(c.subject, data)
}

func (c *NATSConnection) Receive() ([]byte, error) {
	if c.sub == nil {
		return nil, fmt.Errorf("no subscription available")
	}

	msg, err := c.sub.NextMsg(time.Hour * 24 * 365) // effectively no timeout
	if err != nil {
		if err == nats.ErrConnectionClosed || err == nats.ErrBadSubscription {
			return nil, fmt.Errorf("connection closed")
		}
		return nil, err
	}

	// Update the subject to reply to the sender
	c.mu.Lock()
	if msg.Reply != "" {
		c.subject = msg.Reply
	}
	c.mu.Unlock()

	return msg.Data, nil
}

func (c *NATSConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sub != nil {
		if err := c.sub.Unsubscribe(); err != nil {
			return err
		}
		c.sub = nil
	}

	if c.nc != nil {
		c.nc = nil
	}

	return nil
}

// ServerTransport implements ServerTransport for NATS
type ServerTransport struct {
	URL         string
	nc          *nats.Conn
	subs        map[uint64]*nats.Subscription // serviceID -> subscription
	connCh      chan rpc.Connection
	mu          *sync.Mutex
	closed      bool
	serviceInfo map[uint64]string // serviceID -> service name for subject generation
}

type ServerTransportConfig struct {
	URL string // NATS server URL (e.g., "nats://localhost:4222")
}

func NewServerTransport(config ServerTransportConfig) *ServerTransport {
	return &ServerTransport{
		URL:         config.URL,
		subs:        make(map[uint64]*nats.Subscription),
		serviceInfo: make(map[uint64]string),
		connCh:      make(chan rpc.Connection, 16), // buffered channel for connections
		mu:          &sync.Mutex{},
	}
}

// RegisterService registers a service to listen on its own NATS subject
func (t *ServerTransport) RegisterService(serviceID uint64, serviceName string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.nc == nil {
		// Store for later when Listen() is called
		t.serviceInfo[serviceID] = serviceName
		return nil
	}

	// Already listening, subscribe immediately
	return t.subscribeToService(serviceID, serviceName)
}

func (t *ServerTransport) subscribeToService(serviceID uint64, serviceName string) error {
	subject := fmt.Sprintf("rpc.%s", serviceName)

	sub, err := t.nc.Subscribe(subject, func(msg *nats.Msg) {
		// The client sends an initial connection request with their inbox in msg.Reply
		if msg.Reply == "" {
			// No reply address, can't establish connection
			return
		}

		// Create our own inbox for this connection
		serverInbox := t.nc.NewRespInbox()

		// Subscribe to our inbox for receiving messages from this client
		serverSub, err := t.nc.SubscribeSync(serverInbox)
		if err != nil {
			return
		}

		conn := &NATSConnection{
			nc:      t.nc,
			sub:     serverSub,
			subject: msg.Reply, // Client's inbox - this is where we send responses
			inbox:   serverInbox,
			mu:      &sync.Mutex{},
		}

		// Send our inbox back to the client so they know where to send messages
		if err := t.nc.Publish(msg.Reply, []byte(serverInbox)); err != nil {
			serverSub.Unsubscribe()
			return
		}

		t.mu.Lock()
		closed := t.closed
		t.mu.Unlock()

		if !closed {
			select {
			case t.connCh <- conn:
			default:
				// Channel is full, close the connection
				conn.Close()
			}
		} else {
			conn.Close()
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

	// Connect to NATS
	nc, err := nats.Connect(t.URL)
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	t.nc = nc

	// Subscribe to all registered services
	for serviceID, serviceName := range t.serviceInfo {
		if err := t.subscribeToService(serviceID, serviceName); err != nil {
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

	// Unsubscribe from all services
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

// ClientTransport implements ClientTransport for NATS
type ClientTransport struct {
	URL     string
	subject string
}

type ClientTransportConfig struct {
	URL         string // NATS server URL (e.g., "nats://localhost:4222")
	ServiceName string // Service name (e.g., "pingpong") - subject will be "rpc.pingpong"
}

func NewClientTransport(config ClientTransportConfig) *ClientTransport {
	subject := fmt.Sprintf("rpc.%s", config.ServiceName)
	return &ClientTransport{
		URL:     config.URL,
		subject: subject,
	}
}

func (t *ClientTransport) Connect() (rpc.Connection, error) {
	// Connect to NATS
	nc, err := nats.Connect(t.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Create an inbox for receiving messages from the server
	clientInbox := nc.NewRespInbox()

	// Subscribe to our inbox BEFORE sending the connection request
	clientSub, err := nc.SubscribeSync(clientInbox)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to subscribe to inbox: %w", err)
	}

	// Send connection request with our inbox as the reply address
	err = nc.PublishRequest(t.subject, clientInbox, nil)
	if err != nil {
		clientSub.Unsubscribe()
		nc.Close()
		return nil, fmt.Errorf("failed to send connection request: %w", err)
	}

	// Wait for the server to send us its inbox
	msg, err := clientSub.NextMsg(5 * time.Second)
	if err != nil {
		clientSub.Unsubscribe()
		nc.Close()
		return nil, fmt.Errorf("failed to receive server inbox: %w", err)
	}

	// The message contains the server's inbox
	serverInbox := string(msg.Data)

	conn := &NATSConnection{
		nc:      nc,
		sub:     clientSub,
		subject: serverInbox, // Server's inbox - where we send RPC messages
		inbox:   clientInbox,
		mu:      &sync.Mutex{},
	}

	return conn, nil
}
