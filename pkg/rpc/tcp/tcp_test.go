package tcp

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kbirk/scg/pkg/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// listenLocal starts a plain TCP server transport on an ephemeral port.
func listenLocal(t *testing.T, conf ServerTransportConfig) (*ServerTransport, int) {
	t.Helper()
	conf.Port = 0
	st := NewServerTransport(conf)
	require.NoError(t, st.Listen())
	t.Cleanup(func() { st.Close() })
	return st, st.listener.Addr().(*net.TCPAddr).Port
}

func acceptOne(t *testing.T, st interface {
	Accept() (rpc.Connection, error)
}) rpc.Connection {
	t.Helper()
	ch := make(chan rpc.Connection, 1)
	go func() {
		c, err := st.Accept()
		if err == nil {
			ch <- c
		}
	}()
	select {
	case c := <-ch:
		t.Cleanup(func() { c.Close() })
		return c
	case <-time.After(3 * time.Second):
		t.Fatal("no connection accepted")
		return nil
	}
}

func dialLocal(t *testing.T, port int, conf ClientTransportConfig) rpc.Connection {
	t.Helper()
	conf.Host = "127.0.0.1"
	conf.Port = port
	c, err := NewClientTransport(conf).ConnectContext(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { c.Close() })
	return c
}

func TestFramingRoundTrip(t *testing.T) {
	st, port := listenLocal(t, ServerTransportConfig{})
	client := dialLocal(t, port, ClientTransportConfig{})
	server := acceptOne(t, st)

	sizes := []int{0, 1, 3, 100, smallFrameMax - 1, smallFrameMax, smallFrameMax + 1, 300 << 10}
	go func() {
		for range sizes {
			b, err := server.Receive()
			if err != nil {
				return
			}
			if err := server.Send(b, 0); err != nil {
				return
			}
		}
	}()
	for _, n := range sizes {
		payload := make([]byte, n)
		for i := range payload {
			payload[i] = byte(i * 7)
		}
		require.NoError(t, client.Send(payload, 0))
		got, err := client.Receive()
		require.NoError(t, err, "size %d", n)
		require.True(t, bytes.Equal(payload, got), "size %d echoed incorrectly", n)
	}
}

// TestDefaultConfigLatency guards against re-enabling Nagle's algorithm: with
// the default (zero-value) config, request/response round trips must be fast.
// Previously NoDelay=false disabled TCP_NODELAY, and the separate header and
// payload writes then stalled behind the peer's delayed ACK (~40-90ms each).
func TestDefaultConfigLatency(t *testing.T) {
	st, port := listenLocal(t, ServerTransportConfig{})
	client := dialLocal(t, port, ClientTransportConfig{})
	server := acceptOne(t, st)

	go func() {
		for {
			b, err := server.Receive()
			if err != nil {
				return
			}
			if server.Send(b, 0) != nil {
				return
			}
		}
	}()
	payload := make([]byte, 200)
	// Warm up.
	require.NoError(t, client.Send(payload, 0))
	_, err := client.Receive()
	require.NoError(t, err)

	const iters = 40
	start := time.Now()
	for i := 0; i < iters; i++ {
		require.NoError(t, client.Send(payload, 0))
		_, err := client.Receive()
		require.NoError(t, err)
	}
	avg := time.Since(start) / iters
	t.Logf("average round trip with default config: %s", avg)
	assert.Less(t, avg, 10*time.Millisecond, "default config must not incur delayed-ACK stalls")
}

func TestSendSizeLimit(t *testing.T) {
	st, port := listenLocal(t, ServerTransportConfig{})
	client := dialLocal(t, port, ClientTransportConfig{MaxSendMessageSize: 10})
	server := acceptOne(t, st)

	err := client.Send(make([]byte, 11), 0)
	require.Error(t, err)
	assert.ErrorIs(t, err, rpc.ErrMessageTooLarge)
	assert.Equal(t, "message size 11 exceeds send limit 10", err.Error())

	// Nothing was written, so the connection is still usable.
	require.NoError(t, client.Send([]byte("0123456789"), 0))
	got, err := server.Receive()
	require.NoError(t, err)
	assert.Equal(t, []byte("0123456789"), got)
}

