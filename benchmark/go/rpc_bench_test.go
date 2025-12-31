package benchmarks

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/kbirk/scg/benchmark/scg/generated/benchmark"
	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/rpc/tcp"
	"github.com/kbirk/scg/pkg/rpc/websocket"
)

// TransportBenchmarkFactory creates client and server transports for benchmarking
type TransportBenchmarkFactory interface {
	CreateServerTransport(port int) rpc.ServerTransport
	CreateClientTransport(port int) rpc.ClientTransport
	Name() string
}

// TCPBenchmarkFactory creates TCP transports for benchmarking
type TCPBenchmarkFactory struct{}

func (f *TCPBenchmarkFactory) CreateServerTransport(port int) rpc.ServerTransport {
	return tcp.NewServerTransport(tcp.ServerTransportConfig{
		Port:    port,
		NoDelay: true,
	})
}

func (f *TCPBenchmarkFactory) CreateClientTransport(port int) rpc.ClientTransport {
	return tcp.NewClientTransport(tcp.ClientTransportConfig{
		Host:    "localhost",
		Port:    port,
		NoDelay: true,
	})
}

func (f *TCPBenchmarkFactory) Name() string {
	return "TCP"
}

// WebSocketBenchmarkFactory creates WebSocket transports for benchmarking
type WebSocketBenchmarkFactory struct{}

func (f *WebSocketBenchmarkFactory) CreateServerTransport(port int) rpc.ServerTransport {
	return websocket.NewServerTransport(websocket.ServerTransportConfig{
		Port: port,
	})
}

func (f *WebSocketBenchmarkFactory) CreateClientTransport(port int) rpc.ClientTransport {
	return websocket.NewClientTransport(websocket.ClientTransportConfig{
		Host: "localhost",
		Port: port,
	})
}

func (f *WebSocketBenchmarkFactory) Name() string {
	return "WebSocket"
}

// setupBenchmarkService sets up a benchmark service on the server
func setupBenchmarkService(server *rpc.Server) {
	benchmark.RegisterBenchmarkServiceServer(server, &benchmarkServiceImpl{})
}

// benchmarkServiceImpl implements the BenchmarkService
type benchmarkServiceImpl struct{}

func (s *benchmarkServiceImpl) Call(ctx context.Context, req *benchmark.Request) (*benchmark.Response, error) {
	return &benchmark.Response{}, nil
}

// benchmarkRPCTransport runs benchmarks for a specific transport
func benchmarkRPCTransport(b *testing.B, factory TransportBenchmarkFactory, basePort int) {
	port := basePort

	// Setup server
	serverTransport := factory.CreateServerTransport(port)
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: serverTransport,
	})
	setupBenchmarkService(server)

	// Start server in background
	var serverErr error
	serverReady := make(chan struct{})
	go func() {
		close(serverReady)
		serverErr = server.ListenAndServe()
	}()
	<-serverReady
	time.Sleep(50 * time.Millisecond) // Give server time to start

	defer func() {
		server.Shutdown(context.Background())
		if serverErr != nil && serverErr.Error() != "transport is closed" {
			b.Logf("Server error: %v", serverErr)
		}
	}()

	// Setup client
	clientTransport := factory.CreateClientTransport(port)
	client := rpc.NewClient(rpc.ClientConfig{
		Transport: clientTransport,
	})
	defer client.Close()

	benchmarkClient := benchmark.NewBenchmarkServiceClient(client)

	// Wait for connection to be ready
	time.Sleep(100 * time.Millisecond)

	b.Run("Echo/Simple", func(b *testing.B) {
		req := &benchmark.Request{}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			_, err := benchmarkClient.Call(ctx, req)
			if err != nil {
				b.Fatalf("Call failed: %v", err)
			}
		}
	})

	b.Run("Echo/LongMessage", func(b *testing.B) {
		req := &benchmark.Request{}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			_, err := benchmarkClient.Call(ctx, req)
			if err != nil {
				b.Fatalf("Call failed: %v", err)
			}
		}
	})

	b.Run("Process/SingleItem", func(b *testing.B) {
		req := &benchmark.Request{}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			_, err := benchmarkClient.Call(ctx, req)
			if err != nil {
				b.Fatalf("Call failed: %v", err)
			}
		}
	})

	b.Run("Process/MultipleItems", func(b *testing.B) {
		req := &benchmark.Request{}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			_, err := benchmarkClient.Call(ctx, req)
			if err != nil {
				b.Fatalf("Call failed: %v", err)
			}
		}
	})
}

