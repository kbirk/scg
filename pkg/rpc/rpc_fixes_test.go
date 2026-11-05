package rpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kbirk/scg/pkg/serialize"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ----------------------------------------------------------------------------
// In-memory connection pair
// ----------------------------------------------------------------------------

// memConn is one end of an in-memory, message-framed connection pair.
type memConn struct {
	in       chan []byte
	peer     *memConn
	closed   chan struct{}
	once     sync.Once
	sendHook func([]byte) error // optional: overrides Send
}

func memPipe() (*memConn, *memConn) {
	a := &memConn{in: make(chan []byte, 1024), closed: make(chan struct{})}
	b := &memConn{in: make(chan []byte, 1024), closed: make(chan struct{})}
	a.peer, b.peer = b, a
	return a, b
}

func (c *memConn) Send(data []byte, serviceID uint64) error {
	if c.sendHook != nil {
		return c.sendHook(data)
	}
	return c.rawSend(data)
}

// rawSend delivers data to the peer, bypassing any sendHook.
func (c *memConn) rawSend(data []byte) error {
	select {
	case <-c.closed:
		return ErrConnectionClosed
	case <-c.peer.closed:
		return ErrConnectionClosed
	default:
	}
	b := append([]byte(nil), data...)
	select {
	case c.peer.in <- b:
		return nil
	case <-c.closed:
		return ErrConnectionClosed
	case <-c.peer.closed:
		return ErrConnectionClosed
	}
}

func (c *memConn) Receive() ([]byte, error) {
	select {
	case b := <-c.in:
		return b, nil
	case <-c.closed:
		return nil, ErrConnectionClosed
	case <-c.peer.closed:
		// Drain anything already delivered before reporting the close.
		select {
		case b := <-c.in:
			return b, nil
		default:
			return nil, ErrConnectionClosed
		}
	}
}

func (c *memConn) Close() error {
	c.once.Do(func() { close(c.closed) })
	return nil
}

func (c *memConn) isClosed() bool {
	select {
	case <-c.closed:
		return true
	default:
		return false
	}
}

// ----------------------------------------------------------------------------
// Test stub service and message
// ----------------------------------------------------------------------------

// rawMsg is a hand-written Message carrying a byte payload.
type rawMsg struct{ data []byte }

func (m *rawMsg) BitSize() int            { return serialize.BitSizeString(string(m.data)) }
func (m *rawMsg) ToJSON() ([]byte, error) { return nil, nil }
func (m *rawMsg) FromJSON([]byte) error   { return nil }
func (m *rawMsg) ToBytes() []byte {
	w := serialize.NewWriter(serialize.BitsToBytes(m.BitSize()))
	m.Serialize(w)
	return w.Bytes()
}
func (m *rawMsg) FromBytes(b []byte) error      { return m.Deserialize(serialize.NewReader(b)) }
func (m *rawMsg) Serialize(w *serialize.Writer) { serialize.SerializeString(w, string(m.data)) }
func (m *rawMsg) Deserialize(r *serialize.Reader) error {
	var s string
	if err := serialize.DeserializeString(&s, r); err != nil {
		return err
	}
	m.data = []byte(s)
	return nil
}

// echoStub echoes the request payload, optionally running hooks first.
type echoStub struct {
	hook        func(ctx context.Context)
	payloadHook func(payload []byte)
}

func (s *echoStub) HandleWrapper(ctx context.Context, mw []Middleware, requestID uint64, r *serialize.Reader) []byte {
	var methodID uint64
	if err := serialize.DeserializeUInt64(&methodID, r); err != nil {
		return RespondWithError(requestID, err)
	}
	req := &rawMsg{}
	if err := req.Deserialize(r); err != nil {
		return RespondWithError(requestID, err)
	}
	resp, err := ApplyHandlerChain(ctx, req, mw, func(ctx context.Context, m Message) (Message, error) {
		if s.payloadHook != nil {
			s.payloadHook(m.(*rawMsg).data)
		}
		if s.hook != nil {
			s.hook(ctx)
		}
		return m, nil
	})
	if err != nil {
		return RespondWithError(requestID, err)
	}
	return RespondWithMessage(requestID, resp)
}

