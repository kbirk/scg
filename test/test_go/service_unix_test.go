package test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/rpc/unix"
)

// UnixTransportFactory implements TransportFactory for Unix socket transport
type UnixTransportFactory struct {
	basePath string
}

func NewUnixTransportFactory(basePath string) *UnixTransportFactory {
	return &UnixTransportFactory{basePath: basePath}
}

func (f *UnixTransportFactory) getSocketPath(id int) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("%s_%d.sock", f.basePath, id))
}

func (f *UnixTransportFactory) CreateServerTransport(id int) rpc.ServerTransport {
	return unix.NewServerTransport(unix.ServerTransportConfig{
		SocketPath: f.getSocketPath(id),
	})
}

func (f *UnixTransportFactory) CreateClientTransport(id int) rpc.ClientTransport {
	return unix.NewClientTransport(unix.ClientTransportConfig{
		SocketPath: f.getSocketPath(id),
	})
}

func (f *UnixTransportFactory) SupportsMultipleServers() bool {
	return false // Unix sockets don't support multiple servers on same path routing
}

func (f *UnixTransportFactory) Name() string {
	return "Unix"
}

// TestUnix runs the full test suite for Unix socket transport
func TestUnix(t *testing.T) {
	RunTestSuite(t, TestSuiteConfig{
		Factory:      NewUnixTransportFactory("scg_test_unix"),
		StartingPort: 0,
		LargePayloadSizes: []LargePayloadTestCase{
			{"Small 1KB", 1024, true},
			{"Medium 100KB", 100 * 1024, true},
			{"Large 500KB", 500 * 1024, true},
			{"Large 1MB", 1024 * 1024, true},
		},
	})
}

// TestUnixExternalServer runs client-only tests against an external Unix socket server (for cross-language testing)
func TestUnixExternalServer(t *testing.T) {
	RunTestSuite(t, TestSuiteConfig{
		Factory:           NewUnixTransportFactory("scg_test_unix"), // Must match C++ server socket path
		StartingPort:      0,
		UseExternalServer: true,
	})
}
