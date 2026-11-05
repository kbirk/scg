#pragma once

#ifndef ASIO_STANDALONE
#define ASIO_STANDALONE
#endif
#include <asio/bind_executor.hpp>
#include <asio/executor_work_guard.hpp>
#include <asio/io_context.hpp>
#include <asio/ip/tcp.hpp>
#include <asio/post.hpp>
#include <asio/read.hpp>
#include <asio/steady_timer.hpp>
#include <asio/strand.hpp>
#include <asio/write.hpp>
#ifndef SCG_MINIMAL_INCLUDES
// Kept for source compatibility with code relying on the transitive includes.
#include <asio.hpp>
#include <iostream>
#endif

#include "scg/transport.h"
#include "scg/error.h"
#include "scg/logger.h"
#include <array>
#include <atomic>
#include <chrono>
#include <cstdint>
#include <deque>
#include <future>
#include <functional>
#include <memory>
#include <mutex>
#include <string>
#include <thread>
#include <vector>

namespace scg {
namespace tcp {

// DEFAULT_MAX_WRITE_QUEUE_BYTES bounds the bytes a connection will buffer for a
// peer that is not reading. 64 MiB is twice the default maximum frame size, so
// bursts of large responses still fit, while a stalled or malicious peer cannot
// grow the sender's memory without bound. One frame is always accepted when the
// queue is empty, so a single message larger than the cap still goes through.
// When the cap is exceeded the connection is failed (the peer sees it close).
// Set maxWriteQueueBytes to 0 to disable the cap.
constexpr size_t DEFAULT_MAX_WRITE_QUEUE_BYTES = 64u << 20; // 64 MiB

// DEFAULT_CONNECT_TIMEOUT bounds a connect (resolve + TCP connect + TLS
// handshake) that is not otherwise bounded by a call deadline.
constexpr std::chrono::milliseconds DEFAULT_CONNECT_TIMEOUT{10000};

namespace detail {

// CLOSE_FLUSH_TIMEOUT is how long a locally requested close() waits for queued
// writes (and, for TLS, the close_notify exchange) before forcing the socket
// shut.
constexpr std::chrono::milliseconds CLOSE_FLUSH_TIMEOUT{1000};

// drainIoContext runs the completions left in a stopped io_context (aborted
// operations on sockets closed during shutdown, posted closes, ...) so that no
// queued handler keeps a connection alive. Every socket is closed by then, so
// the loop terminates; the iteration cap is only a backstop.
inline void drainIoContext(asio::io_context& io)
{
	for (int i = 0; i < 10000; ++i) {
		io.restart();
		if (io.poll() == 0) {
			break;
		}
	}
}

// PlainStreamOps adapts a raw TCP socket for StreamConnection.
struct PlainStreamOps {
	static asio::ip::tcp::socket& lowest(asio::ip::tcp::socket& s) { return s; }

	// Header and payload go out as one gathered write (writev), so the payload
	// is never copied.
	static constexpr size_t coalesceLimit = 0;

	static bool isCleanEof(const asio::error_code& ec) { return ec == asio::error::eof; }

	template <typename Conn>
	static void finishClose(const std::shared_ptr<Conn>& self)
	{
		self->closeLowestNow();
	}
};

// StreamConnection is the length-prefixed (4-byte big-endian) framed connection
// shared by the plain TCP and TCP+TLS transports, on both client and server.
//
// Threading: every socket operation, the write queue, the read buffers and the
// user callbacks run on a per-connection strand. send()/sendOwned()/close() are
// safe from any thread. When constructed with an io_context keep-alive, the
// connection holds the io_context so it can safely outlive its transport (the
// transport closes it and drains pending handlers on shutdown).
template <typename Stream, typename Ops>
class StreamConnection : public scg::rpc::Connection, public std::enable_shared_from_this<StreamConnection<Stream, Ops>> {
public:
	using Self = StreamConnection<Stream, Ops>;