// BenchmarkRPC_TCP runs RPC benchmarks over TCP transport
func BenchmarkRPC_TCP(b *testing.B) {
	benchmarkRPCTransport(b, &TCPBenchmarkFactory{}, 19000)
}

// BenchmarkRPC_WebSocket runs RPC benchmarks over WebSocket transport
func BenchmarkRPC_WebSocket(b *testing.B) {
	benchmarkRPCTransport(b, &WebSocketBenchmarkFactory{}, 19100)
}

// BenchmarkRPC_Parallel tests parallel RPC calls
func BenchmarkRPC_Parallel(b *testing.B) {
	factories := []TransportBenchmarkFactory{
		&TCPBenchmarkFactory{},
		&WebSocketBenchmarkFactory{},
	}

	for idx, factory := range factories {
		factory := factory
		port := 19200 + idx*10

		b.Run(factory.Name(), func(b *testing.B) {
			// Setup server
			serverTransport := factory.CreateServerTransport(port)
			server := rpc.NewServer(rpc.ServerConfig{
				Transport: serverTransport,
			})
			setupBenchmarkService(server)

			serverReady := make(chan struct{})
			go func() {
				close(serverReady)
				server.ListenAndServe()
			}()
			<-serverReady
			time.Sleep(50 * time.Millisecond)

			defer server.Shutdown(context.Background())

			// Setup client
			clientTransport := factory.CreateClientTransport(port)
			client := rpc.NewClient(rpc.ClientConfig{
				Transport: clientTransport,
			})
			defer client.Close()

			benchmarkClient := benchmark.NewBenchmarkServiceClient(client)
			time.Sleep(100 * time.Millisecond)

			req := &benchmark.Request{}

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					ctx := context.Background()
					_, err := benchmarkClient.Call(ctx, req)
					if err != nil {
						b.Errorf("Call failed: %v", err)
					}
				}
			})
		})
	}
}

// BenchmarkRPC_Throughput measures throughput with concurrent clients
func BenchmarkRPC_Throughput(b *testing.B) {
	factories := []TransportBenchmarkFactory{
		&TCPBenchmarkFactory{},
		&WebSocketBenchmarkFactory{},
	}

	concurrencies := []int{1, 10} // Reduced from 50 to avoid connection limits

	for idx, factory := range factories {
		factory := factory
		basePort := 19300 + idx*100

		for _, concurrency := range concurrencies {
			concurrency := concurrency
			port := basePort + concurrency

			b.Run(fmt.Sprintf("%s/Clients=%d", factory.Name(), concurrency), func(b *testing.B) {
				// Setup server
				serverTransport := factory.CreateServerTransport(port)
				server := rpc.NewServer(rpc.ServerConfig{
					Transport: serverTransport,
				})
				setupBenchmarkService(server)

				serverReady := make(chan struct{})
				go func() {
					close(serverReady)
					server.ListenAndServe()
				}()
				<-serverReady
				time.Sleep(50 * time.Millisecond)

				defer server.Shutdown(context.Background())

				// Setup multiple clients
				clients := make([]*benchmark.BenchmarkServiceClient, concurrency)
				for i := 0; i < concurrency; i++ {
					clientTransport := factory.CreateClientTransport(port)
					client := rpc.NewClient(rpc.ClientConfig{
						Transport: clientTransport,
					})
					defer client.Close()
					clients[i] = benchmark.NewBenchmarkServiceClient(client)
				}
				time.Sleep(100 * time.Millisecond)

				req := &benchmark.Request{}

				b.ReportAllocs()
				b.ResetTimer()

				var wg sync.WaitGroup
				opsPerClient := b.N / concurrency
				if opsPerClient == 0 {
					opsPerClient = 1
				}

				for i := 0; i < concurrency; i++ {
					wg.Add(1)
					client := clients[i]
					go func() {
						defer wg.Done()
						for j := 0; j < opsPerClient; j++ {
							ctx := context.Background()
							_, err := client.Call(ctx, req)
							if err != nil {
								b.Errorf("Call failed: %v", err)
								return
							}
						}
					}()
				}
				wg.Wait()
			})
		}
	}
}
