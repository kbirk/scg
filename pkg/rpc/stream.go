package rpc

import (
	"context"
	"errors"
	"sync"

	"github.com/kbirk/scg/pkg/serialize"
)

// MessageProcessor processes incoming stream messages
type MessageProcessor interface {
	ProcessMessage(methodID uint64, reader *serialize.Reader) (Message, error)
}

// Stream represents a bidirectional stream between client and server
type Stream struct {
	id         uint64
	serviceID  uint64
	ctx        context.Context
	cancel     context.CancelFunc
	mu         *sync.Mutex
	conn       Connection
	requests   map[uint64]chan *serialize.Reader
	requestID  uint64
	processor  MessageProcessor
	closed     bool
	closedChan chan struct{}
	errHandler func(error)
}

// NewStream creates a new stream with the given ID and connection
func NewStream(streamID uint64, serviceID uint64, conn Connection, errHandler func(error)) *Stream {
	ctx, cancel := context.WithCancel(context.Background())

	return &Stream{
		id:         streamID,
		serviceID:  serviceID,
		ctx:        ctx,
		cancel:     cancel,
		mu:         &sync.Mutex{},
		conn:       conn,
		requests:   make(map[uint64]chan *serialize.Reader),
		requestID:  seedRequestID(),
		closed:     false,
		closedChan: make(chan struct{}),
		errHandler: errHandler,
	}
}

// ID returns the stream ID
func (s *Stream) ID() uint64 {
	return s.id
}

// Context returns the stream's context
func (s *Stream) Context() context.Context {
	return s.ctx
}

// SetProcessor sets the message processor for incoming client messages
func (s *Stream) SetProcessor(processor MessageProcessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.processor = processor
}

// IsClosed returns true if the stream is closed
func (s *Stream) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// Wait returns a channel that is closed when the stream is closed
func (s *Stream) Wait() <-chan struct{} {
	return s.closedChan
}

// Close closes the stream
func (s *Stream) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	// Cancel context
	s.cancel()

	// Close wait channel
	close(s.closedChan)

	// Send close message
	writer := serialize.NewFixedSizeWriter(
		serialize.BitsToBytes(
			BitSizePrefix() +
				serialize.BitSizeUInt64(s.id)))

	SerializePrefix(writer, StreamClosePrefix)
	serialize.SerializeUInt64(writer, s.id)

	return s.conn.Send(writer.Bytes(), s.serviceID)
}

// HandleClose handles a close message from the other side
func (s *Stream) HandleClose() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()

	// Cancel context
	s.cancel()

	// Close wait channel
	close(s.closedChan)

	// Fail all pending requests
	s.mu.Lock()
	requests := s.requests
	s.requests = make(map[uint64]chan *serialize.Reader)
	s.mu.Unlock()

	for _, ch := range requests {
		ch <- nil
	}
}

// SendMessage sends a message on the stream and waits for response
func (s *Stream) SendMessage(ctx context.Context, methodID uint64, msg Message) (*serialize.Reader, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, errors.New("stream is closed")
	}

	// Get next request ID
	requestID := s.requestID
	s.requestID++

	// Register request
	ch := make(chan *serialize.Reader)
	s.requests[requestID] = ch
	s.mu.Unlock()

	// Serialize message
	writer := serialize.NewFixedSizeWriter(
		serialize.BitsToBytes(
			BitSizePrefix() +
				BitSizeContext(ctx) +
				serialize.BitSizeUInt64(s.id) +
				serialize.BitSizeUInt64(requestID) +
				serialize.BitSizeUInt64(methodID) +
				msg.BitSize()))

	SerializePrefix(writer, StreamMessagePrefix)
	SerializeContext(writer, ctx)
	serialize.SerializeUInt64(writer, s.id)
	serialize.SerializeUInt64(writer, requestID)
	serialize.SerializeUInt64(writer, methodID)
	msg.Serialize(writer)
	bs := writer.Bytes()

	// Send message
	err := s.conn.Send(bs, s.serviceID)
	if err != nil {
		s.mu.Lock()
		delete(s.requests, requestID)
		s.mu.Unlock()
		return nil, err
	}

	// Wait for response
	select {
	case reader := <-ch:
		if reader == nil {
			return nil, errors.New("stream closed")
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
		s.mu.Lock()
		delete(s.requests, requestID)
		s.mu.Unlock()
		return nil, ctx.Err()
	case <-s.ctx.Done():
		s.mu.Lock()
		delete(s.requests, requestID)
		s.mu.Unlock()
		return nil, errors.New("stream closed")
	}
}

// HandleMessage handles an incoming message response (from SendMessage call)
func (s *Stream) HandleMessageResponse(requestID uint64, reader *serialize.Reader) {
	s.mu.Lock()
	ch, ok := s.requests[requestID]
	delete(s.requests, requestID)
	s.mu.Unlock()

	if ok {
		ch <- reader
	}
}

// HandleIncomingMessage handles an unsolicited incoming message from the peer
func (s *Stream) HandleIncomingMessage(methodID uint64, requestID uint64, reader *serialize.Reader) error {
	s.mu.Lock()
	processor := s.processor
	s.mu.Unlock()

	if processor == nil {
		return errors.New("no message processor registered")
	}

	response, err := processor.ProcessMessage(methodID, reader)
	if err != nil {
		// Send error response
		writer := serialize.NewFixedSizeWriter(
			serialize.BitsToBytes(
				BitSizePrefix() +
					BitSizeContext(s.ctx) +
					serialize.BitSizeUInt64(s.id) +
					serialize.BitSizeUInt64(requestID) +
					serialize.BitSizeUInt8(ErrorResponse) +
					serialize.BitSizeString(err.Error())))

		SerializePrefix(writer, StreamResponsePrefix)
		SerializeContext(writer, s.ctx)
		serialize.SerializeUInt64(writer, s.id)
		serialize.SerializeUInt64(writer, requestID)
		serialize.SerializeUInt8(writer, ErrorResponse)
		serialize.SerializeString(writer, err.Error())

		sendErr := s.conn.Send(writer.Bytes(), s.serviceID)
		if sendErr != nil {
			return sendErr
		}
		return err
	}

	// Send success response with the response message
	writer := serialize.NewFixedSizeWriter(
		serialize.BitsToBytes(
			BitSizePrefix() +
				BitSizeContext(s.ctx) +
				serialize.BitSizeUInt64(s.id) +
				serialize.BitSizeUInt64(requestID) +
				serialize.BitSizeUInt8(MessageResponse) +
				response.BitSize()))

	SerializePrefix(writer, StreamResponsePrefix)
	SerializeContext(writer, s.ctx)
	serialize.SerializeUInt64(writer, s.id)
	serialize.SerializeUInt64(writer, requestID)
	serialize.SerializeUInt8(writer, MessageResponse)
	response.Serialize(writer)

	return s.conn.Send(writer.Bytes(), s.serviceID)
}
