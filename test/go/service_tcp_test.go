package test

import (
	"testing"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/rpc/tcp"
)

// TCPTransportFactory implements TransportFactory for TCP transport
type TCPTransportFactory struct {
	basePort int
	useTLS   bool
	certFile string
	keyFile  string
}

func NewTCPTransportFactory(basePort int) *TCPTransportFactory {
	return &TCPTransportFactory{basePort: basePort, useTLS: false}
}

func NewTCPTLSTransportFactory(basePort int, certFile, keyFile string) *TCPTransportFactory {
	return &TCPTransportFactory{
		basePort: basePort,
		useTLS:   true,
		certFile: certFile,
		keyFile:  keyFile,
	}
}

func (f *TCPTransportFactory) CreateServerTransport(id int) rpc.ServerTransport {
	if f.useTLS {
		return tcp.NewServerTransportTLS(tcp.ServerTransportTLSConfig{
			Port:     f.basePort + id,
			NoDelay:  true, // Disable Nagle's algorithm for better latency
			CertFile: f.certFile,
			KeyFile:  f.keyFile,
		})
	}
	return tcp.NewServerTransport(tcp.ServerTransportConfig{
		Port:    f.basePort + id,
		NoDelay: true, // Disable Nagle's algorithm for better latency
	})
}

func (f *TCPTransportFactory) CreateClientTransport(id int) rpc.ClientTransport {
	if f.useTLS {
		return tcp.NewClientTransportTLS(tcp.ClientTransportTLSConfig{
			Host:               "localhost",
			Port:               f.basePort + id,
			NoDelay:            true, // Disable Nagle's algorithm for better latency
			InsecureSkipVerify: true, // self-signed cert
		})
	}
	return tcp.NewClientTransport(tcp.ClientTransportConfig{
		Host:    "localhost",
		Port:    f.basePort + id,
		NoDelay: true, // Disable Nagle's algorithm for better latency
	})
}

func (f *TCPTransportFactory) SupportsMultipleServers() bool {
	return false // TCP doesn't support multiple servers on same port
}

func (f *TCPTransportFactory) Name() string {
	if f.useTLS {
		return "TCP-TLS"
	}
	return "TCP"
}

// TestTCP runs the full test suite for TCP transport
func TestTCP(t *testing.T) {
	RunTestSuite(t, TestSuiteConfig{
		Factory:      NewTCPTransportFactory(9000),
		StartingPort: 0,
		LargePayloadSizes: []LargePayloadTestCase{
			{"Small 1KB", 1024, true},
			{"Medium 100KB", 100 * 1024, true},
			{"Large 500KB", 500 * 1024, true},
			{"Large 1MB", 1024 * 1024, true},
		},
	})
}

// TestTCPTLS runs the full test suite for TCP transport with TLS
func TestTCPTLS(t *testing.T) {
	RunTestSuite(t, TestSuiteConfig{
		Factory:       NewTCPTLSTransportFactory(9100, "../server.crt", "../server.key"),
		StartingPort:  0,
		SkipEdgeTests: true, // Skip edge tests for TLS variant to reduce test time
	})
}

// TestTCPExternalServer runs client-only tests against an external TCP server (for cross-language testing)
func TestTCPExternalServer(t *testing.T) {
	RunTestSuite(t, TestSuiteConfig{
		Factory:           NewTCPTransportFactory(9001), // Must match C++ server port
		StartingPort:      0,
		UseExternalServer: true,
	})
}

// TestTCPTLSExternalServer runs client-only tests against an external TCP TLS server (for cross-language testing)
func TestTCPTLSExternalServer(t *testing.T) {
	RunTestSuite(t, TestSuiteConfig{
		Factory:           NewTCPTLSTransportFactory(9002, "../server.crt", "../server.key"), // Must match C++ server port
		StartingPort:      0,
		UseExternalServer: true,
	})
}
