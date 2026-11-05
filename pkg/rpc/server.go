package rpc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kbirk/scg/pkg/log"
	"github.com/kbirk/scg/pkg/serialize"
)

type Server struct {
	conf      ServerConfig
	transport ServerTransport

	// mu guards configuration (groups, registrations, middleware), the
	// running flag and the live-connection set.
	mu               sync.Mutex
	rootGroup        *ServerGroup
	groupByServiceID map[uint64]*ServerGroup
	activeGroup      *ServerGroup
	running          bool
	conns            map[*serverConn]struct{}

	// services is an immutable snapshot of service id -> {stub, middleware}
	// used on the request hot path without locking. Configuration changes
	// invalidate it (set nil); it is rebuilt on the next lookup.
	services atomic.Pointer[map[uint64]*serviceEntry]

	shuttingDown   atomic.Bool
	unaryHandlers  activity // in-flight unary handlers
	streamHandlers activity // running stream handlers
	connLoops      activity // live connection read loops
}

type serviceEntry struct {
	stub       serverStub
	middleware []Middleware
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
	// MaxConcurrentRequests caps the unary requests running concurrently per
	// connection (0 = unlimited). A request arriving at the cap waits in a
	// per-connection FIFO of the same capacity and starts when a running
	// request finishes; a request arriving when that queue is also full is
	// answered immediately with ErrServerBusy. The connection's read loop never
	// blocks, so stream frames and keepalive PING/PONGs keep flowing.
	MaxConcurrentRequests int
	// KeepaliveInterval, if > 0, enables server-initiated connection keepalive: a
	// PING is sent after this much idle time. KeepaliveTimeout is the max idle
	// time before a connection is declared dead and closed (defaults to
	// 2*KeepaliveInterval; a value not greater than KeepaliveInterval is raised
	// to 2*KeepaliveInterval). This detects a client that vanished without a
	// clean close (e.g. a dropped mobile connection) and prevents leaked handler
	// goroutines and stream buffers.
	KeepaliveInterval time.Duration
	KeepaliveTimeout  time.Duration
}

type serverStub interface {
	HandleWrapper(context.Context, []Middleware, uint64, *serialize.Reader) []byte
}

// activity counts in-flight work and lets a caller wait (bounded by a context)
// for it to reach zero. Unlike sync.WaitGroup it allows new work to be added
// while another goroutine waits.
type activity struct {
	mu   sync.Mutex
	n    int
	zero chan struct{}
}

func (a *activity) add() {
	a.mu.Lock()
	a.n++
	a.mu.Unlock()
}

func (a *activity) done() {
	a.mu.Lock()
	a.n--
	if a.n == 0 && a.zero != nil {
		close(a.zero)
		a.zero = nil
	}
	a.mu.Unlock()
}

func (a *activity) wait(ctx context.Context) error {
	a.mu.Lock()
	if a.n == 0 {
		a.mu.Unlock()
		return nil
	}
	if a.zero == nil {
		a.zero = make(chan struct{})
	}
	z := a.zero
	a.mu.Unlock()
	select {
	case <-z:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// serverConn is the per-connection state.
type serverConn struct {
	conn    Connection
	ctx     context.Context // cancelled when the connection goes away
	cancel  context.CancelFunc
	streams *connStreams

	// Unary admission when MaxConcurrentRequests > 0: at most maxActive
	// requests run; up to maxActive more wait in a FIFO.
	maxActive int
	reqMu     sync.Mutex
	active    int
	queue     []*serialize.Reader

	pongInFlight atomic.Bool
}

func newServerConn(conn Connection, maxRequests int) *serverConn {
	ctx, cancel := context.WithCancel(context.Background())
	return &serverConn{
		conn:      conn,
		ctx:       ctx,
		cancel:    cancel,
		streams:   newConnStreams(),
		maxActive: maxRequests,
	}
}

// close cancels the connection's context and closes the transport connection,
// unblocking its read loop.
func (sc *serverConn) close() {
	sc.cancel()
	sc.conn.Close()
}

// forceClose closes the connection immediately, skipping any graceful close
// handshake the transport would otherwise attempt.
func (sc *serverConn) forceClose() {
	sc.cancel()
	if fc, ok := sc.conn.(ForceCloser); ok {
		fc.ForceClose()
		return
	}
	sc.conn.Close()
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
		conns:            make(map[*serverConn]struct{}),
	}

	return s
}

// Group runs fn with a new child middleware group active: services registered
// and middleware added inside fn belong to the child group, which inherits the
// parent's middleware. Configuration calls are safe to make concurrently with
// serving; changes apply to requests that start afterwards.
func (s *Server) Group(fn func(*Server)) {
	g := newServerGroup()
	s.mu.Lock()
	g.parent = s.activeGroup
	s.activeGroup.children = append(s.activeGroup.children, g)
	s.activeGroup = g
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.activeGroup = g.parent
		s.mu.Unlock()
	}()
	fn(s)
}