func requestFrame(t testing.TB, ctx context.Context, requestID, serviceID, methodID uint64, payload []byte) []byte {
	msg := &rawMsg{data: payload}
	w := serialize.NewWriter(64)
	SerializePrefix(w, RequestPrefix)
	SerializeContext(w, ctx)
	serialize.SerializeUInt64(w, requestID)
	serialize.SerializeUInt64(w, serviceID)
	serialize.SerializeUInt64(w, methodID)
	msg.Serialize(w)
	return w.Bytes()
}

type decodedResponse struct {
	requestID uint64
	isError   bool
	errMsg    string
	payload   []byte
}

func decodeResponse(t testing.TB, b []byte) decodedResponse {
	t.Helper()
	r := serialize.NewReader(b)
	var prefix [16]byte
	require.NoError(t, DeserializePrefix(&prefix, r))
	require.Equal(t, ResponsePrefix, prefix)
	var d decodedResponse
	require.NoError(t, serialize.DeserializeUInt64(&d.requestID, r))
	var kind uint8
	require.NoError(t, serialize.DeserializeUInt8(&kind, r))
	if kind == ErrorResponse {
		d.isError = true
		require.NoError(t, serialize.DeserializeString(&d.errMsg, r))
		return d
	}
	m := &rawMsg{}
	require.NoError(t, m.Deserialize(r))
	d.payload = m.data
	return d
}

func recvResponse(t testing.TB, c *memConn) decodedResponse {
	t.Helper()
	select {
	case b := <-c.in:
		return decodeResponse(t, b)
	case <-time.After(3 * time.Second):
		t.Fatal("no response received")
		return decodedResponse{}
	}
}

// startServerConn runs a server connection loop over an in-memory pipe and
// returns the client end plus a channel closed when the loop exits.
func startServerConn(s *Server) (*memConn, chan struct{}) {
	clientEnd, serverEnd := memPipe()
	done := make(chan struct{})
	go func() {
		s.handleConnection(serverEnd)
		close(done)
	}()
	return clientEnd, done
}

// ----------------------------------------------------------------------------
// Tests
// ----------------------------------------------------------------------------

func TestAppendMetadataMergesWithoutMutatingParent(t *testing.T) {
	a := NewMetadata()
	a.PutString("a", "1")
	parent := NewContextWithMetadata(context.Background(), a)

	b := NewMetadata()
	b.PutString("b", "2")
	child := AppendMetadataToContext(parent, b)

	md := GetMetadataFromContext(child)
	va, okA, _ := md.GetString("a")
	vb, okB, _ := md.GetString("b")
	assert.True(t, okA, "existing metadata must survive the append")
	assert.True(t, okB)
	assert.Equal(t, "1", va)
	assert.Equal(t, "2", vb)

	_, parentHasB, _ := GetMetadataFromContext(parent).GetString("b")
	assert.False(t, parentHasB, "the parent context's metadata must not be mutated")

	// Appending to a context without metadata.
	plain := AppendMetadataToContext(context.Background(), b)
	_, ok, _ := GetMetadataFromContext(plain).GetString("b")
	assert.True(t, ok)
}

// TestAppendNilMetadataClears: appending nil clears the metadata (the
// behaviour of earlier releases), and the cleared context serializes no
// entries.
func TestAppendNilMetadataClears(t *testing.T) {
	a := NewMetadata()
	a.PutString("a", "1")
	parent := NewContextWithMetadata(context.Background(), a)

	cleared := AppendMetadataToContext(parent, nil)
	assert.Nil(t, GetMetadataFromContext(cleared), "appending nil must clear the metadata")
	assert.NotNil(t, GetMetadataFromContext(parent), "the parent is unaffected")
	assert.Equal(t, serialize.BitSizeUInt32(0), BitSizeContext(cleared))

	w := serialize.NewWriter(8)
	SerializeContext(w, cleared)
	ctx := context.Background()
	require.NoError(t, DeserializeContext(&ctx, serialize.NewReader(w.Bytes())))
	assert.Nil(t, GetMetadataFromContext(ctx))
}

