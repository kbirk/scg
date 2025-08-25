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
	requestID uint64
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
		requests:  make(map[uint64]chan *serialize.Reader),
	}
}

func (c *Client) Middleware(middleware Middleware) {
	c.conf.middleware = append(c.conf.middleware, middleware)
}

func (c *Client) GetMiddleware() []Middleware {
	return c.conf.middleware
}

func (c *Client) handleError(err error) error {

	c.logError("Encountered error: " + err.Error())
	if c.conf.ErrHandler != nil {
		c.conf.ErrHandler(err)
	}

	c.mu.Lock()

	c.conn.Close()
	c.conn = nil
	requests := c.requests
	c.requests = make(map[uint64]chan *serialize.Reader)

	c.mu.Unlock()

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

	c.mu.Lock()
	requestID := c.requestID
	c.requestID += 1
	c.mu.Unlock()

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

	c.mu.Lock()
	defer c.mu.Unlock()

	err := c.connectUnsafe()
	if err != nil {
		return nil, err
	}

	ch := make(chan *serialize.Reader)
	c.requests[requestID] = ch

	err = c.conn.Send(bs)
	if err != nil {
		delete(c.requests, requestID)
		return nil, c.handleError(err)
	}

	return ch, nil
}

func (c *Client) receiveMessage(ctx context.Context, ch chan *serialize.Reader) (*serialize.Reader, error) {

	// TODO: respect any deadlines / timeouts on the context

	reader := <-ch
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
}

func (c *Client) Call(ctx context.Context, serviceID uint64, methodID uint64, msg Message) (*serialize.Reader, error) {

	ch, err := c.sendMessage(ctx, serviceID, methodID, msg)
	if err != nil {
		return nil, err
	}

	return c.receiveMessage(ctx, ch)
}