func (s *Server) handleError(err error) {
	// Check if this is a normal connection close
	if IsConnectionClosed(err) {
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
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.groupByServiceID[id]
	if ok {
		panic(fmt.Sprintf("service with id %d already registered", id))
	}
	s.activeGroup.registerServer(id, service)
	s.groupByServiceID[id] = s.activeGroup
	s.services.Store(nil)

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

// Middleware adds middleware to the active group. It is safe to call while the
// server is serving; requests that start afterwards run the new middleware.
func (s *Server) Middleware(m Middleware) {
	s.mu.Lock()
	s.activeGroup.middleware = append(s.activeGroup.middleware, m)
	s.services.Store(nil)
	s.mu.Unlock()
}

// lookupService returns the stub and full middleware stack for a service id
// from the lock-free snapshot, rebuilding the snapshot if configuration changed.
func (s *Server) lookupService(id uint64) (*serviceEntry, error) {
	tbl := s.services.Load()
	if tbl == nil {
		tbl = s.rebuildServices()
	}
	entry, ok := (*tbl)[id]
	if !ok {
		return nil, fmt.Errorf("service with id %d not found", id)
	}
	return entry, nil
}

func (s *Server) rebuildServices() *map[uint64]*serviceEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if tbl := s.services.Load(); tbl != nil {
		return tbl
	}
	tbl := make(map[uint64]*serviceEntry, len(s.groupByServiceID))
	for id, group := range s.groupByServiceID {
		// get the lineage from this group to the root
		groups := []*ServerGroup{group}
		for g := group; g.parent != nil; g = g.parent {
			groups = append(groups, g.parent)
		}
		// build from root to leaf
		var middleware []Middleware
		for i := len(groups) - 1; i >= 0; i-- {
			middleware = append(middleware, groups[i].middleware...)
		}
		tbl[id] = &serviceEntry{stub: group.services[id], middleware: middleware}
	}
	s.services.Store(&tbl)
	return &tbl
}

// trackConn registers a live connection; it returns false if the server is
// shutting down (the caller must close the connection).
func (s *Server) trackConn(sc *serverConn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.shuttingDown.Load() {
		return false
	}
	s.conns[sc] = struct{}{}
	s.connLoops.add()
	return true
}

func (s *Server) untrackConn(sc *serverConn) {
	s.mu.Lock()
	delete(s.conns, sc)
	s.mu.Unlock()
	s.connLoops.done()
}

func (s *Server) handleConnection(conn Connection) {
	sc := newServerConn(conn, s.conf.MaxConcurrentRequests)
	if !s.trackConn(sc) {
		sc.close()
		return
	}
	defer s.untrackConn(sc)
	defer sc.close()

	// Fail the connection's live streams on disconnect so handler goroutines
	// blocked in Recv observe the terminal error and return.
	defer sc.streams.terminateAll(ErrConnectionClosed)

	// Server-initiated keepalive detects a client that vanished without a clean
	// close: without it, Receive() below would block forever, leaking this
	// goroutine, its per-stream handlers, and their buffers. When enabled, the
	// server PINGs an idle connection and closes it (unblocking Receive) if no
	// frame arrives within the timeout.
	var lastActivity atomic.Int64
	lastActivity.Store(time.Now().UnixNano())
	if s.conf.KeepaliveInterval > 0 {
		stop := make(chan struct{})
		defer close(stop)
		go s.serverKeepaliveLoop(sc, &lastActivity, stop)
	}

	for {
		// read message
		bs, err := conn.Receive()
		if err != nil {
			s.handleError(err)
			break
		}
		lastActivity.Store(time.Now().UnixNano())

		reader := serialize.NewReader(bs)

		var prefix [16]byte
		if err := DeserializePrefix(&prefix, reader); err != nil {
			s.handleError(err)
			continue
		}

		switch prefix {
		case RequestPrefix:
			// Unary calls run concurrently, one goroutine per request (subject
			// to MaxConcurrentRequests). Admission never blocks this loop.
			s.admitUnary(sc, reader)

		case StreamPrefix:
			// Stream frames are routed inline on the read loop to preserve
			// per-stream ordering; only the handler body runs concurrently.
			s.handleStreamFrame(sc, reader)

		default:
			s.handleError(fmt.Errorf("unexpected prefix: %v", prefix))
		}
	}
}

// serverKeepaliveLoop probes an idle connection with a PING and closes it if no
// frame arrives within the timeout window, mirroring the client keepalive. The
// peer (Go or C++ client) auto-replies PONG, which counts as activity. Closing
// the connection unblocks the read loop in handleConnection, which then tears
// down the connection's streams. The PING is sent asynchronously so a stuck
// write cannot stop this loop from enforcing the timeout. It exits when stop is
// closed (connection handler returned).
func (s *Server) serverKeepaliveLoop(sc *serverConn, lastActivity *atomic.Int64, stop chan struct{}) {
	interval := s.conf.KeepaliveInterval
	timeout := s.conf.KeepaliveTimeout
	if timeout <= interval {
		timeout = 2 * interval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var pingInFlight atomic.Bool
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			idle := time.Since(time.Unix(0, lastActivity.Load()))
			if idle > timeout {
				// Dead peer: close the connection (cancelling its handlers'
				// contexts) to unblock Receive and any stuck write, and trigger
				// stream teardown in handleConnection.
				sc.close()
				return
			}
			// Probe once idle for half an interval; see Client.keepaliveLoop.
			if idle >= interval/2 {
				sendAsync(sc.conn, serializeStreamControl(StreamFramePing), &pingInFlight)
			}
		}
	}
}