func TestServerRepliesToUnknownService(t *testing.T) {
	s := NewServer(ServerConfig{ErrHandler: func(error) {}})
	s.RegisterServer(1, "echo", &echoStub{})
	client, done := startServerConn(s)
	defer func() { client.Close(); <-done }()

	require.NoError(t, client.Send(requestFrame(t, context.Background(), 77, 999, 1, []byte("x")), 999))
	resp := recvResponse(t, client)
	assert.Equal(t, uint64(77), resp.requestID)
	assert.True(t, resp.isError)
	assert.Contains(t, resp.errMsg, "service with id 999 not found")
}

func TestServerRepliesWhenResponseCannotBeSent(t *testing.T) {
	s := NewServer(ServerConfig{ErrHandler: func(error) {}})
	s.RegisterServer(1, "echo", &echoStub{})
	clientEnd, serverEnd := memPipe()
	// The server's sends of frames over 100 bytes fail with a size error.
	serverEnd.sendHook = func(b []byte) error {
		if len(b) > 100 {
			return NewMessageSizeError(uint64(len(b)), 100, "send")
		}
		return serverEnd.rawSend(b)
	}
	done := make(chan struct{})
	go func() { s.handleConnection(serverEnd); close(done) }()
	defer func() { clientEnd.Close(); <-done }()

	require.NoError(t, clientEnd.Send(requestFrame(t, context.Background(), 5, 1, 1, make([]byte, 500)), 1))
	resp := recvResponse(t, clientEnd)
	assert.Equal(t, uint64(5), resp.requestID)
	assert.True(t, resp.isError)
	assert.Contains(t, resp.errMsg, "exceeds send limit")
}

func TestMessageSizeErrorMatchesSentinel(t *testing.T) {
	err := NewMessageSizeError(10, 5, "send")
	assert.True(t, errors.Is(err, ErrMessageTooLarge))
	assert.Equal(t, "message size 10 exceeds send limit 5", err.Error())
	assert.True(t, IsConnectionClosed(ErrConnectionClosed))
	assert.True(t, IsConnectionClosed(fmt.Errorf("wrapped: %w", ErrConnectionClosed)))
	assert.True(t, IsConnectionClosed(errors.New("connection closed")), "legacy transports compare by text")
	assert.False(t, IsConnectionClosed(errors.New("boom")))
}

func TestServerRejectsRequestsDuringShutdown(t *testing.T) {
	release := make(chan struct{})
	s := NewServer(ServerConfig{Transport: &nopServerTransport{}, ErrHandler: func(error) {}})
	s.RegisterServer(1, "echo", &echoStub{hook: func(context.Context) { <-release }})
	client, done := startServerConn(s)

	// One request in flight, blocking in its handler.
	require.NoError(t, client.Send(requestFrame(t, context.Background(), 1, 1, 1, []byte("slow")), 1))
	time.Sleep(50 * time.Millisecond)

	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- s.Shutdown(context.Background()) }()
	time.Sleep(50 * time.Millisecond)

	// A new request arriving during the drain is rejected, not served.
	require.NoError(t, client.Send(requestFrame(t, context.Background(), 2, 1, 1, []byte("late")), 1))
	resp := recvResponse(t, client)
	assert.Equal(t, uint64(2), resp.requestID)
	assert.True(t, resp.isError)
	assert.Equal(t, ErrServerShuttingDown.Error(), resp.errMsg)

	select {
	case <-shutdownDone:
		t.Fatal("Shutdown returned before the in-flight handler finished")
	default:
	}
	close(release)
	resp = recvResponse(t, client)
	assert.Equal(t, uint64(1), resp.requestID)
	assert.False(t, resp.isError)
	assert.Equal(t, []byte("slow"), resp.payload)

	require.NoError(t, <-shutdownDone)
	<-done
	assert.True(t, client.peer.isClosed(), "Shutdown must close the connection")

	assert.ErrorIs(t, s.ListenAndServe(), ErrServerShuttingDown, "a shut-down server cannot be restarted")
}

