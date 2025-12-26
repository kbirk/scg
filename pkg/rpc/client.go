package rpc

import (
	"context"
	"errors"
	"fmt"
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
	requestID uint64
	running   bool
}

type ClientConfig struct {
	Transport  ClientTransport
	ErrHandler func(error)
	middleware []Middleware
	Logger     log.Logger
}

func NewClient(conf ClientConfig) *Client {
	return &Client{
		conf:      conf,
		transport: conf.Transport,
		mu:        &sync.Mutex{},
		requests:  make(map[uint64]chan *serialize.Reader),
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

			if prefix != ResponsePrefix {
				c.handleError(fmt.Errorf("unexpected prefix: %v", prefix))
				return
			}

			var requestID uint64
			err = serialize.DeserializeUInt64(&requestID, reader)
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
	size := int(serialize.BitsToBytes(
		BitSizePrefix() +
			BitSizeContext(ctx) +
			serialize.BitSizeUInt64(requestID) +
			serialize.BitSizeUInt64(serviceID) +
			serialize.BitSizeUInt64(methodID) +
			msg.BitSize()))

	writer := getWriter(size)
	defer putWriter(writer)

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
