package rpc

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"

	"github.com/kbirk/scg/pkg/log"
	"github.com/kbirk/scg/pkg/serialize"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Server struct {
	conf             ServerConfig
	server           *http.Server
	groups           []*ServerGroup
	groupByServiceID map[uint64]*ServerGroup
	activeGroup      *ServerGroup
}

type ServerGroup struct {
	services          map[uint64]serverStub
	serviceMiddleware map[uint64][]Middleware
	middleware        []Middleware
}

type ServerConfig struct {
	Port       int
	ErrHandler func(error)
	Logger     log.Logger
}

type serverStub interface {
	HandleWrapper(context.Context, uint64, *serialize.Reader) []byte
}

type Middleware func(context.Context) (context.Context, error)

func RespondWithError(requestID uint64, err error) []byte {
	writer := serialize.NewFixedSizeWriter(ResponseHeaderSize + serialize.ByteSizeString(err.Error()))
	SerializePrefix(writer, ResponsePrefix)
	serialize.SerializeUInt64(writer, requestID)
	serialize.SerializeUInt8(writer, ErrorResponse)
	serialize.SerializeString(writer, err.Error())
	return writer.Bytes()
}

func RespondWithMessage(requestID uint64, msg Message) []byte {
	writer := serialize.NewFixedSizeWriter(ResponseHeaderSize + msg.ByteSize())
	SerializePrefix(writer, ResponsePrefix)
	serialize.SerializeUInt64(writer, requestID)
	serialize.SerializeUInt8(writer, MessageResponse)
	msg.Serialize(writer)
	return writer.Bytes()
}

func newServerGroup() *ServerGroup {
	return &ServerGroup{
		services:          make(map[uint64]serverStub),
		serviceMiddleware: make(map[uint64][]Middleware),
	}
}

func NewServer(conf ServerConfig) *Server {

	defaultGroup := newServerGroup()

	s := &Server{
		conf: conf,
		server: &http.Server{
			Addr: fmt.Sprintf(":%d", conf.Port),
		},
		activeGroup:      defaultGroup,
		groupByServiceID: make(map[uint64]*ServerGroup),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", s.getHandler())
	s.server.Handler = mux

	return s
}

func (s *Server) Group(fn func(*Server)) {
	g := newServerGroup()
	s.activeGroup = g
	s.groups = append(s.groups, g)
	fn(s)
	s.activeGroup = s.groups[0]
}

func (s *Server) handleError(err error) {
	if cErr, ok := err.(*websocket.CloseError); ok {
		s.logInfo("Client disconnected: " + cErr.Text)
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

func (s *Server) RegisterServer(id uint64, service serverStub) {
	_, ok := s.groupByServiceID[id]
	if ok {
		panic(fmt.Sprintf("service with id %d already registered", id))
	}
	s.activeGroup.registerServer(id, service)
	s.groupByServiceID[id] = s.activeGroup
}

func (g *ServerGroup) registerServer(id uint64, service serverStub) {
	if _, ok := g.services[id]; ok {
		panic(fmt.Sprintf("service with id %d already registered", id))
	}
	g.services[id] = service
}

func (s *Server) RegisterMiddleware(m Middleware) {
	s.activeGroup.middleware = append(s.activeGroup.middleware, m)
}

func (g *ServerGroup) registerMiddleware(m Middleware) {
	g.middleware = append(g.middleware, m)
}

func (s *Server) RegisterServerMiddleware(id uint64, m Middleware) {
	s.activeGroup.registerServerMiddleware(id, m)
}

func (g *ServerGroup) registerServerMiddleware(id uint64, m Middleware) {
	middleware, ok := g.serviceMiddleware[id]
	if !ok {
		middleware = make([]Middleware, 0)
	}
	g.serviceMiddleware[id] = append(middleware, m)
}

func (s *Server) applyMiddleware(serviceID uint64, ctx context.Context) (context.Context, error) {

	group, ok := s.groupByServiceID[serviceID]
	if !ok {
		return ctx, nil
	}

	var err error

	for _, m := range group.middleware {
		ctx, err = m(ctx)
		if err != nil {
			return nil, err
		}
	}

	middleware, ok := group.serviceMiddleware[serviceID]
	if ok {
		for _, m := range middleware {
			ctx, err = m(ctx)
			if err != nil {
				return nil, err
			}
		}
	}
	return ctx, nil
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

func (s *Server) getHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			s.conf.ErrHandler(err)
			return
		}
		defer conn.Close()

		for {
			// read message
			_, bs, err := conn.ReadMessage()
			if err != nil {
				s.handleError(err)
				break
			}
			reader := serialize.NewReader(bs)

			var prefix [16]byte
			err = DeserializePrefix(&prefix, reader)
			if err != nil {
				s.handleError(err)
				break
			}

			if prefix != RequestPrefix {
				s.handleError(fmt.Errorf("unexpected prefix: %v", prefix))
				break
			}

			// get the context
			ctx := context.Background()
			err = DeserializeContext(&ctx, reader)
			if err != nil {
				s.handleError(err)
				break
			}

			// get the request id
			var requestID uint64
			err = serialize.DeserializeUInt64(&requestID, reader)
			if err != nil {
				s.handleError(err)
				break
			}

			// get the service id
			var serviceID uint64
			err = serialize.DeserializeUInt64(&serviceID, reader)
			if err != nil {
				s.handleError(err)
				break
			}

			// acquire the service
			service, err := s.getServiceByID(serviceID)
			if err != nil {
				s.handleError(err)
				break
			}

			// apply middleware
			ctx, err = s.applyMiddleware(serviceID, ctx)
			if err != nil {
				bs = RespondWithError(requestID, err)
				err2 := conn.WriteMessage(websocket.BinaryMessage, bs)
				if err2 != nil {
					s.handleError(err2)
				}
				// TODO: should we kill the connection?
				continue
			}

			// handle the request
			bs = service.HandleWrapper(ctx, requestID, reader)

			// send response
			err = conn.WriteMessage(websocket.BinaryMessage, bs)
			if err != nil {
				s.handleError(err)
				break
			}
		}
	}
}

func (s *Server) ListenAndServe() error {
	s.logInfo(fmt.Sprintf("Listening on port %d", s.conf.Port))
	return s.server.ListenAndServe()
}

func (s *Server) ListenAndServeTLS(certFile string, keyFile string) error {
	s.logInfo(fmt.Sprintf("Listening on port %d", s.conf.Port))
	return s.server.ListenAndServeTLS(certFile, keyFile)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
