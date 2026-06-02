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

// Flow control (credit-based, server-authoritative). The server dictates these
// byte windows via the SETTINGS frame on every accepted connection. The
// per-stream window also bounds the receive buffer: a sender that exceeds its
// granted credit overruns the window — a protocol violation.
constexpr uint64_t DEFAULT_INITIAL_STREAM_WINDOW = 1u << 20;     // 1 MiB per-stream window
constexpr uint64_t DEFAULT_INITIAL_CONNECTION_WINDOW = 4u << 20; // 4 MiB connection window (phase 3)

inline uint64_t initialStreamWindowOrDefault(uint64_t n)
{
	return n > 0 ? n : DEFAULT_INITIAL_STREAM_WINDOW;
}

inline uint64_t initialConnectionWindowOrDefault(uint64_t n)
{
	return n > 0 ? n : DEFAULT_INITIAL_CONNECTION_WINDOW;
}

// streamMessageCost returns the exact wire byte size of a MESSAGE frame for msg
// on the given stream — the unit of flow-control credit. The receiver derives
// the identical value from the received frame's length (Reader::size), so both
// ends agree on cost.
template <typename T>
inline size_t streamMessageCost(uint64_t streamID, const T& msg)
{
	using scg::serialize::bit_size;
	return scg::serialize::bits_to_bytes(
		bit_size(STREAM_PREFIX) +
		bit_size(streamID) +
		bit_size(STREAM_FRAME_MESSAGE) +
		bit_size(msg));
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

// serializeStreamWindowUpdate grants `increment` more bytes of credit to the
// sender on that stream (or the whole connection when streamID == 0).
inline std::vector<uint8_t> serializeStreamWindowUpdate(uint64_t streamID, uint64_t increment)
{
	using scg::serialize::bit_size;

	serialize::Writer writer(
		scg::serialize::bits_to_bytes(
			bit_size(STREAM_PREFIX) +
			bit_size(streamID) +
			bit_size(STREAM_FRAME_WINDOW_UPDATE) +
			bit_size(increment)));

	writer.write(STREAM_PREFIX);
	writer.write(streamID);
	writer.write(STREAM_FRAME_WINDOW_UPDATE);
	writer.write(increment);
	return writer.bytes();
}

// serializeStreamSettings builds the server-dictated SETTINGS frame (streamID
// 0). Sent by the server only, as the first frame on each accepted connection.
inline std::vector<uint8_t> serializeStreamSettings(uint64_t initialStreamWindow, uint64_t initialConnectionWindow)
{
	using scg::serialize::bit_size;

	uint64_t streamID = 0;
	serialize::Writer writer(
		scg::serialize::bits_to_bytes(
			bit_size(STREAM_PREFIX) +
			bit_size(streamID) +
			bit_size(STREAM_FRAME_SETTINGS) +
			bit_size(initialStreamWindow) +
			bit_size(initialConnectionWindow)));

	writer.write(STREAM_PREFIX);
	writer.write(streamID);
	writer.write(STREAM_FRAME_SETTINGS);
	writer.write(initialStreamWindow);
	writer.write(initialConnectionWindow);
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
	// maxBytes bounds buffered *bytes* (not message count) so a window's worth of
	// tiny messages cannot blow memory.
	explicit StreamRecvQueue(size_t maxBytes)
		: maxBytes_(maxBytes)
	{
	}

	// deliver enqueues an inbound message (positioned at its payload) tagged with
	// its wire cost. Returns true if the byte window was exceeded — i.e. the peer
	// sent more than its granted credit — in which case the stream is now dead and
	// the caller must respond to the violation. Never blocks the producer.
	bool deliver(serialize::Reader&& reader, size_t cost)
	{
		std::lock_guard<std::mutex> lock(mu_);
		if (recvClosed_) {
			return false;
		}
		if (bytes_ + cost > maxBytes_) {
			dead_ = true;
			recvClosed_ = true;
			recvErr_ = error::Error("stream receive buffer overflow");
			cv_.notify_all();
			return true;
		}
		queue_.push_back(std::make_pair(std::move(reader), cost));
		bytes_ += cost;
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
	// If costOut is non-null it receives the consumed message's wire cost (so the
	// receiver can replenish exactly that many bytes of credit).
	StreamRecvState tryRecv(serialize::Reader& out, error::Error& err, size_t* costOut = nullptr)
	{
		std::lock_guard<std::mutex> lock(mu_);
		if (!queue_.empty()) {
			out = std::move(queue_.front().first);
			size_t cost = queue_.front().second;
			queue_.pop_front();
			bytes_ -= cost;
			if (costOut) {
				*costOut = cost;
			}
			return StreamRecvState::Message;
		}
		if (recvClosed_) {
			err = recvErr_;
			return StreamRecvState::Closed;
		}
		return StreamRecvState::Empty;
	}

	// recv blocks until a message arrives or the stream terminates.
	StreamRecvState recv(serialize::Reader& out, error::Error& err, size_t* costOut = nullptr)
	{
		std::unique_lock<std::mutex> lock(mu_);
		cv_.wait(lock, [this]() { return !queue_.empty() || recvClosed_; });
		if (!queue_.empty()) {
			out = std::move(queue_.front().first);
			size_t cost = queue_.front().second;
			queue_.pop_front();
			bytes_ -= cost;
			if (costOut) {
				*costOut = cost;
			}
			return StreamRecvState::Message;
		}
		err = recvErr_;
		return StreamRecvState::Closed;
	}

private:
	size_t maxBytes_;
	size_t bytes_ = 0;
	std::mutex mu_;
	std::condition_variable cv_;
	std::deque<std::pair<serialize::Reader, size_t>> queue_;
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
		// The client's inbound (server->client) buffer is byte-bounded; until S2C
		// flow control (phase 2) it is just a generous safety bound.
		, queue_(bufferSize > 0 ? bufferSize : static_cast<size_t>(DEFAULT_INITIAL_STREAM_WINDOW))
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

	// send writes a message to the server. Under flow control it BLOCKS until the
	// stream has enough send credit (or the stream dies / closeSend is called), so
	// a fast producer cannot outrun a slow server. Frame-loop callers that must
	// not block should use trySend.
	template <typename T>
	error::Error send(const T& msg)
	{
		size_t cost = streamMessageCost(streamID_, msg);
		std::unique_lock<std::mutex> lock(sendMu_);
		sendCv_.wait(lock, [&]() {
			return sendDead_ || sendClosed_.load() || sendCredit_ >= static_cast<int64_t>(cost);
		});
		if (sendDead_) {
			return error::Error("stream closed");
		}
		if (sendClosed_.load()) {
			return error::Error("stream send is already closed");
		}
		sendCredit_ -= static_cast<int64_t>(cost);
		lock.unlock();
		return sendFn_(serializeStreamMessage(streamID_, msg));
	}

	// trySend is the non-blocking counterpart to send: it sends and returns
	// (true, nil) when credit is available, (false, nil) when the stream is out of
	// credit (hold the message and retry next frame), or (false, err) on a
	// terminal condition.
	template <typename T>
	std::pair<bool, error::Error> trySend(const T& msg)
	{
		size_t cost = streamMessageCost(streamID_, msg);
		std::unique_lock<std::mutex> lock(sendMu_);
		if (sendDead_) {
			return std::make_pair(false, error::Error("stream closed"));
		}
		if (sendClosed_.load()) {
			return std::make_pair(false, error::Error("stream send is already closed"));
		}
		if (sendCredit_ < static_cast<int64_t>(cost)) {
			return std::make_pair(false, error::Error(nullptr)); // out of credit
		}
		sendCredit_ -= static_cast<int64_t>(cost);
		lock.unlock();
		auto err = sendFn_(serializeStreamMessage(streamID_, msg));
		if (err) {
			return std::make_pair(false, err);
		}
		return std::make_pair(true, error::Error(nullptr));
	}

	// addSendCredit grants n more bytes of send credit (the initial window from
	// SETTINGS, or a WINDOW_UPDATE) and wakes a blocked send. Called by the Client.
	void addSendCredit(int64_t n)
	{
		{
			std::lock_guard<std::mutex> lock(sendMu_);
			sendCredit_ += n;
		}
		sendCv_.notify_all();
	}

	// closeSend signals the client is done sending; it may still receive.
	error::Error closeSend()
	{
		if (sendClosed_.exchange(true)) {
			return nullptr; // already closed
		}
		sendCv_.notify_all(); // wake a blocked send so it observes the close
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
	// true if the byte window overflowed (caller must notify the peer).
	bool deliver(serialize::Reader&& reader)
	{
		size_t cost = reader.size();
		return queue_.deliver(std::move(reader), cost);
	}
	void closeRecv(error::Error err) { queue_.closeRecv(err); }
	void die(error::Error err)
	{
		queue_.die(err);
		{
			std::lock_guard<std::mutex> lock(sendMu_);
			sendDead_ = true;
		}
		sendCv_.notify_all();
	}

private:
	context::Context ctx_;
	uint64_t streamID_;
	SendFn sendFn_;
	std::atomic<bool> sendClosed_{false};
	StreamRecvQueue queue_;

	// C2S flow control: send credit in bytes, shared between the app thread (send)
	// and the I/O thread (addSendCredit on WINDOW_UPDATE / SETTINGS).
	std::mutex sendMu_;
	std::condition_variable sendCv_;
	int64_t sendCredit_ = 0;
	bool sendDead_ = false;
};

// ----------------------------------------------------------------------------
// ServerStream — the server side of a bidirectional stream. The handler runs on
// its own thread and may block in recv().
// ----------------------------------------------------------------------------

class ServerStream {
public:
	ServerStream(std::shared_ptr<Connection> conn, const context::Context& ctx, uint64_t streamID, uint64_t window)
		: conn_(conn)
		, ctx_(ctx)
		, streamID_(streamID)
		, queue_(static_cast<size_t>(initialStreamWindowOrDefault(window)))
		, threshold_(static_cast<int64_t>(initialStreamWindowOrDefault(window)) / 2)
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
		size_t cost = 0;
		auto state = queue_.tryRecv(out, err, &cost);
		if (state == StreamRecvState::Message) {
			replenish(cost);
		}
		return state;
	}

	StreamRecvState recv(serialize::Reader& out, error::Error& err)
	{
		size_t cost = 0;
		auto state = queue_.recv(out, err, &cost);
		if (state == StreamRecvState::Message) {
			replenish(cost);
		}
		return state;
	}

	// internal (called by the server demux). deliver returns true if the client
	// exceeded its granted credit (byte window overrun) — a protocol violation.
	bool deliver(serialize::Reader&& reader)
	{
		size_t cost = reader.size();
		return queue_.deliver(std::move(reader), cost);
	}
	void halfClose() { queue_.closeRecv(nullptr); } // clean EOF; handler may still send
	void die(error::Error err) { queue_.die(err); }

private:
	// replenish accrues freed bytes and grants them back to the client as a
	// batched WINDOW_UPDATE once the threshold is crossed, replenishing its send
	// credit (rather than one control frame per message).
	void replenish(size_t cost)
	{
		int64_t grant = 0;
		{
			std::lock_guard<std::mutex> lock(replenishMu_);
			pendingGrant_ += static_cast<int64_t>(cost);
			if (pendingGrant_ < threshold_) {
				return;
			}
			grant = pendingGrant_;
			pendingGrant_ = 0;
		}
		conn_->send(serializeStreamWindowUpdate(streamID_, static_cast<uint64_t>(grant)));
	}

	std::shared_ptr<Connection> conn_;
	context::Context ctx_;
	uint64_t streamID_;
	StreamRecvQueue queue_;

	// C2S flow control replenishment state.
	int64_t threshold_;
	std::mutex replenishMu_;
	int64_t pendingGrant_ = 0;
};

} // namespace rpc
} // namespace scg
