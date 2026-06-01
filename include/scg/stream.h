#pragma once

#include <cstdint>
#include <vector>
#include <deque>
#include <mutex>
#include <condition_variable>
#include <atomic>
#include <memory>
#include <functional>
#include <string>

#include "scg/error.h"
#include "scg/serialize.h"
#include "scg/reader.h"
#include "scg/writer.h"
#include "scg/const.h"
#include "scg/context.h"
#include "scg/transport.h"

namespace scg {
namespace rpc {

// DEFAULT_STREAM_RECV_BUFFER_SIZE bounds the per-stream inbound queue when no
// size is configured. A consumer that cannot keep up cannot grow memory without
// bound: when the buffer overflows the offending stream is terminated with an
// error and the peer is notified, while other streams and the connection's read
// loop are never blocked. This is the v1 backpressure policy. Configurable via
// ClientConfig/ServerConfig.
constexpr size_t DEFAULT_STREAM_RECV_BUFFER_SIZE = 1024;

inline size_t streamRecvBufferSizeOrDefault(size_t size)
{
	return size > 0 ? size : DEFAULT_STREAM_RECV_BUFFER_SIZE;
}

// StreamRecvState is the tri-state result of a (non-blocking) receive. A frame
// loop must distinguish "nothing yet" from "stream ended" without blocking.
enum class StreamRecvState {
	Message, // a message is available
	Empty,   // nothing available right now (tryRecv only)
	Closed   // the stream has terminated (clean if error is nil, else errored)
};

// StreamRecv<T> is the typed result returned by generated stream handles.
template <typename T>
struct StreamRecv {
	StreamRecvState state = StreamRecvState::Empty;
	T message;
	scg::error::Error error;
};

// ----------------------------------------------------------------------------
// Frame serialization
// ----------------------------------------------------------------------------

inline std::vector<uint8_t> serializeStreamOpen(const context::Context& ctx, uint64_t streamID, uint64_t serviceID, uint64_t methodID)
{
	using scg::serialize::bit_size;

	serialize::Writer writer(
		scg::serialize::bits_to_bytes(
			bit_size(STREAM_PREFIX) +
			bit_size(streamID) +
			bit_size(STREAM_FRAME_OPEN) +
			bit_size(ctx) +
			bit_size(serviceID) +
			bit_size(methodID)));

	writer.write(STREAM_PREFIX);
	writer.write(streamID);
	writer.write(STREAM_FRAME_OPEN);
	writer.write(ctx);
	writer.write(serviceID);
	writer.write(methodID);
	return writer.bytes();
}

template <typename T>
inline std::vector<uint8_t> serializeStreamMessage(uint64_t streamID, const T& msg)
{
	using scg::serialize::bit_size;

	serialize::Writer writer(
		scg::serialize::bits_to_bytes(
			bit_size(STREAM_PREFIX) +
			bit_size(streamID) +
			bit_size(STREAM_FRAME_MESSAGE) +
			bit_size(msg)));

	writer.write(STREAM_PREFIX);
	writer.write(streamID);
	writer.write(STREAM_FRAME_MESSAGE);
	writer.write(msg);
	return writer.bytes();
}

// serializeStreamControl builds a connection-level keepalive frame (PING/PONG).
// The stream id is unused (0).
inline std::vector<uint8_t> serializeStreamControl(uint8_t frameKind)
{
	using scg::serialize::bit_size;

	uint64_t streamID = 0;
	serialize::Writer writer(
		scg::serialize::bits_to_bytes(
			bit_size(STREAM_PREFIX) +
			bit_size(streamID) +
			bit_size(frameKind)));

	writer.write(STREAM_PREFIX);
	writer.write(streamID);
	writer.write(frameKind);
	return writer.bytes();
}

inline std::vector<uint8_t> serializeStreamHalfClose(uint64_t streamID)
{
	using scg::serialize::bit_size;

	serialize::Writer writer(
		scg::serialize::bits_to_bytes(
			bit_size(STREAM_PREFIX) +
			bit_size(streamID) +
			bit_size(STREAM_FRAME_HALF_CLOSE)));

	writer.write(STREAM_PREFIX);
	writer.write(streamID);
	writer.write(STREAM_FRAME_HALF_CLOSE);
	return writer.bytes();
}

inline std::vector<uint8_t> serializeStreamClose(uint64_t streamID, uint8_t status, const std::string& message)
{
	using scg::serialize::bit_size;

	serialize::Writer writer(
		scg::serialize::bits_to_bytes(
			bit_size(STREAM_PREFIX) +
			bit_size(streamID) +
			bit_size(STREAM_FRAME_CLOSE) +
			bit_size(status) +
			bit_size(message)));

	writer.write(STREAM_PREFIX);
	writer.write(streamID);
	writer.write(STREAM_FRAME_CLOSE);
	writer.write(status);
	writer.write(message);
	return writer.bytes();
}

// ----------------------------------------------------------------------------
// StreamRecvQueue — the bounded, thread-safe inbound queue shared by both the
// client and server stream handles. Single producer (the I/O thread), single
// consumer (the caller of recv/tryRecv).
// ----------------------------------------------------------------------------

class StreamRecvQueue {
public:
	explicit StreamRecvQueue(size_t bufferSize)
		: bufferSize_(bufferSize)
	{
	}