	StreamConnection(Stream stream,
		uint32_t maxSendMessageSize,
		uint32_t maxRecvMessageSize,
		std::shared_ptr<asio::io_context> ioKeepAlive,
		size_t maxWriteQueueBytes)
		: ioKeepAlive_(std::move(ioKeepAlive))
		, stream_(std::move(stream))
		, strand_(asio::make_strand(stream_.get_executor()))
		, closeTimer_(strand_)
		, maxSendMessageSize_(maxSendMessageSize)
		, maxRecvMessageSize_(maxRecvMessageSize)
		, maxWriteQueueBytes_(maxWriteQueueBytes)
	{
		auto& sock = Ops::lowest(stream_);
		if (sock.is_open()) {
			asio::error_code ec;
			sock.set_option(asio::ip::tcp::no_delay(true), ec);
		}
	}

	error::Error send(const std::vector<uint8_t>& data) override
	{
		if (closed_.load(std::memory_order_acquire)) {
			return error::Error("Connection closed");
		}
		return sendOwned(std::vector<uint8_t>(data));
	}

	error::Error sendOwned(std::vector<uint8_t>&& data) override
	{
		if (closed_.load(std::memory_order_acquire)) {
			return error::Error("Connection closed");
		}
		if (data.size() > UINT32_MAX) {
			return error::Error("Message size exceeds the 4 GiB frame limit");
		}
		uint32_t len = static_cast<uint32_t>(data.size());
		if (maxSendMessageSize_ > 0 && len > maxSendMessageSize_) {
			return error::Error("Message size exceeds send limit");
		}

		size_t frameBytes = size_t(len) + 4;
		size_t prev = queuedBytes_.fetch_add(frameBytes, std::memory_order_relaxed);
		if (maxWriteQueueBytes_ > 0 && prev > 0 && prev + frameBytes > maxWriteQueueBytes_) {
			queuedBytes_.fetch_sub(frameBytes, std::memory_order_relaxed);
			SCG_LOG_ERROR("TCP write queue limit exceeded; failing connection");
			failAsync("Write queue limit exceeded (peer is not reading)");
			return error::Error("Write queue limit exceeded");
		}

		Outgoing out;
		out.frameBytes = frameBytes;
		out.header = {
			static_cast<uint8_t>((len >> 24) & 0xFF),
			static_cast<uint8_t>((len >> 16) & 0xFF),
			static_cast<uint8_t>((len >> 8) & 0xFF),
			static_cast<uint8_t>(len & 0xFF)};
		if (len <= Ops::coalesceLimit) {
			// Small TLS frames: one buffer, so header and payload share a record.
			std::vector<uint8_t> combined;
			combined.reserve(frameBytes);
			combined.insert(combined.end(), out.header.begin(), out.header.end());
			combined.insert(combined.end(), data.begin(), data.end());
			out.payload = std::move(combined);
			out.coalesced = true;
		} else {
			out.payload = std::move(data);
		}

		auto self = this->shared_from_this();
		asio::post(strand_, [self, out = std::move(out)]() mutable {
			self->enqueue(std::move(out));
		});
		return nullptr;
	}

	void setMessageHandler(std::function<void(const std::vector<uint8_t>&)> handler) override
	{
		messageHandler_ = std::move(handler);
		startReading();
	}

	void setOwnedMessageHandler(std::function<void(std::vector<uint8_t>&&)> handler) override
	{
		ownedMessageHandler_ = std::move(handler);
		startReading();
	}

	void setFailHandler(std::function<void(const error::Error&)> handler) override
	{
		failHandler_ = std::move(handler);
	}

	void setCloseHandler(std::function<void()> handler) override
	{
		closeHandler_ = std::move(handler);
	}

	// close flushes queued writes (bounded by CLOSE_FLUSH_TIMEOUT), sends a TLS
	// close_notify where applicable, then closes the socket. The close handler
	// fires once the read loop observes the close.
	error::Error close() override
	{
		bool expected = false;
		if (closed_.compare_exchange_strong(expected, true)) {
			closedLocally_.store(true, std::memory_order_release);
			SCG_LOG_INFO("TCP connection closing");
			auto self = this->shared_from_this();
			asio::post(strand_, [self]() {
				self->beginClose();
			});
		}
		return nullptr;
	}

	// closeNow closes the socket immediately, without flushing. Only call it on
	// the I/O thread or while no thread runs the io_context (transport
	// shutdown); the aborted operations then report the close.
	void closeNow()
	{
		closed_.store(true, std::memory_order_release);
		closedLocally_.store(true, std::memory_order_release);
		closeLowestNow();
	}

