#pragma once

#ifndef ASIO_STANDALONE
#define ASIO_STANDALONE
#endif

#include "scg/transport.h"
#include "scg/error.h"
#include "scg/logger.h"

#include <asio/io_context.hpp>
#include <asio/post.hpp>
#include <asio/steady_timer.hpp>
#ifndef SCG_MINIMAL_INCLUDES
// Kept for source compatibility with code relying on the transitive include.
#include <iostream>
#endif

#include <websocketpp/common/connection_hdl.hpp>
#include <websocketpp/logger/levels.hpp>
#include <websocketpp/error.hpp>
#include <websocketpp/close.hpp>
#include <websocketpp/frame.hpp>

#include <atomic>
#include <chrono>
#include <deque>
#include <functional>
#include <future>
#include <memory>
#include <mutex>
#include <string>
#include <thread>
#include <utility>
#include <vector>

namespace scg {
namespace ws {

// DEFAULT_MAX_WRITE_QUEUE_BYTES bounds the bytes websocketpp may buffer for a
// peer that is not reading (see scg::tcp::DEFAULT_MAX_WRITE_QUEUE_BYTES). One
// message is always accepted when nothing is buffered.
constexpr size_t DEFAULT_MAX_WRITE_QUEUE_BYTES = 64u << 20; // 64 MiB

// DEFAULT_CONNECT_TIMEOUT bounds a connect that is not bounded by a call
// deadline.
constexpr std::chrono::milliseconds DEFAULT_CONNECT_TIMEOUT{10000};

namespace detail {

// MAX_EARLY_MESSAGES bounds the frames buffered between the connection opening
// and the owner installing its message handler.
constexpr size_t MAX_EARLY_MESSAGES = 1024;

// SERVER_CLOSE_GRACE is how long a stopping server waits for peers to answer
// its close frames before closing their sockets.
constexpr std::chrono::milliseconds SERVER_CLOSE_GRACE{500};

inline void drainIoContext(asio::io_context& io)
{
	for (int i = 0; i < 10000; ++i) {
		io.restart();
		if (io.poll() == 0) {
			break;
		}
	}
}

// EndpointHolder owns a websocketpp endpoint and the io_context it runs on.
// Connections hold it by shared_ptr, so a connection can never outlive the
// endpoint or io_context it uses. All endpoint/connection operations run on the
// loop thread (websocketpp is not thread-safe); other threads post().
template <typename Endpoint>
class EndpointHolder {
public:
	EndpointHolder()
		: io(std::make_shared<asio::io_context>())
	{
		endpoint.clear_access_channels(websocketpp::log::alevel::all);
		endpoint.clear_error_channels(websocketpp::log::elevel::all);
		endpoint.init_asio(io.get());
	}

	// post runs fn on the loop thread; returns false once stopped.
	template <typename F>
	bool post(F&& fn)
	{
		std::lock_guard<std::mutex> lock(mu_);
		if (stopped_) {
			return false;
		}
		asio::post(*io, std::forward<F>(fn));
		return true;
	}

	bool onLoopThread() const
	{
		return io->get_executor().running_in_this_thread();
	}

	// markStopped refuses further post()s; returns false if already stopped.
	bool markStopped()
	{
		std::lock_guard<std::mutex> lock(mu_);
		if (stopped_) {
			return false;
		}
		stopped_ = true;
		return true;
	}

	bool stopped()
	{
		std::lock_guard<std::mutex> lock(mu_);
		return stopped_;
	}

	// Declared first: destroyed after the endpoint.
	std::shared_ptr<asio::io_context> io;
	Endpoint endpoint;

private:
	std::mutex mu_;
	bool stopped_ = false;
};

// WSConnection adapts a websocketpp connection (client or server side) to
// scg::rpc::Connection.
//
// Threading: websocketpp state is only touched on the loop thread. Handlers
// installed on the websocketpp connection capture a weak_ptr to this object, so
// there is no reference cycle (and no leaked socket) no matter how the
// connection ends. Owner callbacks installed from another thread are posted to
// the loop; frames or a terminal event that arrive before the owner installs
// its message handler are buffered and replayed.
template <typename Endpoint>
class WSConnection : public scg::rpc::Connection, public std::enable_shared_from_this<WSConnection<Endpoint>> {
public:
	using Holder = EndpointHolder<Endpoint>;
	using connection_ptr = typename Endpoint::connection_ptr;
	using message_ptr = typename Endpoint::message_ptr;

