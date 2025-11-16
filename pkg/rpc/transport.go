package rpc

// Connection represents a bidirectional communication channel
type Connection interface {
	// Send sends a message to the remote peer
	Send(data []byte) error

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

// ServiceAwareTransport is an optional interface that transports can implement
// to register services by name for service-specific routing (e.g., NATS subjects)
type ServiceAwareTransport interface {
	ServerTransport
	// RegisterService notifies the transport about a service registration
	// This allows the transport to set up service-specific routing (e.g., NATS subjects)
	RegisterService(serviceID uint64, serviceName string) error
}

// ConnectionHandler is called for each new connection on the server
type ConnectionHandler func(Connection)
