package test

import (
	"crypto/tls"
	"testing"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/rpc/websocket"
)

// WebSocketTransportFactory implements TransportFactory for WebSocket transport
type WebSocketTransportFactory struct {
	basePort int
	useTLS   bool
	certFile string
	keyFile  string
}

func NewWebSocketTransportFactory(basePort int) *WebSocketTransportFactory {
	return &WebSocketTransportFactory{basePort: basePort, useTLS: false}
}

func NewWebSocketTLSTransportFactory(basePort int, certFile, keyFile string) *WebSocketTransportFactory {
	return &WebSocketTransportFactory{
		basePort: basePort,
		useTLS:   true,
		certFile: certFile,
		keyFile:  keyFile,
	}
}

func (f *WebSocketTransportFactory) CreateServerTransport(id int) rpc.ServerTransport {
	config := websocket.ServerTransportConfig{
		Port: f.basePort + id,
	}
	if f.useTLS {
		config.CertFile = f.certFile
		config.KeyFile = f.keyFile
	}
	return websocket.NewServerTransport(config)
}

func (f *WebSocketTransportFactory) CreateClientTransport(id int) rpc.ClientTransport {
	config := websocket.ClientTransportConfig{
		Host: "localhost",
		Port: f.basePort + id,
	}
	if f.useTLS {
		config.TLSConfig = &tls.Config{
			InsecureSkipVerify: true, // self signed
		}
	}
	return websocket.NewClientTransport(config)
}

func (f *WebSocketTransportFactory) SupportsMultipleServers() bool {
	return false // WebSocket doesn't support multiple servers on same port routing
}

func (f *WebSocketTransportFactory) Name() string {
	if f.useTLS {
		return "WebSocket-TLS"
	}
	return "WebSocket"
}

// TestWebSocket runs the full test suite for WebSocket transport
func TestWebSocket(t *testing.T) {
	RunTestSuite(t, TestSuiteConfig{
		Factory:      NewWebSocketTransportFactory(8000),
		StartingPort: 0,
		LargePayloadSizes: []LargePayloadTestCase{
			{"Small 1KB", 1024, true},
			{"Medium 100KB", 100 * 1024, true},
			{"Large 1MB", 1024 * 1024, true},
			{"Very Large 5MB", 5 * 1024 * 1024, true},
			{"Huge 10MB", 10 * 1024 * 1024, true},
		},
	})
}

// TestWebSocketTLS runs the full test suite for WebSocket transport with TLS
func TestWebSocketTLS(t *testing.T) {
	RunTestSuite(t, TestSuiteConfig{
		Factory:       NewWebSocketTLSTransportFactory(8100, "../server.crt", "../server.key"),
		StartingPort:  0,
		SkipEdgeTests: true, // Skip edge tests for TLS variant to reduce test time
	})
}

// TestWebSocketExternalServer runs client-only tests against an external WebSocket server (for cross-language testing)
func TestWebSocketExternalServer(t *testing.T) {
	RunTestSuite(t, TestSuiteConfig{
		Factory:           NewWebSocketTransportFactory(8000), // Must match C++ server port
		StartingPort:      0,
		UseExternalServer: true,
	})
}

// TestWebSocketTLSExternalServer runs client-only tests against an external WebSocket TLS server (for cross-language testing)
func TestWebSocketTLSExternalServer(t *testing.T) {
	RunTestSuite(t, TestSuiteConfig{
		Factory:           NewWebSocketTLSTransportFactory(8000, "../server.crt", "../server.key"), // Must match C++ server port
		StartingPort:      0,
		UseExternalServer: true,
	})
}