// sendResponse writes a unary response. If the response exceeds the
// transport's send limit (nothing was written), a small error response is sent
// instead so the caller does not wait forever. Any other send failure means
// the connection is unusable (it may hold a partial frame), so it is torn
// down; the caller's pending calls then fail rather than hang.
func (s *Server) sendResponse(sc *serverConn, requestID uint64, serviceID uint64, bs []byte) {
	err := sc.conn.Send(bs, serviceID)
	if err == nil {
		return
	}
	s.handleError(err)
	if errors.Is(err, ErrMessageTooLarge) {
		if err2 := sc.conn.Send(RespondWithError(requestID, fmt.Errorf("failed to send response: %w", err)), serviceID); err2 == nil {
			return
		} else {
			s.handleError(err2)
		}
	}
	sc.close()
}

// admitUnary starts, queues, or rejects an inbound unary request without
// blocking the connection's read loop.
func (s *Server) admitUnary(sc *serverConn, reader *serialize.Reader) {
	// Requests that arrive after Shutdown has begun are rejected; requests
	// admitted (running or queued) before it are drained.
	if s.shuttingDown.Load() {
		s.rejectUnary(sc, reader, ErrServerShuttingDown)
		return
	}
	if sc.maxActive <= 0 {
		s.unaryHandlers.add()
		go s.serveUnary(sc, reader)
		return
	}
	sc.reqMu.Lock()
	switch {
	case sc.active < sc.maxActive:
		sc.active++
		sc.reqMu.Unlock()
		s.unaryHandlers.add()
		go s.serveUnary(sc, reader)
	case len(sc.queue) < sc.maxActive:
		sc.queue = append(sc.queue, reader)
		sc.reqMu.Unlock()
		s.unaryHandlers.add() // queued requests are drained by Shutdown too
	default:
		sc.reqMu.Unlock()
		s.rejectUnary(sc, reader, ErrServerBusy)
	}
}

// serveUnary runs a unary request and then, with MaxConcurrentRequests set,
// any queued requests in FIFO order on the same goroutine. It is a plain
// method rather than a closure to avoid a per-request allocation.
func (s *Server) serveUnary(sc *serverConn, reader *serialize.Reader) {
	for {
		s.handleUnaryRequest(sc, reader)
		s.unaryHandlers.done()
		if sc.maxActive <= 0 {
			return
		}
		sc.reqMu.Lock()
		if len(sc.queue) == 0 {
			sc.active--
			sc.reqMu.Unlock()
			return
		}
		reader = sc.queue[0]
		sc.queue[0] = nil
		sc.queue = sc.queue[1:]
		sc.reqMu.Unlock()
	}
}

// rejectUnary answers a request with err without running it. It decodes only
// enough of the frame to address the reply.
func (s *Server) rejectUnary(sc *serverConn, reader *serialize.Reader, reason error) {
	ctx := context.Background()
	if err := DeserializeContext(&ctx, reader); err != nil {
		s.handleError(err)
		return
	}
	var requestID uint64
	if err := serialize.DeserializeUInt64(&requestID, reader); err != nil {
		s.handleError(err)
		return
	}
	var serviceID uint64
	_ = serialize.DeserializeUInt64(&serviceID, reader)
	s.sendResponse(sc, requestID, serviceID, RespondWithError(requestID, reason))
}

