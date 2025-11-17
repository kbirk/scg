package nats

import (
	"fmt"
	"sync"
	"time"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/nats-io/nats.go"
)

// ServerTransport implements ServerTransport for NATS using request/response pattern
type ServerTransport struct {
	URL    string
	nc     *nats.Conn
	subs   map[uint64]*nats.Subscription
	connCh chan rpc.Connection
	mu     *sync.Mutex
	closed bool
}

type ServerTransportConfig struct {
	URL string
}

func NewServerTransport(config ServerTransportConfig) *ServerTransport {
	return &ServerTransport{
		URL:    config.URL,
		subs:   make(map[uint64]*nats.Subscription),
		connCh: make(chan rpc.Connection, 100),
		mu:     &sync.Mutex{},
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
		conn := &natsServerConnection{
			msg:     msg,
			request: msg.Data,
		}

		t.mu.Lock()
		closed := t.closed
		t.mu.Unlock()

		if !closed {
			select {
			case t.connCh <- conn:
			default:
				msg.Respond([]byte("server busy"))
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
	msg      *nats.Msg
	request  []byte
	mu       sync.Mutex
	received bool
}

func (c *natsServerConnection) Send(data []byte, serviceID uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.msg.Respond(data)
}

func (c *natsServerConnection) Receive() ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.received {
		return nil, fmt.Errorf("connection closed")
	}

	c.received = true
	return c.request, nil
}

func (c *natsServerConnection) Close() error {
	return nil
}

type ClientTransport struct {
	URL     string
	nc      *nats.Conn
	mu      *sync.Mutex
	timeout time.Duration
}

type ClientTransportConfig struct {
	URL     string
	Timeout time.Duration
}

func NewClientTransport(config ClientTransportConfig) *ClientTransport {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &ClientTransport{
		URL:     config.URL,
		mu:      &sync.Mutex{},
		timeout: timeout,
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
	responseCh := make(chan *nats.Msg, 1)

	sub, err := t.nc.ChanSubscribe(inbox, responseCh)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to inbox: %w", err)
	}

	return &natsClientConnection{
		transport:  t,
		inbox:      inbox,
		sub:        sub,
		responseCh: responseCh,
	}, nil
}

// natsClientConnection implements Connection for NATS
type natsClientConnection struct {
	transport  *ClientTransport
	inbox      string
	sub        *nats.Subscription
	responseCh chan *nats.Msg
}

func (c *natsClientConnection) Send(data []byte, serviceID uint64) error {
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

	select {
	case msg := <-c.responseCh:
		return msg.Data, nil
	case <-time.After(c.transport.timeout):
		return nil, fmt.Errorf("NATS request timeout")
	}
}

func (c *natsClientConnection) Close() error {
	if c.sub != nil {
		c.sub.Unsubscribe()
	}
	if c.responseCh != nil {
		close(c.responseCh)
	}
	return nil
}