	bool isClosed() const
	{
		return closed_.load(std::memory_order_acquire);
	}

	// Used by the stream ops (on the strand).
	Stream& stream() { return stream_; }
	asio::strand<typename Stream::executor_type>& strand() { return strand_; }

	void closeLowestNow()
	{
		asio::error_code ec;
		closeTimer_.cancel();
		auto& sock = Ops::lowest(stream_);
		if (sock.is_open()) {
			sock.shutdown(asio::ip::tcp::socket::shutdown_both, ec);
			sock.close(ec);
		}
	}

private:
	struct Outgoing {
		std::array<uint8_t, 4> header{};
		std::vector<uint8_t> payload;
		size_t frameBytes = 0;
		bool coalesced = false;
	};

	void startReading()
	{
		auto self = this->shared_from_this();
		asio::post(strand_, [self]() {
			self->readHeader();
		});
	}

	void enqueue(Outgoing&& out)
	{
		if (terminated_) {
			queuedBytes_.fetch_sub(out.frameBytes, std::memory_order_relaxed);
			return;
		}
		bool idle = writeQueue_.empty();
		writeQueue_.push_back(std::move(out));
		if (idle) {
			doWrite();
		}
	}

	void doWrite()
	{
		auto self = this->shared_from_this();
		auto onWritten = asio::bind_executor(strand_, [self](std::error_code ec, std::size_t /*length*/) {
			self->onWrite(ec);
		});
		Outgoing& front = writeQueue_.front();
		if (front.coalesced) {
			asio::async_write(stream_, asio::buffer(front.payload), std::move(onWritten));
		} else {
			std::array<asio::const_buffer, 2> buffers = {
				asio::buffer(front.header),
				asio::buffer(front.payload)};
			asio::async_write(stream_, buffers, std::move(onWritten));
		}
	}

	void onWrite(const std::error_code& ec)
	{
		if (ec || terminated_) {
			writeQueue_.clear();
			queuedBytes_.store(0, std::memory_order_relaxed);
			if (!terminated_) {
				SCG_LOG_ERROR("TCP write error: " + ec.message());
				onTerminal(false, ec.message());
			}
			return;
		}
		queuedBytes_.fetch_sub(writeQueue_.front().frameBytes, std::memory_order_relaxed);
		writeQueue_.pop_front();
		if (!writeQueue_.empty()) {
			doWrite();
		} else if (closePending_) {
			finishCloseNow();
		}
	}

	void readHeader()
	{
		if (terminated_) {
			return;
		}
		auto self = this->shared_from_this();
		asio::async_read(stream_, asio::buffer(readHeader_, 4),
			asio::bind_executor(strand_, [self](std::error_code ec, std::size_t /*length*/) {
				self->onHeader(ec);
			}));
	}

	void onHeader(const std::error_code& ec)
	{
		if (ec) {
			onReadError(ec);
			return;
		}
		uint32_t len = (uint32_t(readHeader_[0]) << 24) | (uint32_t(readHeader_[1]) << 16) | (uint32_t(readHeader_[2]) << 8) | uint32_t(readHeader_[3]);
		if (maxRecvMessageSize_ > 0 && len > maxRecvMessageSize_) {
			SCG_LOG_ERROR("TCP message size exceeds receive limit");
			onTerminal(false, "Message size exceeds receive limit");
			return;
		}
		body_.resize(len);
		auto self = this->shared_from_this();
		asio::async_read(stream_, asio::buffer(body_),
			asio::bind_executor(strand_, [self](std::error_code ec, std::size_t /*length*/) {
				self->onBody(ec);
			}));
	}

	void onBody(const std::error_code& ec)
	{
		if (ec) {
			onReadError(ec);
			return;
		}
		if (ownedMessageHandler_) {
			// Hand the buffer over; the next frame allocates a fresh one.
			std::vector<uint8_t> frame = std::move(body_);
			body_.clear();
			ownedMessageHandler_(std::move(frame));
		} else if (messageHandler_) {
			messageHandler_(body_);
		}
		readHeader();
	}