func TestRecvSizeLimit(t *testing.T) {
	st, port := listenLocal(t, ServerTransportConfig{MaxRecvMessageSize: 10})
	client := dialLocal(t, port, ClientTransportConfig{})
	server := acceptOne(t, st)

	require.NoError(t, client.Send(make([]byte, 11), 0))
	_, err := server.Receive()
	require.Error(t, err)
	assert.ErrorIs(t, err, rpc.ErrMessageTooLarge)
}

func TestCloseSemantics(t *testing.T) {
	st, port := listenLocal(t, ServerTransportConfig{})
	client := dialLocal(t, port, ClientTransportConfig{})
	server := acceptOne(t, st)

	// Peer close: the other side sees a clean ErrConnectionClosed.
	require.NoError(t, client.Close())
	_, err := server.Receive()
	assert.ErrorIs(t, err, rpc.ErrConnectionClosed)

	// Local close: our own blocked/next Receive and Send report the same.
	_, err = client.Receive()
	assert.ErrorIs(t, err, rpc.ErrConnectionClosed)
	assert.ErrorIs(t, client.Send([]byte("x"), 0), rpc.ErrConnectionClosed)
}

func TestListenErrors(t *testing.T) {
	l, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer l.Close()
	busy := l.Addr().(*net.TCPAddr).Port

	st := NewServerTransport(ServerTransportConfig{Port: busy})
	assert.Error(t, st.Listen(), "binding a port in use must fail")

	st2 := NewServerTransport(ServerTransportConfig{Port: 0})
	require.NoError(t, st2.Close())
	assert.ErrorIs(t, st2.Listen(), rpc.ErrTransportClosed, "Listen after Close must not leak a listener")
	_, err = st2.Accept()
	assert.ErrorIs(t, err, rpc.ErrTransportClosed)
}

func TestWriteTimeout(t *testing.T) {
	st, port := listenLocal(t, ServerTransportConfig{})
	client := dialLocal(t, port, ClientTransportConfig{WriteTimeout: 100 * time.Millisecond})
	_ = acceptOne(t, st) // never reads

	chunk := make([]byte, 1<<20)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := client.Send(chunk, 0); err != nil {
			var ne net.Error
			require.True(t, errors.As(err, &ne) && ne.Timeout(), "expected a timeout, got %v", err)
			return
		}
	}
	t.Fatal("writes to a peer that never reads did not time out")
}

// ----------------------------------------------------------------------------
// TLS
// ----------------------------------------------------------------------------

func writeSelfSignedCert(t *testing.T) (certFile, keyFile string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	dir := t.TempDir()
	certFile = filepath.Join(dir, "cert.pem")
	keyFile = filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600))
	return certFile, keyFile
}

func TestTLSRoundTripAndVerification(t *testing.T) {
	certFile, keyFile := writeSelfSignedCert(t)
	st := NewServerTransportTLS(ServerTransportTLSConfig{Port: 0, CertFile: certFile, KeyFile: keyFile})
	require.NoError(t, st.Listen())
	defer st.Close()
	port := st.listener.Addr().(*net.TCPAddr).Port

	// Verified against the CA file.
	ct := NewClientTransportTLS(ClientTransportTLSConfig{Host: "localhost", Port: port, CAFile: certFile})
	client, err := ct.ConnectContext(context.Background())
	require.NoError(t, err)
	defer client.Close()
	server := acceptOne(t, st)
	require.NoError(t, client.Send([]byte("hello"), 0))
	got, err := server.Receive()
	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), got)

	// Without the CA, verification must fail.
	bad := NewClientTransportTLS(ClientTransportTLSConfig{Host: "localhost", Port: port})
	_, err = bad.ConnectContext(context.Background())
	assert.Error(t, err, "an untrusted certificate must be rejected")
}

// TestTLSServerHandshakeTimeout: a client that connects and never handshakes
// is dropped after HandshakeTimeout, and does not block other clients.
func TestTLSServerHandshakeTimeout(t *testing.T) {
	certFile, keyFile := writeSelfSignedCert(t)
	st := NewServerTransportTLS(ServerTransportTLSConfig{
		Port: 0, CertFile: certFile, KeyFile: keyFile, HandshakeTimeout: 200 * time.Millisecond,
	})
	require.NoError(t, st.Listen())
	defer st.Close()
	port := st.listener.Addr().(*net.TCPAddr).Port

	silent, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", itoa(port)))
	require.NoError(t, err)
	defer silent.Close()

	// A well-behaved client is served while the silent one is pending.
	ct := NewClientTransportTLS(ClientTransportTLSConfig{Host: "localhost", Port: port, InsecureSkipVerify: true})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	good, err := ct.ConnectContext(ctx)
	require.NoError(t, err, "a stalled handshake must not block other clients")
	good.Close()

	// The silent client is disconnected once the handshake times out.
	start := time.Now()
	silent.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, err = silent.Read(make([]byte, 1))
	require.Error(t, err)
	var ne net.Error
	assert.False(t, errors.As(err, &ne) && ne.Timeout(), "server should have closed the connection: %v", err)
	assert.Less(t, time.Since(start), 2*time.Second)
}

