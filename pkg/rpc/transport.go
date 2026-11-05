package rpc

import "context"

// Connection represents a bidirectional communication channel
type Connection interface {
	// Send sends a message to the remote peer
	// serviceID is used by service-aware transports for routing
	Send(data []byte, serviceID uint64) error

	// Receive blocks until a message is received from the remote peer
	Receive() ([]byte, error)

	// Close closes the connection
	Close() error
}

// ServerTransport handles incoming connections for the server
type ServerTransport interface {
	// Listen starts listening for incoming connections
	Listen() error

	// Accept blocks until a new connection is available
	Accept() (Connection, error)

	// Close stops listening and closes the transport
	Close() error
}

// ClientTransport handles outgoing connections for the client
type ClientTransport interface {
	// Connect establishes a connection to the server
	Connect() (Connection, error)
}

// ContextClientTransport is an optional interface for client transports whose
// connection establishment can be bounded by a context. The client uses it when
// available so a call's deadline also bounds the dial (and any TLS/websocket
// handshake); otherwise it falls back to Connect.
type ContextClientTransport interface {
	ClientTransport
	ConnectContext(ctx context.Context) (Connection, error)
}

// ServiceAwareTransport is an optional interface that transports can implement
// to register services by name for service-specific routing
type ServiceAwareTransport interface {
	ServerTransport
	// RegisterService notifies the transport about a service registration
	// This allows the transport to set up service-specific routing
	RegisterService(serviceID uint64, serviceName string) error
}

// ConnectionHandler is called for each new connection on the server
type ConnectionHandler func(Connection)
