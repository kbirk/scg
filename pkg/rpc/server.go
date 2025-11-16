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
}

type serverStub interface {
	HandleWrapper(context.Context, []Middleware, uint64, *serialize.Reader) []byte
}

func RespondWithError(requestID uint64, err error) []byte {
	writer := serialize.NewFixedSizeWriter(
		serialize.BitsToBytes(
			BitSizePrefix() +
				serialize.BitSizeUInt64(requestID) +
				serialize.BitSizeUInt8(ErrorResponse) +
				serialize.BitSizeString(err.Error())))

	SerializePrefix(writer, ResponsePrefix)
	serialize.SerializeUInt64(writer, requestID)
	serialize.SerializeUInt8(writer, ErrorResponse)
	serialize.SerializeString(writer, err.Error())
	return writer.Bytes()
}

func RespondWithMessage(requestID uint64, msg Message) []byte {
	writer := serialize.NewFixedSizeWriter(
		serialize.BitsToBytes(
			BitSizePrefix() +
				serialize.BitSizeUInt64(requestID) +
				serialize.BitSizeUInt8(MessageResponse) +
				msg.BitSize()))

	SerializePrefix(writer, ResponsePrefix)
	serialize.SerializeUInt64(writer, requestID)
	serialize.SerializeUInt8(writer, MessageResponse)
	msg.Serialize(writer)
	return writer.Bytes()
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

func (g *ServerGroup) registerMiddleware(m Middleware) {
	g.middleware = append(g.middleware, m)
}

func (s *Server) getMiddlewareStackForServiceID(serviceID uint64) ([]Middleware, error) {
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

	return middleware, nil
}

func (s *Server) getServiceByID(id uint64) (serverStub, error) {
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

		go func() {
			reader := serialize.NewReader(bs)

			var prefix [16]byte
			err = DeserializePrefix(&prefix, reader)
			if err != nil {
				s.handleError(err)
				return
			}

			if prefix != RequestPrefix {
				s.handleError(fmt.Errorf("unexpected prefix: %v", prefix))
				return
			}

			// get the context
			ctx := context.Background()
			err = DeserializeContext(&ctx, reader)
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
			bs = service.HandleWrapper(ctx, middleware, requestID, reader)

			// send response
			err = conn.Send(bs)
			if err != nil {
				s.handleError(err)
				return
			}
		}()
	}
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
