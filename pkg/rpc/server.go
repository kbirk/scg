package rpc

import (
	"context"
	"fmt"
	"sync"

	"github.com/kbirk/scg/pkg/log"
	"github.com/kbirk/scg/pkg/serialize"
)

type Server struct {
	conf             ServerConfig
	transport        ServerTransport
	rootGroup        *ServerGroup
	groupByServiceID map[uint64]*ServerGroup
	activeGroup      *ServerGroup
	running          bool
	mu               *sync.Mutex
	middlewareCache  map[uint64][]Middleware
}

type ServerGroup struct {
	services   map[uint64]serverStub
	middleware []Middleware
	parent     *ServerGroup
	children   []*ServerGroup
}

type ServerConfig struct {
	Transport  ServerTransport
	ErrHandler func(error)
	Logger     log.Logger
	// StreamRecvBufferSize bounds each stream's inbound queue (0 = default).
	StreamRecvBufferSize int
	// MaxConcurrentStreams caps live streams per connection (0 = unlimited).
	MaxConcurrentStreams int
	// InitialStreamWindow is the per-stream flow-control window in bytes the
	// server grants each client for the client->server direction (0 = default,
	// 1 MiB). It also bounds the per-stream receive buffer, so it must be at
	// least as large as the biggest message a client may send. A client that
	// sends beyond its granted credit overruns the window — a protocol violation
	// that closes the connection.
	InitialStreamWindow uint64
	// InitialConnectionWindow is the connection-wide flow-control window in bytes
	// (0 = default, 4 MiB). Reserved for the connection-level window (phase 3).
	InitialConnectionWindow uint64
}

type serverStub interface {
	HandleWrapper(context.Context, []Middleware, uint64, *serialize.Reader) []byte
}

func RespondWithError(requestID uint64, err error) []byte {
	size := serialize.BitsToBytes(
		BitSizePrefix() +
			serialize.BitSizeUInt64(requestID) +
			serialize.BitSizeUInt8(ErrorResponse) +
			serialize.BitSizeString(err.Error()))

	writer := getWriter(size)
	defer putWriter(writer)

	SerializePrefix(writer, ResponsePrefix)
	serialize.SerializeUInt64(writer, requestID)
	serialize.SerializeUInt8(writer, ErrorResponse)
	serialize.SerializeString(writer, err.Error())

	// Copy bytes since we're returning the writer to the pool
	bs := writer.Bytes()
	result := make([]byte, len(bs))
	copy(result, bs)
	return result
}

func RespondWithMessage(requestID uint64, msg Message) []byte {
	size := serialize.BitsToBytes(
		BitSizePrefix() +
			serialize.BitSizeUInt64(requestID) +
			serialize.BitSizeUInt8(MessageResponse) +
			msg.BitSize())

	writer := getWriter(size)
	defer putWriter(writer)

	SerializePrefix(writer, ResponsePrefix)
	serialize.SerializeUInt64(writer, requestID)
	serialize.SerializeUInt8(writer, MessageResponse)
	msg.Serialize(writer)

	// Copy bytes since we're returning the writer to the pool
	bs := writer.Bytes()
	result := make([]byte, len(bs))
	copy(result, bs)
	return result
}

func newServerGroup() *ServerGroup {
	return &ServerGroup{
		services: make(map[uint64]serverStub),
	}
}

func NewServer(conf ServerConfig) *Server {

	rootGroup := newServerGroup()

	s := &Server{
		conf:             conf,
		transport:        conf.Transport,
		rootGroup:        rootGroup,
		activeGroup:      rootGroup,
		groupByServiceID: make(map[uint64]*ServerGroup),
		middlewareCache:  make(map[uint64][]Middleware),
		mu:               &sync.Mutex{},
	}

	return s
}

func (s *Server) Group(fn func(*Server)) {
	g := newServerGroup()
	g.parent = s.activeGroup
	s.activeGroup.children = append(s.activeGroup.children, g)
	s.activeGroup = g
	fn(s)
	s.activeGroup = g.parent
}