// handleUnaryRequest processes a single unary request frame (prefix already
// consumed) and writes the response. Once the request id is known, every
// failure is reported back to the caller as an error response.
func (s *Server) handleUnaryRequest(sc *serverConn, reader *serialize.Reader) {
	// The handler context derives from the connection's context, so it is
	// cancelled if the client disconnects mid-request.
	ctx := sc.ctx
	if err := DeserializeContext(&ctx, reader); err != nil {
		// The request id follows the context on the wire, so there is no way
		// to address a reply.
		s.handleError(err)
		return
	}

	var requestID uint64
	if err := serialize.DeserializeUInt64(&requestID, reader); err != nil {
		s.handleError(err)
		return
	}

	var serviceID uint64
	if err := serialize.DeserializeUInt64(&serviceID, reader); err != nil {
		s.handleError(err)
		s.sendResponse(sc, requestID, 0, RespondWithError(requestID, fmt.Errorf("malformed request: %w", err)))
		return
	}

	entry, err := s.lookupService(serviceID)
	if err != nil {
		s.handleError(err)
		s.sendResponse(sc, requestID, serviceID, RespondWithError(requestID, err))
		return
	}

	bs := entry.stub.HandleWrapper(ctx, entry.middleware, requestID, reader)
	s.sendResponse(sc, requestID, serviceID, bs)
}