	WSConnection(std::shared_ptr<Holder> holder, uint32_t maxSendMessageSize, size_t maxWriteQueueBytes)
		: holder_(std::move(holder))
		, maxSendMessageSize_(maxSendMessageSize)
		, maxWriteQueueBytes_(maxWriteQueueBytes)
	{
	}

	~WSConnection() override
	{
		if (!closed_.load(std::memory_order_acquire) && holder_) {
			// Owner dropped the connection without closing it: close the
			// websocket on the loop. Capture only weak/handle state (no cycle).
			std::weak_ptr<Holder> weakHolder = holder_;
			auto hdl = hdl_;
			holder_->post([weakHolder, hdl]() {
				if (auto h = weakHolder.lock()) {
					websocketpp::lib::error_code ec;
					h->endpoint.close(hdl, websocketpp::close::status::going_away, "", ec);
				}
			});
		}
	}

	// attach binds this object to a websocketpp connection: it records the
	// handle and installs the websocketpp handlers. Call it before the client
	// connection is started, or on the loop thread for server connections.
	void attach(const connection_ptr& con, bool clientSide)
	{
		hdl_ = con->get_handle();
		std::weak_ptr<WSConnection> weak = this->shared_from_this();
		std::weak_ptr<Holder> weakHolder = holder_;

		con->set_message_handler([weak](websocketpp::connection_hdl, message_ptr msg) {
			if (auto self = weak.lock()) {
				self->onMessage(msg);
			}
		});
		con->set_close_handler([weak](websocketpp::connection_hdl) {
			if (auto self = weak.lock()) {
				self->onTerminal(true, "");
			}
		});
		con->set_fail_handler([weak, weakHolder](websocketpp::connection_hdl hdl) {
			auto self = weak.lock();
			if (!self) {
				return;
			}
			std::string reason = "connection failed";
			if (auto h = weakHolder.lock()) {
				websocketpp::lib::error_code ec;
				auto c = h->endpoint.get_con_from_hdl(hdl, ec);
				if (!ec && c) {
					reason = c->get_ec().message();
				}
			}
			self->onFail(reason);
		});
		if (clientSide) {
			con->set_open_handler([weak](websocketpp::connection_hdl) {
				if (auto self = weak.lock()) {
					self->onOpen();
				}
			});
		} else {
			opened_ = true;
		}
	}

	// connectFuture resolves when a client connection opens (nullptr) or fails.
	std::future<error::Error> connectFuture()
	{
		return connectPromise_.get_future();
	}

	error::Error send(const std::vector<uint8_t>& data) override
	{
		if (closed_.load(std::memory_order_acquire)) {
			return error::Error("Connection closed");
		}
		return sendOwned(std::vector<uint8_t>(data));
	}

	// sendOwned validates synchronously and queues the frame on the loop
	// thread. A failure to send there fails the connection, which fails the
	// caller's pending request through the fail handler.
	error::Error sendOwned(std::vector<uint8_t>&& data) override
	{
		if (closed_.load(std::memory_order_acquire)) {
			return error::Error("Connection closed");
		}
		if (maxSendMessageSize_ > 0 && data.size() > maxSendMessageSize_) {
			return error::Error("Message size exceeds send limit");
		}
		auto self = this->shared_from_this();
		bool posted = holder_->post([self, data = std::move(data)]() mutable {
			self->sendOnLoop(std::move(data));
		});
		if (!posted) {
			return error::Error("Connection closed");
		}
		return nullptr;
	}

	void setMessageHandler(std::function<void(const std::vector<uint8_t>&)> handler) override
	{
		onLoop([handler](WSConnection& c) {
			c.messageHandler_ = handler;
			c.handlersReady();
		});
	}

	void setOwnedMessageHandler(std::function<void(std::vector<uint8_t>&&)> handler) override
	{
		onLoop([handler](WSConnection& c) {
			c.ownedMessageHandler_ = handler;
			c.handlersReady();
		});
	}