func (s *Server) handleError(err error) {
	// Check if this is a normal connection close
	if err.Error() == "connection closed" {
		s.logInfo("Client disconnected")
		return
	}
	s.logError("Encountered error: " + err.Error())
	if s.conf.ErrHandler != nil {
		s.conf.ErrHandler(err)
	}
	// TODO: respond with an error?!
}

func (s *Server) logDebug(msg string) {
	if s.conf.Logger != nil {
		s.conf.Logger.Debug(msg)
	}
}

func (s *Server) logInfo(msg string) {
	if s.conf.Logger != nil {
		s.conf.Logger.Info(msg)
	}
}

func (s *Server) logWarn(msg string) {
	if s.conf.Logger != nil {
		s.conf.Logger.Warn(msg)
	}
}

func (s *Server) logError(msg string) {
	if s.conf.Logger != nil {
		s.conf.Logger.Error(msg)
	}
}

func (s *Server) RegisterServer(id uint64, serviceName string, service serverStub) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.groupByServiceID[id]
	if ok {
		panic(fmt.Sprintf("service with id %d already registered", id))
	}
	s.activeGroup.registerServer(id, service)
	s.groupByServiceID[id] = s.activeGroup

	// If the transport is service-aware, notify it about the service
	if sat, ok := s.transport.(ServiceAwareTransport); ok {
		if err := sat.RegisterService(id, serviceName); err != nil {
			panic(fmt.Sprintf("failed to register service %s with transport: %v", serviceName, err))
		}
	}
}

func (g *ServerGroup) registerServer(id uint64, service serverStub) {
	if _, ok := g.services[id]; ok {
		panic(fmt.Sprintf("service with id %d already registered", id))
	}
	g.services[id] = service
}

func (s *Server) Middleware(m Middleware) {
	s.activeGroup.middleware = append(s.activeGroup.middleware, m)
}

func (s *Server) getMiddlewareStackForServiceID(serviceID uint64) ([]Middleware, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if stack, ok := s.middlewareCache[serviceID]; ok {
		return stack, nil
	}

	group, ok := s.groupByServiceID[serviceID]
	if !ok {
		return nil, fmt.Errorf("service with id %d not found", serviceID)
	}

	// get the lineage from this group to the root
	groups := []*ServerGroup{group}
	for group.parent != nil {
		groups = append(groups, group.parent)
		group = group.parent
	}

	// build from root to leaf
	var middleware []Middleware
	for i := len(groups) - 1; i >= 0; i-- {
		middleware = append(middleware, groups[i].middleware...)
	}

	s.middlewareCache[serviceID] = middleware
	return middleware, nil
}

func (s *Server) getServiceByID(id uint64) (serverStub, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	group, ok := s.groupByServiceID[id]
	if !ok {
		return nil, fmt.Errorf("service with id %d not found", id)
	}
	return group.getServiceByID(id)
}

func (g *ServerGroup) getServiceByID(id uint64) (serverStub, error) {
	service, ok := g.services[id]
	if !ok {
		return nil, fmt.Errorf("service with id %d not found", id)
	}
	return service, nil
}

func (s *Server) handleConnection(conn Connection) {
	defer conn.Close()

	// Per-connection registry of live streams. Failed on disconnect so handler
	// goroutines blocked in Recv observe the terminal error and return.
	cs := newConnStreams()
	defer cs.terminateAll(fmt.Errorf("connection closed"))

	// Flow control is server-authoritative: dictate the windows up front via
	// SETTINGS, the first frame on the connection, so the client knows its
	// initial send credit before its first message (no startup stall).
	if err := conn.Send(serializeStreamSettings(
		initialStreamWindowOrDefault(s.conf.InitialStreamWindow),
		initialConnectionWindowOrDefault(s.conf.InitialConnectionWindow)), 0); err != nil {
		// Couldn't greet the client; nothing more to do on this connection.
		if err.Error() != "connection closed" {
			s.handleError(err)
		}
		return
	}

	for {
		// read message
		bs, err := conn.Receive()
		if err != nil {
			// Don't treat normal connection closures as errors
			if err.Error() == "connection closed" {
				break
			}
			s.handleError(err)
			break
		}

		reader := serialize.NewReader(bs)

		var prefix [16]byte
		if err := DeserializePrefix(&prefix, reader); err != nil {
			s.handleError(err)
			continue
		}

		switch prefix {
		case RequestPrefix:
			// Unary calls run concurrently, one goroutine per request.
			r := reader
			go s.handleUnaryRequest(conn, r)

		case StreamPrefix:
			// Stream frames are routed inline on the read loop to preserve
			// per-stream ordering; only the handler body runs concurrently. A
			// non-nil return is a fatal protocol violation (e.g. a client that
			// exceeded its flow-control credit) — tear the connection down.
			if ferr := s.handleStreamFrame(conn, cs, reader); ferr != nil {
				s.handleError(ferr)
				return
			}

		default:
			s.handleError(fmt.Errorf("unexpected prefix: %v", prefix))
		}
	}
}

