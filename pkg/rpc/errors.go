package rpc

import (
	"errors"
	"fmt"
	"net"
)

// Sentinel errors. Transports return (or wrap) these so the client and server
// can classify failures with errors.Is instead of comparing error strings. The
// messages are unchanged from earlier releases for compatibility with callers
// that still compare text.
var (
	// ErrConnectionClosed reports a connection that was closed cleanly, by
	// either peer or locally. It is not treated as an error by the client or
	// server (ErrHandler is not invoked for it).
	ErrConnectionClosed = errors.New("connection closed")

	// ErrTransportClosed is returned by ServerTransport.Accept (and Listen)
	// once the transport has been closed.
	ErrTransportClosed = errors.New("transport is closed")

	// ErrServerShuttingDown is returned to callers whose requests arrive after
	// Server.Shutdown has begun.
	ErrServerShuttingDown = errors.New("server shutting down")

	// ErrMessageTooLarge is matched (via errors.Is) by the errors transports
	// return when a frame exceeds the configured send or receive limit.
	ErrMessageTooLarge = errors.New("message too large")

	// ErrServerBusy is returned to callers whose unary request arrives when a
	// connection already has MaxConcurrentRequests requests running and as
	// many queued.
	ErrServerBusy = errors.New("server busy")
)

// ForceCloser is optionally implemented by connections whose Close performs a
// graceful handshake that may block (e.g. a websocket close frame).
// ForceClose must close the connection immediately; the server uses it once a
// Shutdown deadline has passed.
type ForceCloser interface {
	ForceClose() error
}

type messageSizeError struct {
	msg string
}

func (e *messageSizeError) Error() string        { return e.msg }
func (e *messageSizeError) Is(target error) bool { return target == ErrMessageTooLarge }

// NewMessageSizeError builds the error transports return when a frame of size
// bytes exceeds limit. direction is "send" or "receive". The result matches
// ErrMessageTooLarge with errors.Is and keeps the historical message text.
func NewMessageSizeError(size uint64, limit uint64, direction string) error {
	return &messageSizeError{msg: fmt.Sprintf("message size %d exceeds %s limit %d", size, direction, limit)}
}

// IsConnectionClosed reports whether err represents a closed connection: the
// ErrConnectionClosed sentinel, a use-of-closed-connection error from the net
// package, or (for third-party transports predating the sentinel) an error
// whose text is exactly "connection closed".
func IsConnectionClosed(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrConnectionClosed) || errors.Is(err, net.ErrClosed) {
		return true
	}
	return err.Error() == ErrConnectionClosed.Error()
}

func isTransportClosed(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrTransportClosed) || err.Error() == ErrTransportClosed.Error()
}