// TestTLSClientHandshakeHonoursContext: a server that accepts TCP but never
// speaks TLS cannot hang ConnectContext past the caller's deadline.
func TestTLSClientHandshakeHonoursContext(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			defer c.Close() // hold it open, say nothing
		}
	}()

	ct := NewClientTransportTLS(ClientTransportTLSConfig{Host: "127.0.0.1", Port: l.Addr().(*net.TCPAddr).Port, InsecureSkipVerify: true})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err = ct.ConnectContext(ctx)
	require.Error(t, err)
	assert.Less(t, time.Since(start), 2*time.Second)
}

func itoa(i int) string { return big.NewInt(int64(i)).String() }

// TestTLSClientRereadsCAFile: a CA file that is missing (or rotated) at one
// connect is picked up at the next; a failed read is not cached.
func TestTLSClientRereadsCAFile(t *testing.T) {
	certFile, keyFile := writeSelfSignedCert(t)
	st := NewServerTransportTLS(ServerTransportTLSConfig{Port: 0, CertFile: certFile, KeyFile: keyFile})
	require.NoError(t, st.Listen())
	defer st.Close()
	go func() {
		for {
			c, err := st.Accept()
			if err != nil {
				return
			}
			defer c.Close()
		}
	}()
	port := st.listener.Addr().(*net.TCPAddr).Port

	caFile := filepath.Join(t.TempDir(), "ca.pem")
	ct := NewClientTransportTLS(ClientTransportTLSConfig{Host: "localhost", Port: port, CAFile: caFile})

	_, err := ct.ConnectContext(context.Background())
	require.Error(t, err, "the CA file does not exist yet")

	pem, err := os.ReadFile(certFile)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(caFile, pem, 0o600))

	c, err := ct.ConnectContext(context.Background())
	require.NoError(t, err, "the CA file written after a failed read must be used")
	c.Close()

	// Rotate to a different (untrusted for this server) CA: verification fails.
	otherCert, _ := writeSelfSignedCert(t)
	otherPEM, err := os.ReadFile(otherCert)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(caFile, otherPEM, 0o600))
	_, err = ct.ConnectContext(context.Background())
	require.Error(t, err, "a rotated CA file must take effect")
}

// TestWriteFailureClosesConnection: a write that fails partway through a frame
// (here a WriteTimeout to a peer that is not reading) must tear the connection
// down. Otherwise later sends land inside the truncated frame and the peer
// waits forever for the rest of it.
func TestWriteFailureClosesConnection(t *testing.T) {
	st, port := listenLocal(t, ServerTransportConfig{})
	client := dialLocal(t, port, ClientTransportConfig{WriteTimeout: 100 * time.Millisecond})
	server := acceptOne(t, st) // not reading yet

	err := client.Send(make([]byte, 16<<20), 0)
	require.Error(t, err, "16 MiB to a peer that is not reading must time out")
	assert.NotErrorIs(t, err, rpc.ErrConnectionClosed, "a broken stream is an error, not a clean close")

	// The peer starts reading; it must see the connection end.
	peerErr := make(chan error, 1)
	go func() {
		for {
			if _, err := server.Receive(); err != nil {
				peerErr <- err
				return
			}
		}
	}()

	assert.Error(t, client.Send([]byte("next"), 0), "sends after a failed write must fail")

	localErr := make(chan error, 1)
	go func() {
		_, err := client.Receive()
		localErr <- err
	}()
	select {
	case err := <-localErr:
		assert.Error(t, err)
		assert.NotErrorIs(t, err, rpc.ErrConnectionClosed, "the local side reports the write failure")
	case <-time.After(5 * time.Second):
		t.Fatal("the local side's Receive did not observe the broken connection")
	}

	select {
	case err := <-peerErr:
		assert.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("the peer is stuck waiting for the rest of a truncated frame")
	}
}