	void setFailHandler(std::function<void(const error::Error&)> handler) override
	{
		onLoop([handler](WSConnection& c) {
			c.failHandler_ = handler;
		});
	}

	void setCloseHandler(std::function<void()> handler) override
	{
		onLoop([handler](WSConnection& c) {
			c.closeHandler_ = handler;
		});
	}

	// close starts the websocket close handshake (on the loop thread). The
	// close handler fires when it completes.
	error::Error close() override
	{
		bool expected = false;
		if (closed_.compare_exchange_strong(expected, true)) {
			SCG_LOG_INFO("WebSocket connection closing");
			auto self = this->shared_from_this();
			holder_->post([self]() {
				self->closeOnLoop(websocketpp::close::status::normal);
			});
		}
		return nullptr;
	}

	// Loop thread only (or while no thread runs the loop).
	void closeGoingAway()
	{
		closed_.store(true, std::memory_order_release);
		closeOnLoop(websocketpp::close::status::going_away);
	}

	// closeNow closes the underlying socket immediately, whether or not the
	// terminal event has already been reported (a connection failed for
	// overflowing its write queue must still release its socket and buffers).
	// Loop thread only (or while no thread runs the loop).
	void closeNow()
	{
		closed_.store(true, std::memory_order_release);
		websocketpp::lib::error_code ec;
		auto con = holder_->endpoint.get_con_from_hdl(hdl_, ec);
		if (ec || !con) {
			return;
		}
		asio::error_code aec;
		auto& raw = con->get_raw_socket();
		if (raw.is_open()) {
			raw.close(aec);
		} else if (!terminated_) {
			con->terminate(websocketpp::error::make_error_code(websocketpp::error::operation_canceled));
		}
	}

	// Loop thread only.
	bool terminated() const
	{
		return terminated_;
	}

private:
	template <typename F>
	void onLoop(F&& fn)
	{
		if (holder_->onLoopThread()) {
			fn(*this);
			return;
		}
		auto self = this->shared_from_this();
		holder_->post([self, fn = std::forward<F>(fn)]() mutable {
			fn(*self);
		});
	}

	void onOpen()
	{
		opened_ = true;
		connectPromise_.set_value(nullptr);
		if (closed_.load(std::memory_order_acquire)) {
			// Closed (e.g. connect timed out) while still connecting.
			closeOnLoop(websocketpp::close::status::normal);
		}
	}

	void onFail(const std::string& reason)
	{
		if (!opened_) {
			opened_ = true; // the connect attempt is over
			terminated_ = true;
			closed_.store(true, std::memory_order_release);
			connectPromise_.set_value(error::Error(reason.empty() ? "connection failed" : reason));
			return;
		}
		onTerminal(false, reason);
	}

	void onMessage(const message_ptr& msg)
	{
		if (terminated_) {
			return;
		}
		const auto& payload = msg->get_payload();
		std::vector<uint8_t> data(payload.begin(), payload.end());
		if (ownedMessageHandler_) {
			ownedMessageHandler_(std::move(data));
		} else if (messageHandler_) {
			messageHandler_(data);
		} else if (early_.size() < MAX_EARLY_MESSAGES) {
			early_.push_back(std::move(data));
		}
	}

	void handlersReady()
	{
		ready_ = true;
		while (!early_.empty() && !terminated_) {
			auto data = std::move(early_.front());
			early_.pop_front();
			if (ownedMessageHandler_) {
				ownedMessageHandler_(std::move(data));
			} else if (messageHandler_) {
				messageHandler_(data);
			}
		}
		early_.clear();
		if (pendingTerminal_) {
			pendingTerminal_ = false;
			fireTerminal(pendingClean_, pendingReason_);
		}
	}

	void onTerminal(bool clean, const std::string& reason)
	{
		if (terminated_) {
			return;
		}
		terminated_ = true;
		closed_.store(true, std::memory_order_release);
		if (!ready_) {
			pendingTerminal_ = true;
			pendingClean_ = clean;
			pendingReason_ = reason;
			return;
		}
		fireTerminal(clean, reason);
	}