func TestShutdownTimesOut(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	s := NewServer(ServerConfig{Transport: &nopServerTransport{}, ErrHandler: func(error) {}})
	s.RegisterServer(1, "echo", &echoStub{hook: func(context.Context) { <-release }})
	client, done := startServerConn(s)
	require.NoError(t, client.Send(requestFrame(t, context.Background(), 1, 1, 1, nil), 1))
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	assert.ErrorIs(t, s.Shutdown(ctx), context.DeadlineExceeded)
	<-done // connections are force-closed on timeout
}

type nopServerTransport struct{}

func (nopServerTransport) Listen() error               { return ErrTransportClosed }
func (nopServerTransport) Accept() (Connection, error) { return nil, ErrTransportClosed }
func (nopServerTransport) Close() error                { return nil }

func TestMaxConcurrentRequestsBoundsHandlers(t *testing.T) {
	const limit = 3
	var active, peak atomic.Int32
	release := make(chan struct{})
	s := NewServer(ServerConfig{MaxConcurrentRequests: limit})
	s.RegisterServer(1, "echo", &echoStub{hook: func(context.Context) {
		n := active.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		<-release
		active.Add(-1)
	}})
	client, done := startServerConn(s)
	defer func() { client.Close(); <-done }()

	// limit run, limit queue, the rest are rejected immediately as busy.
	const total = 10
	for i := 0; i < total; i++ {
		require.NoError(t, client.Send(requestFrame(t, context.Background(), uint64(i), 1, 1, nil), 1))
	}
	busy := 0
	for i := 0; i < total-2*limit; i++ {
		resp := recvResponse(t, client)
		require.True(t, resp.isError)
		assert.Equal(t, ErrServerBusy.Error(), resp.errMsg)
		busy++
	}
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(limit), peak.Load(), "no more than MaxConcurrentRequests handlers may run at once")
	close(release)
	for i := 0; i < 2*limit; i++ {
		resp := recvResponse(t, client)
		assert.False(t, resp.isError, "admitted requests (running or queued) must be served: %s", resp.errMsg)
	}
	assert.Equal(t, total-2*limit, busy)
	assert.Equal(t, int32(limit), peak.Load())
}

// TestMaxConcurrentRequestsQueueIsFIFO: queued requests start in arrival order.
func TestMaxConcurrentRequestsQueueIsFIFO(t *testing.T) {
	started := make(chan string, 16)
	releases := map[string]chan struct{}{}
	for _, k := range []string{"r1", "r2", "r3", "r4"} {
		releases[k] = make(chan struct{})
	}
	s := NewServer(ServerConfig{MaxConcurrentRequests: 2})
	s.RegisterServer(1, "echo", &echoStub{payloadHook: func(p []byte) {
		started <- string(p)
		<-releases[string(p)]
	}})
	client, done := startServerConn(s)
	defer func() { client.Close(); <-done }()

	for i, k := range []string{"r1", "r2", "r3", "r4"} {
		require.NoError(t, client.Send(requestFrame(t, context.Background(), uint64(i), 1, 1, []byte(k)), 1))
	}
	first := map[string]bool{<-started: true, <-started: true}
	require.True(t, first["r1"] && first["r2"], "the first two requests run immediately")

	close(releases["r1"])
	select {
	case next := <-started:
		assert.Equal(t, "r3", next, "the queue must be FIFO")
	case <-time.After(2 * time.Second):
		t.Fatal("queued request did not start when a slot freed")
	}
	close(releases["r2"])
	assert.Equal(t, "r4", <-started)
	close(releases["r3"])
	close(releases["r4"])
	for i := 0; i < 4; i++ {
		assert.False(t, recvResponse(t, client).isError)
	}
}