	void onReadError(const std::error_code& ec)
	{
		bool clean = Ops::isCleanEof(ec) || closedLocally_.load(std::memory_order_acquire);
		if (!clean) {
			SCG_LOG_ERROR("TCP read error: " + ec.message());
		}
		onTerminal(clean, ec.message());
	}

	void failAsync(const std::string& message)
	{
		auto self = this->shared_from_this();
		asio::post(strand_, [self, message]() {
			self->onTerminal(false, message);
		});
	}

	// onTerminal reports the terminal event exactly once, then closes the socket
	// and releases the user callbacks (on the strand).
	void onTerminal(bool clean, const std::string& message)
	{
		if (terminated_) {
			return;
		}
		terminated_ = true;
		closed_.store(true, std::memory_order_release);

		if (clean) {
			if (closeHandler_) {
				closeHandler_();
			}
		} else if (failHandler_) {
			failHandler_(error::Error(message));
		}

		closeLowestNow();

		messageHandler_ = nullptr;
		ownedMessageHandler_ = nullptr;
		failHandler_ = nullptr;
		closeHandler_ = nullptr;
	}

	void beginClose()
	{
		if (terminated_) {
			closeLowestNow();
			return;
		}
		// Bound the whole graceful close; the timer forces the socket shut.
		auto self = this->shared_from_this();
		closeTimer_.expires_after(CLOSE_FLUSH_TIMEOUT);
		closeTimer_.async_wait([self](const std::error_code& ec) {
			if (!ec) {
				self->closeLowestNow();
			}
		});
		if (writeQueue_.empty()) {
			finishCloseNow();
		} else {
			closePending_ = true; // onWrite finishes the close once flushed
		}
	}

	void finishCloseNow()
	{
		closePending_ = false;
		Ops::finishClose(this->shared_from_this());
	}

	// Declared first so it is destroyed last: the socket, strand and timer must
	// be destroyed before their io_context.
	std::shared_ptr<asio::io_context> ioKeepAlive_;
	Stream stream_;
	asio::strand<typename Stream::executor_type> strand_;
	asio::steady_timer closeTimer_;

	std::function<void(const std::vector<uint8_t>&)> messageHandler_;
	std::function<void(std::vector<uint8_t>&&)> ownedMessageHandler_;
	std::function<void(const error::Error&)> failHandler_;
	std::function<void()> closeHandler_;

	std::atomic<bool> closed_{false};
	std::atomic<bool> closedLocally_{false};
	std::atomic<size_t> queuedBytes_{0};

	// strand-only state
	bool terminated_ = false;
	bool closePending_ = false;
	std::deque<Outgoing> writeQueue_;
	uint8_t readHeader_[4] = {0, 0, 0, 0};
	std::vector<uint8_t> body_;

	uint32_t maxSendMessageSize_;
	uint32_t maxRecvMessageSize_;
	size_t maxWriteQueueBytes_;
};

// ConnectionRegistry tracks a transport's live connections (weakly) so the
// transport can close them all on shutdown.
template <typename Conn>
class ConnectionRegistry {
public:
	void add(const std::shared_ptr<Conn>& conn)
	{
		std::lock_guard<std::mutex> lock(mu_);
		// Prune expired entries so the registry does not grow with churn.
		if (conns_.size() >= nextPrune_) {
			std::vector<std::weak_ptr<Conn>> live;
			for (auto& w : conns_) {
				if (!w.expired()) {
					live.push_back(w);
				}
			}
			conns_.swap(live);
			nextPrune_ = conns_.size() * 2 + 16;
		}
		conns_.push_back(conn);
	}

	std::vector<std::shared_ptr<Conn>> snapshot()
	{
		std::lock_guard<std::mutex> lock(mu_);
		std::vector<std::shared_ptr<Conn>> out;
		for (auto& w : conns_) {
			if (auto c = w.lock()) {
				out.push_back(std::move(c));
			}
		}
		return out;
	}

