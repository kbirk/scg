package test

import (
	"testing"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/rpc/nats"
)

const (
	natsURL = "nats://localhost:4222"
)

// NATSTransportFactory implements TransportFactory for NATS transport
type NATSTransportFactory struct {
	url string
}

func NewNATSTransportFactory(url string) *NATSTransportFactory {
	return &NATSTransportFactory{url: url}
}

func (f *NATSTransportFactory) CreateServerTransport(id int) rpc.ServerTransport {
	return nats.NewServerTransport(nats.ServerTransportConfig{
		URL: f.url,
	})
}

func (f *NATSTransportFactory) CreateClientTransport(id int) rpc.ClientTransport {
	return nats.NewClientTransport(nats.ClientTransportConfig{
		URL: f.url,
	})
}

func (f *NATSTransportFactory) SupportsMultipleServers() bool {
	return true // NATS naturally supports multiple subscribers
}

func (f *NATSTransportFactory) Name() string {
	return "NATS"
}

// TestNATS runs the full test suite for NATS transport
func TestNATS(t *testing.T) {
	RunTestSuite(t, TestSuiteConfig{
		Factory:        NewNATSTransportFactory(natsURL),
		StartingPort:   0,
		SkipGroupTests: true, // NATS doesn't support server groups in the same way
		LargePayloadSizes: []LargePayloadTestCase{
			{"Small 1KB", 1024, true},
			{"Medium 100KB", 100 * 1024, true},
			{"Large 500KB", 500 * 1024, true},
			// NATS has a 1MB default limit
			{"Very Large 2MB", 2 * 1024 * 1024, false},
		},
	})
}