// pipeTransport connects a Client to a Server over in-memory pipes.
type pipeTransport struct {
	server *Server
	dials  atomic.Int32
}

func (p *pipeTransport) Connect() (Connection, error) {
	p.dials.Add(1)
	clientEnd, serverEnd := memPipe()
	go p.server.handleConnection(serverEnd)
	return clientEnd, nil
}

// TestMaxConcurrentRequestsKeepsKeepaliveFlowing: with the cap reached, the
// read loop must keep reading so PING/PONGs flow; neither side's keepalive may
// kill a healthy connection, and both responses arrive.
func TestMaxConcurrentRequestsKeepsKeepaliveFlowing(t *testing.T) {
	s := NewServer(ServerConfig{
		MaxConcurrentRequests: 1,
		KeepaliveInterval:     30 * time.Millisecond,
		KeepaliveTimeout:      90 * time.Millisecond,
	})
	s.RegisterServer(1, "echo", &echoStub{hook: func(context.Context) { time.Sleep(400 * time.Millisecond) }})
	tr := &pipeTransport{server: s}
	var clientErrs atomic.Int32
	c := NewClient(ClientConfig{
		Transport:         tr,
		KeepaliveInterval: 30 * time.Millisecond,
		KeepaliveTimeout:  90 * time.Millisecond,
		ErrHandler:        func(error) { clientErrs.Add(1) },
	})
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	results := make([]string, 2)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = echoCall(c, ctx, fmt.Sprintf("req%d", i))
		}(i)
	}
	wg.Wait()
	for i := 0; i < 2; i++ {
		require.NoError(t, errs[i])
		assert.Equal(t, fmt.Sprintf("req%d", i), results[i])
	}
	assert.Zero(t, clientErrs.Load(), "the keepalive must not declare the busy connection dead")
	assert.Equal(t, int32(1), tr.dials.Load(), "the connection must stay up (no reconnect)")
}

// TestSendFailureTearsDownConnection: a response that fails to send for any
// reason other than the size limit leaves the stream unusable, so the server
// closes the connection rather than writing a fallback frame onto it.
func TestSendFailureTearsDownConnection(t *testing.T) {
	s := NewServer(ServerConfig{ErrHandler: func(error) {}})
	s.RegisterServer(1, "echo", &echoStub{})
	clientEnd, serverEnd := memPipe()
	var sends atomic.Int32
	serverEnd.sendHook = func([]byte) error {
		sends.Add(1)
		return errors.New("write: broken pipe")
	}
	done := make(chan struct{})
	go func() { s.handleConnection(serverEnd); close(done) }()

	require.NoError(t, clientEnd.Send(requestFrame(t, context.Background(), 1, 1, 1, []byte("x")), 1))
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the server kept a connection whose write failed")
	}
	assert.Equal(t, int32(1), sends.Load(), "no fallback frame may be written to a broken stream")
}

func TestUnaryHandlerContextCancelledOnDisconnect(t *testing.T) {
	cancelled := make(chan struct{})
	s := NewServer(ServerConfig{})
	s.RegisterServer(1, "echo", &echoStub{hook: func(ctx context.Context) {
		select {
		case <-ctx.Done():
			close(cancelled)
		case <-time.After(3 * time.Second):
		}
	}})
	client, done := startServerConn(s)
	require.NoError(t, client.Send(requestFrame(t, context.Background(), 1, 1, 1, nil), 1))
	time.Sleep(50 * time.Millisecond)
	client.Close()
	<-done
	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("handler context not cancelled after disconnect")
	}
}