// handleUnaryRequest processes a single unary request frame (prefix already
// consumed) and writes the response.
func (s *Server) handleUnaryRequest(conn Connection, reader *serialize.Reader) {
	// get the context
	ctx := context.Background()
	err := DeserializeContext(&ctx, reader)
	if err != nil {
		s.handleError(err)
		return
	}

	// get the request id
	var requestID uint64
	err = serialize.DeserializeUInt64(&requestID, reader)
	if err != nil {
		s.handleError(err)
		return
	}

	// get the service id
	var serviceID uint64
	err = serialize.DeserializeUInt64(&serviceID, reader)
	if err != nil {
		s.handleError(err)
		return
	}

	// acquire the service
	service, err := s.getServiceByID(serviceID)
	if err != nil {
		s.handleError(err)
		return
	}

	// gather middleware for the call
	middleware, err := s.getMiddlewareStackForServiceID(serviceID)
	if err != nil {
		s.handleError(err)
		return
	}

	// handle the request
	bs := service.HandleWrapper(ctx, middleware, requestID, reader)

	// send response
	err = conn.Send(bs, serviceID)
	if err != nil {
		s.handleError(err)
		return
	}
}

// handleStreamFrame routes one inbound stream frame. OPEN spawns a handler
// goroutine; MSG/HALF_CLOSE/CLOSE are delivered to the existing stream. A
// non-nil return is a fatal protocol violation: the caller closes the whole
// connection. A malformed (but non-abusive) frame is logged and tolerated
// (returns nil) so a single bad frame cannot take the connection down.
func (s *Server) handleStreamFrame(conn Connection, cs *connStreams, reader *serialize.Reader) error {
	frameLen := reader.Len()

	var streamID uint64
	if err := serialize.DeserializeUInt64(&streamID, reader); err != nil {
		s.handleError(err)
		return nil
	}

	var frameKind uint8
	if err := serialize.DeserializeUInt8(&frameKind, reader); err != nil {
		s.handleError(err)
		return nil
	}

	// Connection-level keepalive frames are not associated with a stream.
	if frameKind == StreamFramePing {
		_ = conn.Send(serializeStreamControl(StreamFramePong), 0)
		return nil
	}
	if frameKind == StreamFramePong {
		return nil
	}

	// SETTINGS is server->client only; a client must never send it. Treat as a
	// protocol violation and close the connection.
	if frameKind == StreamFrameSettings {
		return fmt.Errorf("protocol violation: client sent SETTINGS frame")
	}

	// WINDOW_UPDATE from the client replenishes the server's send credit
	// (server->client direction, phase 2). Accepted and ignored until then.
	if frameKind == StreamFrameWindowUpdate {
		var increment uint64
		if err := serialize.DeserializeUInt64(&increment, reader); err != nil {
			s.handleError(err)
		}
		return nil
	}

	switch frameKind {
	case StreamFrameOpen:
		ctx := context.Background()
		if err := DeserializeContext(&ctx, reader); err != nil {
			s.handleError(err)
			return nil
		}
		var serviceID uint64
		if err := serialize.DeserializeUInt64(&serviceID, reader); err != nil {
			s.handleError(err)
			return nil
		}
		var methodID uint64
		if err := serialize.DeserializeUInt64(&methodID, reader); err != nil {
			s.handleError(err)
			return nil
		}

		// Reject a duplicate stream id rather than orphaning the existing stream.
		if cs.get(streamID) != nil {
			_ = conn.Send(serializeStreamClose(streamID, StreamStatusError, "duplicate stream id"), serviceID)
			return nil
		}
		// Enforce the per-connection concurrent-stream cap.
		if max := s.conf.MaxConcurrentStreams; max > 0 && cs.count() >= max {
			_ = conn.Send(serializeStreamClose(streamID, StreamStatusError, "max concurrent streams exceeded"), serviceID)
			return nil
		}

		stream := newServerStream(conn, ctx, streamID, serviceID, s.conf.InitialStreamWindow)
		cs.add(streamID, stream)
		go s.runStreamHandler(conn, cs, stream, methodID)

	case StreamFrameMessage:
		if st := cs.get(streamID); st != nil {
			// The frame's wire length is the flow-control cost; the client sized
			// its send credit identically. An overflow means the client sent more
			// than its granted credit — a protocol violation → close connection.
			if st.deliver(reader, frameLen) {
				return fmt.Errorf("protocol violation: stream %d exceeded flow-control window", streamID)
			}
		}

	case StreamFrameHalfClose:
		if st := cs.get(streamID); st != nil {
			st.halfClose()
		}

	case StreamFrameClose:
		// Client cancelled the stream; surface an error to the handler.
		if st := cs.get(streamID); st != nil {
			st.die(fmt.Errorf("stream cancelled by client"))
			cs.remove(streamID)
		}

	default:
		s.handleError(fmt.Errorf("unknown stream frame kind: %d", frameKind))
	}
	return nil
}

