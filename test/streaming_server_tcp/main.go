package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/kbirk/scg/pkg/rpc/tcp"
	streaming "github.com/kbirk/scg/test/files/output/streaming/go"
)

// ChatStreamHandler implements the server-side stream handler
type ChatStreamHandler struct {
	mu               sync.Mutex
	messagesReceived []string
}

func (h *ChatStreamHandler) SendMessage(ctx context.Context, req *streaming.ChatMessage) (*streaming.ChatResponse, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	log.Printf("[Server] Received message: %s", req.Text)
	h.messagesReceived = append(h.messagesReceived, req.Text)

	return &streaming.ChatResponse{
		Status:    "received",
		MessageID: uint64(len(h.messagesReceived)),
	}, nil
}

func (h *ChatStreamHandler) SendNotification(ctx context.Context, req *streaming.ServerNotification) (*streaming.Empty, error) {
	// Not used in basic test
	return &streaming.Empty{}, nil
}

// ChatServiceImpl implements the ChatService
type ChatServiceImpl struct{}

func (s *ChatServiceImpl) OpenChat(ctx context.Context, req *streaming.Empty) (*streaming.ChatStreamStreamServer, error) {
	log.Printf("[Server] Opening chat stream")
	handler := &ChatStreamHandler{
		messagesReceived: make([]string, 0),
	}
	stream := streaming.NewChatStreamStreamServer(handler)

	// Keep stream alive (in real app, would track this for cleanup)
	go func() {
		<-stream.Wait()
		log.Printf("[Server] Stream closed")
	}()

	return stream, nil
}

func main() {
	port := flag.Int("port", 29001, "TCP port to listen on")
	flag.Parse()

	// Create TCP transport
	transport := tcp.NewServerTransport(tcp.ServerTransportConfig{
		Port: *port,
	})

	// Create server
	server := rpc.NewServer(rpc.ServerConfig{
		Transport: transport,
	})

	// Register service
	service := &ChatServiceImpl{}
	streaming.RegisterChatServiceServer(server, service)

	// Start server
	log.Printf("Starting streaming server on port %d...", *port)
	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}