// TestServerKeepaliveClosesConnectionWithStuckWriter: a write that never
// completes (peer not reading) must not stop the keepalive from closing the
// dead connection.
func TestServerKeepaliveClosesConnectionWithStuckWriter(t *testing.T) {
	s := NewServer(ServerConfig{KeepaliveInterval: 30 * time.Millisecond, KeepaliveTimeout: 90 * time.Millisecond})
	clientEnd, serverEnd := memPipe()
	stuck := make(chan struct{})
	serverEnd.sendHook = func([]byte) error {
		select {
		case <-stuck: // blocks until the connection is closed
		case <-serverEnd.closed:
		}
		return ErrConnectionClosed
	}
	defer close(stuck)
	done := make(chan struct{})
	go func() { s.handleConnection(serverEnd); close(done) }()
	_ = clientEnd

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("server keepalive did not close a connection whose writes are stuck")
	}
}

func TestKeepaliveTimeoutNotBelowInterval(t *testing.T) {
	// A timeout <= interval would declare an idle connection dead before it
	// was ever probed; it is raised to 2*interval, so a peer that answers
	// PINGs stays connected.
	s := NewServer(ServerConfig{KeepaliveInterval: 40 * time.Millisecond, KeepaliveTimeout: 10 * time.Millisecond})
	clientEnd, done := startServerConn(s)
	defer func() { clientEnd.Close(); <-done }()

	stop := make(chan struct{})
	go func() {
		// Answer PINGs like a real client.
		for {
			b, err := clientEnd.Receive()
			if err != nil {
				return
			}
			r := serialize.NewReader(b)
			var prefix [16]byte
			if DeserializePrefix(&prefix, r) == nil && prefix == StreamPrefix {
				_ = clientEnd.Send(serializeStreamControl(StreamFramePong), 0)
			}
			select {
			case <-stop:
				return
			default:
			}
		}
	}()
	time.Sleep(300 * time.Millisecond)
	close(stop)
	select {
	case <-done:
		t.Fatal("idle-but-responsive connection was closed by the keepalive")
	default:
	}
}

func TestConcurrentConfigurationIsRaceFree(t *testing.T) {
	s := NewServer(ServerConfig{})
	c := NewClient(ClientConfig{})
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				id := uint64(g*1000 + i)
				s.RegisterServer(id, "svc", &echoStub{})
				s.Middleware(func(ctx context.Context, m Message, next Handler) (Message, error) { return next(ctx, m) })
				s.Group(func(s *Server) {
					s.Middleware(func(ctx context.Context, m Message, next Handler) (Message, error) { return next(ctx, m) })
				})
				_, _ = s.lookupService(id)
				c.Middleware(func(ctx context.Context, m Message, next Handler) (Message, error) { return next(ctx, m) })
				_ = len(c.GetMiddleware())
			}
		}(g)
	}
	wg.Wait()
	assert.Len(t, c.GetMiddleware(), 8*50)
	e, err := s.lookupService(7049)
	require.NoError(t, err)
	assert.NotNil(t, e.stub)
}

func TestMiddlewareAddedAfterLookupApplies(t *testing.T) {
	s := NewServer(ServerConfig{})
	s.RegisterServer(1, "echo", &echoStub{})
	e, err := s.lookupService(1)
	require.NoError(t, err)
	assert.Len(t, e.middleware, 0)

	s.Middleware(func(ctx context.Context, m Message, next Handler) (Message, error) { return next(ctx, m) })
	e, err = s.lookupService(1)
	require.NoError(t, err)
	assert.Len(t, e.middleware, 1, "the cached middleware stack must be invalidated")
}