// handleStreamFrame routes one inbound stream frame. OPEN spawns a handler
// goroutine; MSG/HALF_CLOSE/CLOSE are delivered to the existing stream.
func (s *Server) handleStreamFrame(sc *serverConn, reader *serialize.Reader) {
	conn := sc.conn
	cs := sc.streams

	var streamID uint64
	if err := serialize.DeserializeUInt64(&streamID, reader); err != nil {
		s.handleError(err)
		return
	}

	var frameKind uint8
	if err := serialize.DeserializeUInt8(&frameKind, reader); err != nil {
		s.handleError(err)
		return
	}

	// Connection-level keepalive frames are not associated with a stream. The
	// PONG is sent asynchronously so a slow client cannot stall the read loop.
	if frameKind == StreamFramePing {
		sendAsync(conn, serializeStreamControl(StreamFramePong), &sc.pongInFlight)
		return
	}
	if frameKind == StreamFramePong {
		return
	}

	switch frameKind {
	case StreamFrameOpen:
		ctx := context.Context(sc.ctx)
		if err := DeserializeContext(&ctx, reader); err != nil {
			s.handleError(err)
			return
		}
		var serviceID uint64
		if err := serialize.DeserializeUInt64(&serviceID, reader); err != nil {
			s.handleError(err)
			return
		}
		var methodID uint64
		if err := serialize.DeserializeUInt64(&methodID, reader); err != nil {
			s.handleError(err)
			return
		}

		if s.shuttingDown.Load() {
			_ = conn.Send(serializeStreamClose(streamID, StreamStatusError, ErrServerShuttingDown.Error()), serviceID)
			return
		}
		// Reject a duplicate stream id rather than orphaning the existing stream.
		if cs.get(streamID) != nil {
			_ = conn.Send(serializeStreamClose(streamID, StreamStatusError, "duplicate stream id"), serviceID)
			return
		}
		// Enforce the per-connection concurrent-stream cap.
		if max := s.conf.MaxConcurrentStreams; max > 0 && cs.count() >= max {
			_ = conn.Send(serializeStreamClose(streamID, StreamStatusError, "max concurrent streams exceeded"), serviceID)
			return
		}

		stream := newServerStream(conn, ctx, streamID, serviceID, s.conf.StreamRecvBufferSize)
		cs.add(streamID, stream)
		s.streamHandlers.add()
		go func() {
			defer s.streamHandlers.done()
			s.runStreamHandler(conn, cs, stream, methodID)
		}()

	case StreamFrameMessage:
		if st := cs.get(streamID); st != nil {
			if st.deliver(reader) {
				// Bounded buffer overflowed: notify the client and drop the stream.
				_ = conn.Send(serializeStreamClose(streamID, StreamStatusError, errStreamOverflow.Error()), st.serviceID)
				cs.remove(streamID)
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
}

// runStreamHandler validates/authorizes the stream and runs its handler to
// completion, then sends the terminal CLOSE frame.
func (s *Server) runStreamHandler(conn Connection, cs *connStreams, stream *ServerStream, methodID uint64) {
	serviceID := stream.serviceID
	defer cs.remove(stream.streamID)
	// Always release the stream context when the handler returns (die() already
	// cancels with a specific cause on client cancel / connection drop; the first
	// cause wins, so this is a no-op there and a clean release on normal exit).
	defer stream.cancel(context.Canceled)

	closeWithError := func(err error) {
		stream.markDone()
		_ = conn.Send(serializeStreamClose(stream.streamID, StreamStatusError, err.Error()), serviceID)
	}

	entry, err := s.lookupService(serviceID)
	if err != nil {
		closeWithError(err)
		return
	}

	streamStub, ok := entry.stub.(streamServerStub)
	if !ok {
		closeWithError(fmt.Errorf("service with id %d does not support streaming", serviceID))
		return
	}

	// Validate/authorize once on OPEN by running the middleware chain with a
	// sentinel request. Message-oriented middleware (e.g. auth) gates the stream.
	if _, mwErr := ApplyHandlerChain(stream.ctx, &emptyStreamMessage{}, entry.middleware,
		func(ctx context.Context, req Message) (Message, error) { return req, nil }); mwErr != nil {
		closeWithError(mwErr)
		return
	}

	if herr := streamStub.HandleStreamWrapper(stream.ctx, stream, methodID); herr != nil {
		closeWithError(herr)
		return
	}

	stream.markDone()
	_ = conn.Send(serializeStreamClose(stream.streamID, StreamStatusOK, ""), serviceID)
}

// ListenAndServe starts the transport and serves connections until Shutdown.
// It returns nil after a Shutdown, or an error if the server is already running,
// was shut down before it started, or the transport fails to listen.
func (s *Server) ListenAndServe() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server is already running")
	}
	if s.shuttingDown.Load() {
		s.mu.Unlock()
		return ErrServerShuttingDown
	}
	s.running = true
	s.mu.Unlock()

	s.logInfo("Starting server")

	err := s.transport.Listen()
	if err != nil {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		if isTransportClosed(err) && s.shuttingDown.Load() {
			return nil
		}
		return err
	}

	for {
		conn, err := s.transport.Accept()
		if err != nil {
			// If the transport is closed (during shutdown), don't treat it as an error
			if isTransportClosed(err) || s.shuttingDown.Load() {
				break
			}
			s.handleError(err)
			continue
		}

		go s.handleConnection(conn)
	}

	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
	return nil
}

// closeConns closes connections concurrently. Graceful closes (which may wait
// to send a close frame to a peer that has stopped reading) are bounded by
// ctx: once it expires, the remaining connections are force-closed and
// closeConns returns without waiting for them.
func closeConns(ctx context.Context, conns []*serverConn) {
	if len(conns) == 0 {
		return
	}
	if ctx.Err() != nil {
		for _, sc := range conns {
			go sc.forceClose()
		}
		return
	}
	var wg sync.WaitGroup
	wg.Add(len(conns))
	for _, sc := range conns {
		go func(sc *serverConn) {
			defer wg.Done()
			sc.close()
		}(sc)
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		for _, sc := range conns {
			go sc.forceClose()
		}
	}
}

// Shutdown gracefully stops the server: it stops accepting connections, rejects
// requests and streams that arrive from now on with ErrServerShuttingDown, and
// waits for in-flight unary handlers to finish and their responses to be sent.
// It then closes every connection — which terminates open streams and cancels
// their handlers' contexts (streams are long-lived and are not drained) — and
// waits for the connection read loops and stream handlers to exit. If ctx
// expires first, the remaining connections are closed immediately and
// ctx.Err() is returned.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.shuttingDown.Store(true)
	s.mu.Unlock()

	terr := s.transport.Close()

	werr := s.unaryHandlers.wait(ctx)

	s.mu.Lock()
	conns := make([]*serverConn, 0, len(s.conns))
	for sc := range s.conns {
		conns = append(conns, sc)
	}
	s.mu.Unlock()
	closeConns(ctx, conns)

	if werr != nil {
		return werr
	}
	if err := s.connLoops.wait(ctx); err != nil {
		return err
	}
	if err := s.streamHandlers.wait(ctx); err != nil {
		return err
	}
	if terr != nil && !isTransportClosed(terr) && !errors.Is(terr, context.Canceled) {
		return terr
	}
	return nil
}