	// deliver enqueues an inbound message (positioned at its payload). Returns
	// true if the bounded buffer overflowed, in which case the stream is now dead
	// and the caller must notify the peer. Never blocks the producer.
	bool deliver(serialize::Reader&& reader)
	{
		std::lock_guard<std::mutex> lock(mu_);
		if (recvClosed_) {
			return false;
		}
		if (queue_.size() >= bufferSize_) {
			dead_ = true;
			recvClosed_ = true;
			recvErr_ = error::Error("stream receive buffer overflow");
			cv_.notify_all();
			return true;
		}
		queue_.push_back(std::move(reader));
		cv_.notify_all();
		return false;
	}

	// closeRecv marks the recv direction terminal (clean EOF when err is nil).
	void closeRecv(error::Error err)
	{
		std::lock_guard<std::mutex> lock(mu_);
		if (recvClosed_) {
			return;
		}
		recvClosed_ = true;
		recvErr_ = err;
		cv_.notify_all();
	}

	// die marks the whole stream dead (connection dropped / cancelled).
	void die(error::Error err)
	{
		std::lock_guard<std::mutex> lock(mu_);
		dead_ = true;
		if (!recvClosed_) {
			recvClosed_ = true;
			recvErr_ = err;
		}
		cv_.notify_all();
	}

	bool isDead()
	{
		std::lock_guard<std::mutex> lock(mu_);
		return dead_;
	}

	// tryRecv never blocks. Buffered messages are returned before the terminal.
	StreamRecvState tryRecv(serialize::Reader& out, error::Error& err)
	{
		std::lock_guard<std::mutex> lock(mu_);
		if (!queue_.empty()) {
			out = std::move(queue_.front());
			queue_.pop_front();
			return StreamRecvState::Message;
		}
		if (recvClosed_) {
			err = recvErr_;
			return StreamRecvState::Closed;
		}
		return StreamRecvState::Empty;
	}

	// recv blocks until a message arrives or the stream terminates.
	StreamRecvState recv(serialize::Reader& out, error::Error& err)
	{
		std::unique_lock<std::mutex> lock(mu_);
		cv_.wait(lock, [this]() { return !queue_.empty() || recvClosed_; });
		if (!queue_.empty()) {
			out = std::move(queue_.front());
			queue_.pop_front();
			return StreamRecvState::Message;
		}
		err = recvErr_;
		return StreamRecvState::Closed;
	}

private:
	size_t bufferSize_;
	std::mutex mu_;
	std::condition_variable cv_;
	std::deque<serialize::Reader> queue_;
	bool recvClosed_ = false;
	error::Error recvErr_;
	bool dead_ = false;
};

// ----------------------------------------------------------------------------
// ClientStream — the client side of a bidirectional stream. Decoupled from the
// Client type via a send callback so it can live in this header.
// ----------------------------------------------------------------------------

class ClientStream {
public:
	using SendFn = std::function<error::Error(const std::vector<uint8_t>&)>;