func TestRecvQueueSemantics(t *testing.T) {
	q := newRecvQueue(4)
	assert.Equal(t, 0, cap(q.items), "the queue must not preallocate its bound")

	readers := make([]*serialize.Reader, 5)
	for i := range readers {
		readers[i] = serialize.NewReader([]byte{byte(i)})
	}
	for i := 0; i < 4; i++ {
		accepted, overflow := q.push(readers[i])
		require.True(t, accepted)
		require.False(t, overflow)
	}
	_, overflow := q.push(readers[4])
	assert.True(t, overflow, "the 5th message overflows a queue bounded at 4")

	// FIFO, then terminal status only after the buffer drains.
	q.close(io.EOF)
	for i := 0; i < 4; i++ {
		r, err, ok := q.pop()
		require.True(t, ok)
		require.NoError(t, err)
		assert.Same(t, readers[i], r)
	}
	_, err, ok := q.pop()
	require.True(t, ok)
	assert.Equal(t, io.EOF, err)

	accepted, _ := q.push(readers[0])
	assert.False(t, accepted, "no deliveries after close")
	assert.False(t, q.close(errors.New("again")), "close is idempotent")

	// A consumer trailing the producer must not grow the buffer unboundedly.
	q2 := newRecvQueue(8)
	for i := 0; i < 10000; i++ {
		_, overflow := q2.push(readers[0])
		require.False(t, overflow)
		if i%2 == 1 {
			q2.pop()
			q2.pop()
		}
	}
	assert.LessOrEqual(t, cap(q2.items), 64)
}

func TestClientStreamRecvHonoursCancellationUnderLoad(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	clientEnd, _ := memPipe()
	c := NewClient(ClientConfig{})
	s := newClientStream(c, clientEnd, ctx, 1, 1, 0)
	for i := 0; i < 100; i++ {
		s.deliver(serialize.NewReader([]byte{1}))
	}
	cancel()
	_, err := s.Recv()
	assert.ErrorIs(t, err, context.Canceled, "cancellation must win over buffered messages")
}

// gatedTransport is a client transport over in-memory pipes to a Server whose
// dials block until the test releases them, one gate per dial.
type gatedTransport struct {
	server *Server
	dials  chan chan struct{}
}

func (g *gatedTransport) Connect() (Connection, error) {
	return g.ConnectContext(context.Background())
}

func (g *gatedTransport) ConnectContext(ctx context.Context) (Connection, error) {
	gate := make(chan struct{})
	g.dials <- gate
	select {
	case <-gate:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	clientEnd, serverEnd := memPipe()
	go g.server.handleConnection(serverEnd)
	return clientEnd, nil
}

func echoCall(c *Client, ctx context.Context, payload string) (string, error) {
	r, err := c.Call(ctx, 1, 1, &rawMsg{data: []byte(payload)})
	if err != nil {
		return "", err
	}
	m := &rawMsg{}
	if err := m.Deserialize(r); err != nil {
		return "", err
	}
	return string(m.data), nil
}

// TestCallAfterCloseDoesNotJoinDiscardedDial: a dial in flight when Close is
// called is discarded; a call made after Close must start its own dial and
// succeed, not wait on (and fail with) the discarded one.
func TestCallAfterCloseDoesNotJoinDiscardedDial(t *testing.T) {
	s := NewServer(ServerConfig{})
	s.RegisterServer(1, "echo", &echoStub{})
	tr := &gatedTransport{server: s, dials: make(chan chan struct{}, 4)}
	c := NewClient(ClientConfig{Transport: tr})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errA := make(chan error, 1)
	go func() { _, err := echoCall(c, ctx, "a"); errA <- err }()
	gate1 := <-tr.dials

	require.NoError(t, c.Close())

	resB := make(chan string, 1)
	errB := make(chan error, 1)
	go func() {
		v, err := echoCall(c, ctx, "b")
		resB <- v
		errB <- err
	}()

	var gate2 chan struct{}
	select {
	case gate2 = <-tr.dials:
	case <-time.After(time.Second):
		t.Fatal("a call made after Close waited on the discarded dial instead of dialing")
	}

	close(gate1)
	assert.ErrorIs(t, <-errA, ErrConnectionClosed, "the call in flight during Close fails")

	close(gate2)
	require.NoError(t, <-errB)
	assert.Equal(t, "b", <-resB)
	c.Close()
}