	void fireTerminal(bool clean, const std::string& reason)
	{
		if (clean) {
			if (closeHandler_) {
				closeHandler_();
			}
		} else if (failHandler_) {
			failHandler_(error::Error(reason.empty() ? "connection failed" : reason));
		}
		messageHandler_ = nullptr;
		ownedMessageHandler_ = nullptr;
		failHandler_ = nullptr;
		closeHandler_ = nullptr;
	}

	void sendOnLoop(std::vector<uint8_t>&& data)
	{
		if (terminated_) {
			return;
		}
		websocketpp::lib::error_code ec;
		auto con = holder_->endpoint.get_con_from_hdl(hdl_, ec);
		if (ec || !con) {
			onTerminal(false, "connection lost");
			return;
		}
		size_t buffered = con->get_buffered_amount();
		if (maxWriteQueueBytes_ > 0 && buffered > 0 && buffered + data.size() > maxWriteQueueBytes_) {
			SCG_LOG_ERROR("WebSocket write queue limit exceeded; failing connection");
			closeNow();
			onTerminal(false, "Write queue limit exceeded (peer is not reading)");
			return;
		}
		ec = con->send(data.data(), data.size(), websocketpp::frame::opcode::binary);
		if (ec) {
			SCG_LOG_ERROR("WebSocket send error: " + ec.message());
			closeNow();
			onTerminal(false, "WebSocket send failed: " + ec.message());
		}
	}

	void closeOnLoop(websocketpp::close::status::value code)
	{
		if (terminated_) {
			return;
		}
		websocketpp::lib::error_code ec;
		holder_->endpoint.close(hdl_, code, "", ec);
		if (ec) {
			// Not open (still connecting, or already closing): force it.
			closeNow();
		}
	}

	std::shared_ptr<Holder> holder_;
	websocketpp::connection_hdl hdl_;

	std::atomic<bool> closed_{false};

	// loop-thread state
	bool opened_ = false;
	bool ready_ = false;
	bool terminated_ = false;
	bool pendingTerminal_ = false;
	bool pendingClean_ = false;
	std::string pendingReason_;
	std::deque<std::vector<uint8_t>> early_;
	std::promise<error::Error> connectPromise_;

	std::function<void(const std::vector<uint8_t>&)> messageHandler_;
	std::function<void(std::vector<uint8_t>&&)> ownedMessageHandler_;
	std::function<void(const error::Error&)> failHandler_;
	std::function<void()> closeHandler_;

	uint32_t maxSendMessageSize_;
	size_t maxWriteQueueBytes_;
};

// ConnectionList tracks a transport's live connections (loop thread only).
template <typename Conn>
class ConnectionList {
public:
	void add(const std::shared_ptr<Conn>& conn)
	{
		if (conns_.size() >= nextPrune_) {
			prune();
			nextPrune_ = conns_.size() * 2 + 16;
		}
		conns_.push_back(conn);
	}

	std::vector<std::shared_ptr<Conn>> live()
	{
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
		conns_.clear();
	}

private:
	void prune()
	{
		std::vector<std::weak_ptr<Conn>> keep;
		for (auto& w : conns_) {
			if (!w.expired()) {
				keep.push_back(w);
			}
		}
		conns_.swap(keep);
	}

	std::vector<std::weak_ptr<Conn>> conns_;
	size_t nextPrune_ = 16;
};

// ClientEndpoint drives a websocketpp client endpoint on its own thread and
// implements connect/shutdown for the WS and WSS client transports.
template <typename Endpoint>
class ClientEndpoint {
public:
	using Holder = EndpointHolder<Endpoint>;
	using Conn = WSConnection<Endpoint>;

	explicit ClientEndpoint(uint32_t maxRecvMessageSize)
		: holder_(std::make_shared<Holder>())
	{
		if (maxRecvMessageSize > 0) {
			holder_->endpoint.set_max_message_size(maxRecvMessageSize);
		}
		holder_->endpoint.start_perpetual();
		auto holder = holder_;
		thread_ = std::thread([holder]() {
			holder->endpoint.run();
		});
	}

