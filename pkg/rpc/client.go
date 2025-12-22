package rpc

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"

	"github.com/kbirk/scg/pkg/log"
	"github.com/kbirk/scg/pkg/serialize"
)

type Client struct {
	conf      ClientConfig
	mu        *sync.Mutex
	conn      Connection
	transport ClientTransport
	requests  map[uint64]chan *serialize.Reader
	streams   map[uint64]*Stream
	requestID uint64
	streamID  uint64
	running   bool
}

type ClientConfig struct {
	Transport  ClientTransport
	ErrHandler func(error)
	middleware []Middleware
	Logger     log.Logger
}

func seedRequestID() uint64 {
	return uint64(rand.Uint32())<<32 + uint64(rand.Uint32())
}

func NewClient(conf ClientConfig) *Client {
	return &Client{
		conf:      conf,
		transport: conf.Transport,
		mu:        &sync.Mutex{},
		requestID: seedRequestID(),
		streamID:  seedRequestID(),
		requests:  make(map[uint64]chan *serialize.Reader),
		streams:   make(map[uint64]*Stream),
	}
}

func (c *Client) Middleware(middleware Middleware) {
	c.conf.middleware = append(c.conf.middleware, middleware)
}

func (c *Client) GetMiddleware() []Middleware {
	return c.conf.middleware
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

func (c *Client) handleError(err error) error {
	c.logError("Encountered error: " + err.Error())
	if c.conf.ErrHandler != nil {
		c.conf.ErrHandler(err)
	}

	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	requests := c.requests
	c.requests = make(map[uint64]chan *serialize.Reader)
	c.mu.Unlock()

	// Notify all pending requests of the error
	go func() {
		for _, ch := range requests {
			ch <- nil
		}
	}()

	return err
}

func (c *Client) logDebug(msg string) {
	if c.conf.Logger != nil {
		c.conf.Logger.Debug(msg)
	}
}

func (c *Client) logInfo(msg string) {
	if c.conf.Logger != nil {
		c.conf.Logger.Info(msg)
	}
}

func (c *Client) logWarn(msg string) {
	if c.conf.Logger != nil {
		c.conf.Logger.Warn(msg)
	}
}

func (c *Client) logError(msg string) {
	if c.conf.Logger != nil {
		c.conf.Logger.Error(msg)
	}
}

func (c *Client) connectUnsafe() error {
	if c.conn != nil {
		return nil
	}

	// connect using transport
	c.logDebug("Connecting to server")
	conn, err := c.transport.Connect()
	if err != nil {
		return err
	}
	c.conn = conn

	go func() {
		for {
			c.logDebug("Waiting for message")
			bs, err := conn.Receive()
			if err != nil {
				// Don't treat normal connection closures as errors
				if err.Error() == "connection closed" {
					c.logDebug("Connection closed normally")
					return
				}
				c.handleError(err)
				return
			}
			c.logDebug("Received message")

			reader := serialize.NewReader(bs)

			var prefix [16]byte
			err = DeserializePrefix(&prefix, reader)
			if err != nil {
				c.handleError(err)
				return
			}

			// Handle different message types
			if prefix == ResponsePrefix {
				c.handleRPCResponse(reader)
			} else if prefix == StreamMessagePrefix {
				c.handleStreamMessage(reader)
			} else if prefix == StreamResponsePrefix {
				c.handleStreamResponse(reader)
			} else if prefix == StreamClosePrefix {
				c.handleStreamClose(reader)
			} else {
				c.handleError(fmt.Errorf("unexpected prefix: %v", prefix))
				return
			}
		}
	}()

	return nil
}

func (c *Client) sendMessage(ctx context.Context, serviceID uint64, methodID uint64, msg Message) (chan *serialize.Reader, error) {
	// Get next request ID
	c.mu.Lock()
	requestID := c.requestID
	c.requestID++
	c.mu.Unlock()

	// Serialize message
	writer := serialize.NewFixedSizeWriter(
		int(serialize.BitsToBytes(
			BitSizePrefix() +
				BitSizeContext(ctx) +
				serialize.BitSizeUInt64(requestID) +
				serialize.BitSizeUInt64(serviceID) +
				serialize.BitSizeUInt64(methodID) +
				msg.BitSize())))

	SerializePrefix(writer, RequestPrefix)
	SerializeContext(writer, ctx)
	serialize.SerializeUInt64(writer, requestID)
	serialize.SerializeUInt64(writer, serviceID)
	serialize.SerializeUInt64(writer, methodID)
	msg.Serialize(writer)
	bs := writer.Bytes()

	// Ensure connection and register request
	c.mu.Lock()
	err := c.connectUnsafe()
	if err != nil {
		c.mu.Unlock()
		return nil, err
	}

	ch := make(chan *serialize.Reader)
	c.requests[requestID] = ch
	c.mu.Unlock()

	// Send message
	err = c.conn.Send(bs, serviceID)
	if err != nil {
		c.mu.Lock()
		delete(c.requests, requestID)
		c.mu.Unlock()
		return nil, c.handleError(err)
	}

	return ch, nil
}

func (c *Client) receiveMessage(ctx context.Context, ch chan *serialize.Reader) (*serialize.Reader, error) {
	select {
	case reader := <-ch:
		if reader == nil {
			return nil, errors.New("channel closed")
		}

		var responseType uint8
		serialize.DeserializeUInt8(&responseType, reader)

		if responseType == MessageResponse {
			return reader, nil
		}

		var errMsg string
		serialize.DeserializeString(&errMsg, reader)
		return nil, errors.New(errMsg)
	case <-ctx.Done():
		// Context cancelled or timed out
		return nil, ctx.Err()
	}
}

func (c *Client) Call(ctx context.Context, serviceID uint64, methodID uint64, msg Message) (*serialize.Reader, error) {
	ch, err := c.sendMessage(ctx, serviceID, methodID, msg)
	if err != nil {
		return nil, err
	}

	return c.receiveMessage(ctx, ch)
}

func (c *Client) handleRPCResponse(reader *serialize.Reader) {
	var requestID uint64
	err := serialize.DeserializeUInt64(&requestID, reader)
	if err != nil {
		c.handleError(err)
		return
	}

	c.mu.Lock()
	ch, ok := c.requests[requestID]
	delete(c.requests, requestID)
	c.mu.Unlock()

	if !ok {
		c.handleError(fmt.Errorf("unrecognized request id: %d", requestID))
		return
	}

	ch <- reader
}

// handleStreamMessage handles incoming unsolicited stream messages from server
func (c *Client) handleStreamMessage(reader *serialize.Reader) {
	// get the context
	ctx := context.Background()
	err := DeserializeContext(&ctx, reader)
	if err != nil {
		c.handleError(err)
		return
	}

	var streamID uint64
	err = serialize.DeserializeUInt64(&streamID, reader)
	if err != nil {
		c.handleError(err)
		return
	}

	var requestID uint64
	err = serialize.DeserializeUInt64(&requestID, reader)
	if err != nil {
		c.handleError(err)
		return
	}

	var methodID uint64
	err = serialize.DeserializeUInt64(&methodID, reader)
	if err != nil {
		c.handleError(err)
		return
	}

	c.mu.Lock()
	stream, ok := c.streams[streamID]
	c.mu.Unlock()

	if !ok {
		c.handleError(fmt.Errorf("unrecognized stream id: %d", streamID))
		return
	}

	// Route to stream processor
	err = stream.HandleIncomingMessage(methodID, requestID, reader)
	if err != nil {
		c.handleError(err)
	}
}

func (c *Client) handleStreamResponse(reader *serialize.Reader) {
	// get the context
	ctx := context.Background()
	err := DeserializeContext(&ctx, reader)
	if err != nil {
		c.handleError(err)
		return
	}

	var streamID uint64
	err = serialize.DeserializeUInt64(&streamID, reader)
	if err != nil {
		c.handleError(err)
		return
	}

	var requestID uint64
	err = serialize.DeserializeUInt64(&requestID, reader)
	if err != nil {
		c.handleError(err)
		return
	}

	c.mu.Lock()
	stream, ok := c.streams[streamID]
	c.mu.Unlock()

	if !ok {
		c.handleError(fmt.Errorf("unrecognized stream id: %d", streamID))
		return
	}

	stream.HandleMessageResponse(requestID, reader)
}

func (c *Client) handleStreamClose(reader *serialize.Reader) {
	var streamID uint64
	err := serialize.DeserializeUInt64(&streamID, reader)
	if err != nil {
		c.handleError(err)
		return
	}

	c.mu.Lock()
	stream, ok := c.streams[streamID]
	delete(c.streams, streamID)
	c.mu.Unlock()

	if ok {
		stream.HandleClose()
	}
}

// OpenStream opens a new stream and returns it
func (c *Client) OpenStream(ctx context.Context, serviceID uint64, methodID uint64, msg Message) (*Stream, error) {
	// Get next stream ID and request ID
	c.mu.Lock()
	streamID := c.streamID
	c.streamID++
	requestID := c.requestID
	c.requestID++
	c.mu.Unlock()

	// Serialize stream open message
	writer := serialize.NewFixedSizeWriter(
		int(serialize.BitsToBytes(
			BitSizePrefix() +
				BitSizeContext(ctx) +
				serialize.BitSizeUInt64(requestID) +
				serialize.BitSizeUInt64(streamID) +
				serialize.BitSizeUInt64(serviceID) +
				serialize.BitSizeUInt64(methodID) +
				msg.BitSize())))

	SerializePrefix(writer, StreamOpenPrefix)
	SerializeContext(writer, ctx)
	serialize.SerializeUInt64(writer, requestID)
	serialize.SerializeUInt64(writer, streamID)
	serialize.SerializeUInt64(writer, serviceID)
	serialize.SerializeUInt64(writer, methodID)
	msg.Serialize(writer)
	bs := writer.Bytes()

	// Ensure connection and register request
	c.mu.Lock()
	err := c.connectUnsafe()
	if err != nil {
		c.mu.Unlock()
		return nil, err
	}

	ch := make(chan *serialize.Reader)
	c.requests[requestID] = ch
	c.mu.Unlock()

	// Send stream open message
	err = c.conn.Send(bs, serviceID)
	if err != nil {
		c.mu.Lock()
		delete(c.requests, requestID)
		c.mu.Unlock()
		return nil, c.handleError(err)
	}

	// Wait for response
	_, err = c.receiveMessage(ctx, ch)
	if err != nil {
		return nil, err
	}

	// Create stream with error handler wrapper
	errHandler := func(err error) {
		c.handleError(err)
	}
	stream := NewStream(streamID, serviceID, c.conn, errHandler)

	// Register stream
	c.mu.Lock()
	c.streams[streamID] = stream
	c.mu.Unlock()

	return stream, nil
}