	ClientStream(const context::Context& ctx, uint64_t streamID, SendFn sendFn, size_t bufferSize)
		: ctx_(ctx)
		, streamID_(streamID)
		, sendFn_(std::move(sendFn))
		, queue_(streamRecvBufferSizeOrDefault(bufferSize))
	{
	}

	const context::Context& context() const
	{
		return ctx_;
	}

	uint64_t streamID() const
	{
		return streamID_;
	}

	// Send writes a message to the server. Non-blocking; fails once the stream is
	// dead or after closeSend. A server half-close does not stop the client.
	template <typename T>
	error::Error send(const T& msg)
	{
		if (queue_.isDead()) {
			return error::Error("stream closed");
		}
		if (sendClosed_.load()) {
			return error::Error("stream send is already closed");
		}
		return sendFn_(serializeStreamMessage(streamID_, msg));
	}

	// closeSend signals the client is done sending; it may still receive.
	error::Error closeSend()
	{
		if (sendClosed_.exchange(true)) {
			return nullptr; // already closed
		}
		if (queue_.isDead()) {
			return nullptr;
		}
		return sendFn_(serializeStreamHalfClose(streamID_));
	}

	StreamRecvState tryRecv(serialize::Reader& out, error::Error& err)
	{
		return queue_.tryRecv(out, err);
	}

	StreamRecvState recv(serialize::Reader& out, error::Error& err)
	{
		return queue_.recv(out, err);
	}

	// internal (called by the Client demux on the I/O thread). deliver returns
	// true if the bounded buffer overflowed (caller must notify the peer).
	bool deliver(serialize::Reader&& reader) { return queue_.deliver(std::move(reader)); }
	void closeRecv(error::Error err) { queue_.closeRecv(err); }
	void die(error::Error err) { queue_.die(err); }

private:
	context::Context ctx_;
	uint64_t streamID_;
	SendFn sendFn_;
	std::atomic<bool> sendClosed_{false};
	StreamRecvQueue queue_;
};

// ----------------------------------------------------------------------------
// ServerStream — the server side of a bidirectional stream. The handler runs on
// its own thread and may block in recv().
// ----------------------------------------------------------------------------

class ServerStream {
public:
	ServerStream(std::shared_ptr<Connection> conn, const context::Context& ctx, uint64_t streamID, size_t bufferSize)
		: conn_(conn)
		, ctx_(ctx)
		, streamID_(streamID)
		, queue_(streamRecvBufferSizeOrDefault(bufferSize))
	{
	}

	const context::Context& context() const
	{
		return ctx_;
	}

	uint64_t streamID() const
	{
		return streamID_;
	}

	// Send pushes a message to the client. Remains valid after the client
	// half-closes (recv returns Closed); fails only once the stream is dead.
	template <typename T>
	error::Error send(const T& msg)
	{
		if (queue_.isDead()) {
			return error::Error("stream closed");
		}
		return conn_->send(serializeStreamMessage(streamID_, msg));
	}

	StreamRecvState tryRecv(serialize::Reader& out, error::Error& err)
	{
		return queue_.tryRecv(out, err);
	}

	StreamRecvState recv(serialize::Reader& out, error::Error& err)
	{
		return queue_.recv(out, err);
	}

	// internal (called by the server demux). deliver returns true if the bounded
	// buffer overflowed (caller must notify the peer).
	bool deliver(serialize::Reader&& reader) { return queue_.deliver(std::move(reader)); }
	void halfClose() { queue_.closeRecv(nullptr); } // clean EOF; handler may still send
	void die(error::Error err) { queue_.die(err); }

private:
	std::shared_ptr<Connection> conn_;
	context::Context ctx_;
	uint64_t streamID_;
	StreamRecvQueue queue_;
};

} // namespace rpc
} // namespace scg