	~ClientEndpoint()
	{
		shutdown();
	}

	Endpoint& endpoint()
	{
		return holder_->endpoint;
	}

	std::pair<std::shared_ptr<scg::rpc::Connection>, error::Error> connect(const std::string& uri, uint32_t maxSendMessageSize, size_t maxWriteQueueBytes, std::chrono::steady_clock::time_point deadline)
	{
		if (holder_->stopped()) {
			return {nullptr, error::Error("Transport is shut down")};
		}
		websocketpp::lib::error_code ec;
		auto con = holder_->endpoint.get_connection(uri, ec);
		if (ec) {
			SCG_LOG_ERROR("Failed to create WebSocket connection: " + ec.message());
			return {nullptr, error::Error(ec.message())};
		}

		auto conn = std::make_shared<Conn>(holder_, maxSendMessageSize, maxWriteQueueBytes);
		// Handlers are installed before the connection is started, so no event
		// can race their installation.
		conn->attach(con, true);
		auto future = conn->connectFuture();

		auto holder = holder_;
		bool posted = holder_->post([this, holder, con, conn]() {
			conns_.add(conn);
			holder->endpoint.connect(con);
		});
		if (!posted) {
			return {nullptr, error::Error("Transport is shut down")};
		}

		if (deadline == std::chrono::steady_clock::time_point::max()) {
			future.wait(); // websocketpp's connect/handshake timeouts still apply
		} else if (future.wait_until(deadline) == std::future_status::timeout) {
			conn->close();
			return {nullptr, error::Error("Connection timed out")};
		}
		auto err = future.get();
		if (err) {
			SCG_LOG_ERROR("WebSocket connection failed: " + err.message());
			return {nullptr, err};
		}
		SCG_LOG_INFO("WebSocket connection established");
		return {conn, nullptr};
	}

	// shutdown closes every connection, stops and joins the loop thread, and
	// drains the leftover completions.
	void shutdown()
	{
		if (!holder_->markStopped()) {
			return;
		}
		auto holder = holder_;
		asio::post(*holder_->io, [this, holder]() {
			for (auto& conn : conns_.live()) {
				conn->closeNow();
			}
			holder->endpoint.stop_perpetual();
			holder->io->stop();
		});
		if (thread_.joinable()) {
			if (thread_.get_id() == std::this_thread::get_id()) {
				thread_.detach();
				return;
			}
			thread_.join();
		}
		drainIoContext(*holder_->io);
	}

private:
	std::shared_ptr<Holder> holder_;
	ConnectionList<Conn> conns_; // loop thread only
	std::thread thread_;
};

// ServerEndpoint implements the WS and WSS server transports.
template <typename Endpoint>
class ServerEndpoint {
public:
	using Holder = EndpointHolder<Endpoint>;
	using Conn = WSConnection<Endpoint>;

	ServerEndpoint(uint32_t maxRecvMessageSize, uint32_t maxSendMessageSize, size_t maxWriteQueueBytes)
		: holder_(std::make_shared<Holder>())
		, stopTimer_(*holder_->io)
		, maxSendMessageSize_(maxSendMessageSize)
		, maxWriteQueueBytes_(maxWriteQueueBytes)
	{
		if (maxRecvMessageSize > 0) {
			holder_->endpoint.set_max_message_size(maxRecvMessageSize);
		}
		holder_->endpoint.set_open_handler([this](websocketpp::connection_hdl hdl) {
			onOpen(hdl);
		});
	}

	~ServerEndpoint()
	{
		// The owning Server joins the loop before releasing the transport; if
		// the loop is not running, close and drain here.
		bool running;
		{
			std::lock_guard<std::mutex> lock(mu_);
			stopping_ = true;
			running = loopRunning_;
		}
		if (!running) {
			closeAllNow();
			drainIoContext(*holder_->io);
		} else {
			holder_->io->stop();
		}
		// Connections may keep the endpoint alive; drop the handler that
		// refers to this object.
		holder_->endpoint.set_open_handler(nullptr);
	}

	Endpoint& endpoint()
	{
		return holder_->endpoint;
	}