// runStreamHandler validates/authorizes the stream and runs its handler to
// completion, then sends the terminal CLOSE frame.
func (s *Server) runStreamHandler(conn Connection, cs *connStreams, stream *ServerStream, methodID uint64) {
	serviceID := stream.serviceID
	defer cs.remove(stream.streamID)

	closeWithError := func(err error) {
		_ = conn.Send(serializeStreamClose(stream.streamID, StreamStatusError, err.Error()), serviceID)
	}

	service, err := s.getServiceByID(serviceID)
	if err != nil {
		closeWithError(err)
		return
	}

	streamStub, ok := service.(streamServerStub)
	if !ok {
		closeWithError(fmt.Errorf("service with id %d does not support streaming", serviceID))
		return
	}

	middleware, err := s.getMiddlewareStackForServiceID(serviceID)
	if err != nil {
		closeWithError(err)
		return
	}

	// Validate/authorize once on OPEN by running the middleware chain with a
	// sentinel request. Message-oriented middleware (e.g. auth) gates the stream.
	if _, mwErr := ApplyHandlerChain(stream.ctx, &emptyStreamMessage{}, middleware,
		func(ctx context.Context, req Message) (Message, error) { return req, nil }); mwErr != nil {
		closeWithError(mwErr)
		return
	}

	if herr := streamStub.HandleStreamWrapper(stream.ctx, stream, methodID); herr != nil {
		closeWithError(herr)
		return
	}

	_ = conn.Send(serializeStreamClose(stream.streamID, StreamStatusOK, ""), serviceID)
}

func (s *Server) ListenAndServe() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server is already running")
	}
	s.running = true
	s.mu.Unlock()

	s.logInfo("Starting server")

	err := s.transport.Listen()
	if err != nil {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		return err
	}

	for {
		s.mu.Lock()
		running := s.running
		s.mu.Unlock()

		if !running {
			break
		}

		conn, err := s.transport.Accept()
		if err != nil {
			// If the transport is closed (during shutdown), don't treat it as an error
			if err.Error() == "transport is closed" {
				break
			}
			s.handleError(err)
			continue
		}

		go s.handleConnection(conn)
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()

	return s.transport.Close()
}
