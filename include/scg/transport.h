#pragma once

#include <chrono>
#include <vector>
#include <memory>
#include <functional>
#include "scg/error.h"
#include "scg/const.h"

namespace scg {
namespace rpc {

// deadlineAfter returns now + timeout on the steady clock. A non-positive
// timeout means "no timeout" and yields time_point::max(); a timeout too large
// to represent saturates to time_point::max() instead of overflowing.
inline std::chrono::steady_clock::time_point deadlineAfter(std::chrono::milliseconds timeout)
{
	using clock = std::chrono::steady_clock;
	if (timeout.count() <= 0) {
		return clock::time_point::max();
	}
	auto now = clock::now();
	auto room = std::chrono::duration_cast<std::chrono::milliseconds>(clock::time_point::max() - now);
	if (timeout >= room) {
		return clock::time_point::max();
	}
	return now + std::chrono::duration_cast<clock::duration>(timeout);
}

// addSaturating returns tp + d (d >= 0), saturating at time_point::max().
inline std::chrono::steady_clock::time_point addSaturating(std::chrono::steady_clock::time_point tp, std::chrono::milliseconds d)
{
	using clock = std::chrono::steady_clock;
	auto room = std::chrono::duration_cast<std::chrono::milliseconds>(clock::time_point::max() - tp);
	if (d >= room) {
		return clock::time_point::max();
	}
	return tp + std::chrono::duration_cast<clock::duration>(d);
}

// Connection interface for transport abstraction.
//
// Callback contract (for implementers):
// - message, fail and close handlers are invoked on the transport's I/O thread;
// - exactly one of the fail/close handlers fires per connection (the terminal
//   event), after which no further message handler calls are made. A close the
//   local side requested via close() is reported through the close handler;
// - send()/sendOwned() and close() may be called from any thread.
class Connection {
public:
	virtual ~Connection() = default;

	// Send binary data over the connection
	virtual error::Error send(const std::vector<uint8_t>& data) = 0;

	// sendOwned sends a buffer the caller no longer needs, letting the
	// transport take it without copying. The default copies via send().
	virtual error::Error sendOwned(std::vector<uint8_t>&& data)
	{
		return send(static_cast<const std::vector<uint8_t>&>(data));
	}

	// Set callback for when messages are received. This starts the read loop, so
	// it must be called LAST — after setFailHandler/setCloseHandler — otherwise
	// the io thread may read those handlers while they are still being assigned.
	virtual void setMessageHandler(std::function<void(const std::vector<uint8_t>&)> handler) = 0;

	// setOwnedMessageHandler is setMessageHandler for consumers that want to keep
	// the received buffer: the handler receives it by rvalue and may move from
	// it. Like setMessageHandler it starts the read loop, so call it last. The
	// default adapts setMessageHandler with a copy.
	virtual void setOwnedMessageHandler(std::function<void(std::vector<uint8_t>&&)> handler)
	{
		setMessageHandler([handler](const std::vector<uint8_t>& data) {
			std::vector<uint8_t> copy(data);
			handler(std::move(copy));
		});
	}

	// Set callback for when connection fails
	virtual void setFailHandler(std::function<void(const error::Error&)> handler) = 0;

	// Set callback for when connection closes
	virtual void setCloseHandler(std::function<void()> handler) = 0;

	// Close the connection
	virtual error::Error close() = 0;
};

// Client transport interface
class ClientTransport {
public:
	virtual ~ClientTransport() = default;

	// Connect to the server and return a connection
	virtual std::pair<std::shared_ptr<Connection>, error::Error> connect() = 0;

	// connectWithDeadline is connect() bounded by an absolute deadline
	// (time_point::max() means no deadline). The default ignores the deadline
	// and calls connect(); the built-in transports abort the attempt when the
	// deadline passes.
	virtual std::pair<std::shared_ptr<Connection>, error::Error> connectWithDeadline(std::chrono::steady_clock::time_point deadline)
	{
		(void)deadline;
		return connect();
	}

	// Cleanup and shutdown the transport
	virtual void shutdown() = 0;
};

// Server transport interface
class ServerTransport {
public:
	virtual ~ServerTransport() = default;

	// Start listening for incoming connections
	virtual error::Error startListening() = 0;

	// Set handler for new connections
	virtual void setOnConnection(std::function<void(std::shared_ptr<Connection>)> handler) = 0;

	// Run the transport loop (blocking)
	virtual void runEventLoop() = 0;

	// Stop the transport and unblock runEventLoop(). The built-in transports
	// also close every accepted connection, so peers observe the shutdown
	// promptly instead of waiting on a silent socket.
	virtual void stop() = 0;
};

} // namespace rpc
} // namespace scg