	void setOnConnection(std::function<void(std::shared_ptr<scg::rpc::Connection>)> handler)
	{
		onConnection_ = std::move(handler);
	}

	error::Error listen(int port)
	{
		try {
			holder_->endpoint.set_reuse_addr(true);
			holder_->endpoint.listen(static_cast<uint16_t>(port));
			holder_->endpoint.start_accept();
			std::lock_guard<std::mutex> lock(mu_);
			stopping_ = false;
			return nullptr;
		} catch (const std::exception& e) {
			return error::Error(e.what());
		}
	}

	// run drives the loop until stop(). If stop() already ran (e.g. start()
	// immediately followed by shutdown()), it returns without running, so stop()
	// never touches the endpoint concurrently with the loop.
	void run()
	{
		{
			std::lock_guard<std::mutex> lock(mu_);
			if (stopping_) {
				return;
			}
			if (holder_->io->stopped()) {
				holder_->io->restart();
			}
			loopRunning_ = true;
		}
		holder_->endpoint.run();
		drainIoContext(*holder_->io);
		std::lock_guard<std::mutex> lock(mu_);
		loopRunning_ = false;
	}

	// stop stops accepting, sends close frames to every peer, waits briefly
	// (SERVER_CLOSE_GRACE) for the close handshakes, closes whatever is left and
	// stops the loop. Safe from any thread.
	void stop()
	{
		bool running;
		{
			std::lock_guard<std::mutex> lock(mu_);
			stopping_ = true;
			running = loopRunning_;
			if (running) {
				asio::post(*holder_->io, [this]() {
					beginStop();
				});
			}
		}
		if (!running) {
			closeAllNow();
			drainIoContext(*holder_->io);
		}
	}

private:
	bool stopping()
	{
		std::lock_guard<std::mutex> lock(mu_);
		return stopping_;
	}

	void onOpen(websocketpp::connection_hdl hdl)
	{
		websocketpp::lib::error_code ec;
		auto con = holder_->endpoint.get_con_from_hdl(hdl, ec);
		if (ec || !con) {
			return;
		}
		if (stopping()) {
			holder_->endpoint.close(hdl, websocketpp::close::status::going_away, "server shutting down", ec);
			return;
		}
		SCG_LOG_INFO("WebSocket server accepted new connection");
		auto conn = std::make_shared<Conn>(holder_, maxSendMessageSize_, maxWriteQueueBytes_);
		conn->attach(con, false);
		conns_.add(conn);
		if (onConnection_) {
			onConnection_(conn);
		}
	}

	void beginStop()
	{
		websocketpp::lib::error_code ec;
		holder_->endpoint.stop_listening(ec);
		for (auto& conn : conns_.live()) {
			conn->closeGoingAway();
		}
		stopDeadline_ = std::chrono::steady_clock::now() + SERVER_CLOSE_GRACE;
		pollStop();
	}

	void pollStop()
	{
		bool allDone = true;
		for (auto& conn : conns_.live()) {
			if (!conn->terminated()) {
				allDone = false;
				break;
			}
		}
		if (allDone || std::chrono::steady_clock::now() >= stopDeadline_) {
			closeAllNow();
			holder_->io->stop();
			return;
		}
		stopTimer_.expires_after(std::chrono::milliseconds(5));
		stopTimer_.async_wait([this](const std::error_code& ec) {
			if (!ec) {
				pollStop();
			}
		});
	}

	void closeAllNow()
	{
		websocketpp::lib::error_code ec;
		holder_->endpoint.stop_listening(ec);
		stopTimer_.cancel();
		for (auto& conn : conns_.live()) {
			conn->closeNow();
		}
	}

	std::shared_ptr<Holder> holder_;
	asio::steady_timer stopTimer_;
	std::chrono::steady_clock::time_point stopDeadline_;
	ConnectionList<Conn> conns_; // loop thread only
	std::function<void(std::shared_ptr<scg::rpc::Connection>)> onConnection_;
	uint32_t maxSendMessageSize_;
	size_t maxWriteQueueBytes_;

	std::mutex mu_;
	bool loopRunning_ = false;
	bool stopping_ = false;
};

} // namespace detail
} // namespace ws
} // namespace scg