	void clear()
	{
		std::lock_guard<std::mutex> lock(mu_);
		conns_.clear();
	}

private:
	std::mutex mu_;
	std::vector<std::weak_ptr<Conn>> conns_;
	size_t nextPrune_ = 16;
};


// ClientIoThread owns a client transport's io_context and the thread running
// it. shutdown() closes the transport's connections on the I/O thread, stops
// and joins the thread, then drains the leftover completions so no queued
// handler keeps a connection (or the io_context) alive.
class ClientIoThread {
public:
	ClientIoThread()
		: io_(std::make_shared<asio::io_context>())
		, guard_(std::make_unique<asio::executor_work_guard<asio::io_context::executor_type>>(asio::make_work_guard(*io_)))
	{
		auto io = io_;
		thread_ = std::thread([io]() {
			io->run();
		});
	}

	~ClientIoThread()
	{
		shutdown([]() {});
	}

	const std::shared_ptr<asio::io_context>& context() const
	{
		return io_;
	}

	bool stopped()
	{
		std::lock_guard<std::mutex> lock(mu_);
		return stopped_;
	}

	// post runs fn on the I/O thread; returns false once shut down.
	template <typename F>
	bool post(F&& fn)
	{
		std::lock_guard<std::mutex> lock(mu_);
		if (stopped_) {
			return false;
		}
		asio::post(*io_, std::forward<F>(fn));
		return true;
	}

	void shutdown(std::function<void()> closeAll)
	{
		{
			std::lock_guard<std::mutex> lock(mu_);
			if (stopped_) {
				return;
			}
			stopped_ = true;
			asio::post(*io_, [closeAll]() {
				closeAll();
			});
			auto io = io_;
			asio::post(*io_, [io]() {
				io->stop();
			});
		}
		guard_.reset();
		if (thread_.joinable()) {
			if (thread_.get_id() == std::this_thread::get_id()) {
				// Shut down from a callback on the I/O thread itself: it cannot
				// join itself; the loop exits once this handler returns.
				thread_.detach();
				return;
			}
			thread_.join();
		}
		drainIoContext(*io_);
	}

private:
	std::mutex mu_;
	bool stopped_ = false;
	std::shared_ptr<asio::io_context> io_;
	std::unique_ptr<asio::executor_work_guard<asio::io_context::executor_type>> guard_;
	std::thread thread_;
};

// PendingConnect is an in-flight connect attempt. Transport shutdown aborts the
// pending attempts on the I/O thread so their callers do not wait on handlers
// that would never run.
struct PendingConnect {
	virtual ~PendingConnect() = default;
	virtual void abort() = 0; // I/O thread only
};

class PendingConnects {
public:
	void add(const std::shared_ptr<PendingConnect>& p)
	{
		std::lock_guard<std::mutex> lock(mu_);
		if (list_.size() >= 16) {
			std::vector<std::weak_ptr<PendingConnect>> live;
			for (auto& w : list_) {
				if (!w.expired()) {
					live.push_back(w);
				}
			}
			list_.swap(live);
		}
		list_.push_back(p);
	}

	void abortAll()
	{
		std::vector<std::shared_ptr<PendingConnect>> live;
		{
			std::lock_guard<std::mutex> lock(mu_);
			for (auto& w : list_) {
				if (auto p = w.lock()) {
					live.push_back(std::move(p));
				}
			}
			list_.clear();
		}
		for (auto& p : live) {
			p->abort();
		}
	}

private:
	std::mutex mu_;
	std::vector<std::weak_ptr<PendingConnect>> list_;
};

// waitForConnect waits for a connect attempt's result. The attempt's own timer
// completes it by the deadline; the slack only guards against the loop being
// stopped underneath it. Returns false on timeout.
template <typename Future>
inline bool waitForConnect(Future& future, std::chrono::steady_clock::time_point deadline)
{
	if (deadline == std::chrono::steady_clock::time_point::max()) {
		future.wait(); // no timeout; transport shutdown aborts the attempt
		return true;
	}
	return future.wait_until(scg::rpc::addSaturating(deadline, std::chrono::seconds(5))) == std::future_status::ready;
}

// connectError converts a connect failure into an Error.
inline error::Error connectError(const std::error_code& ec)
{
	if (ec == asio::error::timed_out) {
		return error::Error("Connection timed out");
	}
	if (ec == asio::error::operation_aborted) {
		return error::Error("Connection aborted: transport shut down");
	}
	return error::Error(ec.message());
}

} // namespace detail
} // namespace tcp
} // namespace scg
