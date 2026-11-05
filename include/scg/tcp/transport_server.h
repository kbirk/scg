#pragma once

#include "scg/tcp/connection.h"
#include "scg/tcp/transport_client.h"

#include <algorithm>
#include <chrono>
#include <memory>
#include <mutex>
#include <string>
#include <thread>
#include <vector>

namespace scg {
namespace tcp {

struct ServerTransportConfig {
	int port = 0;
	uint32_t maxSendMessageSize = 0; // 0 for no limit
	uint32_t maxRecvMessageSize = scg::rpc::DEFAULT_MAX_RECV_MESSAGE_SIZE; // 0 disables the cap
	size_t maxWriteQueueBytes = DEFAULT_MAX_WRITE_QUEUE_BYTES; // 0 disables the cap
};

namespace detail {

// ServerLoopState coordinates runEventLoop() (on the server's transport thread)
// with stop() (from any thread). The transport must be owned by a shared_ptr.
class ServerLoopState {
public:
	// beginRun marks the loop running; returns false (and leaves it not
	// running) if the transport was already stopped before the loop started.
	// Checked under the same mutex as markStopping(), so stop() either sees the
	// loop running (and posts its close onto it) or the loop never runs.
	bool beginRun(asio::io_context& io)
	{
		std::lock_guard<std::mutex> lock(mu_);
		if (stopping_) {
			return false;
		}
		if (io.stopped()) {
			io.restart();
		}
		running_ = true;
		return true;
	}

	void endRun()
	{
		std::lock_guard<std::mutex> lock(mu_);
		running_ = false;
	}

	// markStopping returns whether the loop is currently running (in which case
	// the caller must post the close onto the loop).
	bool markStopping()
	{
		std::lock_guard<std::mutex> lock(mu_);
		stopping_ = true;
		return running_;
	}

	void clearStopping()
	{
		std::lock_guard<std::mutex> lock(mu_);
		stopping_ = false;
	}

	bool stopping()
	{
		std::lock_guard<std::mutex> lock(mu_);
		return stopping_;
	}

private:
	std::mutex mu_;
	bool running_ = false;
	bool stopping_ = false;
};

// AcceptBackoff spaces out accept retries after errors such as EMFILE so a
// persistent failure does not spin the I/O thread.
class AcceptBackoff {
public:
	std::chrono::milliseconds next()
	{
		delay_ = delay_.count() == 0 ? std::chrono::milliseconds(10) : (std::min)(delay_ * 2, std::chrono::milliseconds(1000));
		return delay_;
	}
	void reset() { delay_ = std::chrono::milliseconds(0); }

private:
	std::chrono::milliseconds delay_{0};
};

} // namespace detail

class ServerTransportTCP : public scg::rpc::ServerTransport, public std::enable_shared_from_this<ServerTransportTCP> {
public:
	ServerTransportTCP(const ServerTransportConfig& config)
		: config_(config)
		, io_(std::make_shared<asio::io_context>())
		, acceptor_(*io_)
		, retryTimer_(*io_)
	{
	}

	~ServerTransportTCP()
	{
		// No handler can reach `this` any more (they hold weak references).
		// Close everything and run the aborted completions so nothing queued
		// keeps a connection alive.
		bool running = state_.markStopping();
		if (!running) {
			closeAll();
			detail::drainIoContext(*io_);
		} else if (io_->get_executor().running_in_this_thread()) {
			closeAll();
			io_->stop();
		} else {
			io_->stop();
		}
	}

	void setOnConnection(std::function<void(std::shared_ptr<scg::rpc::Connection>)> handler) override
	{
		onConnectionHandler_ = handler;
	}

	error::Error startListening() override
	{
		if (weak_from_this().expired()) {
			return error::Error("ServerTransportTCP must be owned by a std::shared_ptr");
		}
		try {
			SCG_LOG_INFO("TCP server listening on port " + std::to_string(config_.port));
			asio::ip::tcp::endpoint endpoint(asio::ip::tcp::v4(), static_cast<asio::ip::port_type>(config_.port));
			acceptor_.open(endpoint.protocol());
			acceptor_.set_option(asio::ip::tcp::acceptor::reuse_address(true));
			acceptor_.bind(endpoint);
			acceptor_.listen();

			state_.clearStopping();
			startAccept();

			return nullptr;
		} catch (const std::exception& e) {
			SCG_LOG_ERROR("TCP server failed to start: " + std::string(e.what()));
			asio::error_code ec;
			acceptor_.close(ec);
			return error::Error(e.what());
		}
	}

	void runEventLoop() override
	{
		// Single io thread: a latency-optimized event loop. Unary handlers run on
		// the server's worker pool and stream handlers on their own threads, so
		// this thread only does I/O.
		if (!state_.beginRun(*io_)) {
			return; // stopped before the loop started
		}
		io_->run();
		// stop() closed every socket; run the resulting completions here, on the
		// loop thread, so connection callbacks fire and release their state.
		detail::drainIoContext(*io_);
		state_.endRun();
	}

	// stop closes the listener and every accepted connection, then stops the
	// loop. Safe from any thread.
	void stop() override
	{
		SCG_LOG_INFO("Stopping TCP server");
		if (state_.markStopping()) {
			auto self = shared_from_this();
			asio::post(*io_, [self]() {
				self->closeAll();
				self->io_->stop();
			});
			return;
		}
		// No loop is running: nothing else touches the sockets.
		closeAll();
		detail::drainIoContext(*io_);
	}

private:
	void closeAll()
	{
		asio::error_code ec;
		acceptor_.close(ec);
		retryTimer_.cancel();
		for (auto& conn : registry_.snapshot()) {
			conn->closeNow();
		}
	}

	void startAccept()
	{
		std::weak_ptr<ServerTransportTCP> weak = shared_from_this();
		acceptor_.async_accept([weak](const std::error_code& error, asio::ip::tcp::socket socket) {
			auto self = weak.lock();
			if (!self) {
				return;
			}
			self->onAccept(error, std::move(socket));
		});
	}

	void onAccept(const std::error_code& error, asio::ip::tcp::socket socket)
	{
		if (error) {
			if (error == asio::error::operation_aborted || state_.stopping()) {
				return; // clean shutdown
			}
			// e.g. EMFILE: back off instead of spinning on immediate failures.
			SCG_LOG_ERROR("TCP server accept error: " + error.message());
			std::weak_ptr<ServerTransportTCP> weak = shared_from_this();
			retryTimer_.expires_after(backoff_.next());
			retryTimer_.async_wait([weak](const std::error_code& ec) {
				auto self = weak.lock();
				if (ec || !self) {
					return;
				}
				self->startAccept();
			});
			return;
		}
		backoff_.reset();
		if (state_.stopping()) {
			asio::error_code ec;
			socket.close(ec);
			return;
		}
		SCG_LOG_INFO("TCP server accepted new connection");
		auto conn = std::make_shared<ConnectionTCP>(std::move(socket), config_.maxSendMessageSize, config_.maxRecvMessageSize, io_, config_.maxWriteQueueBytes);
		registry_.add(conn);
		if (onConnectionHandler_) {
			onConnectionHandler_(conn);
		}
		startAccept();
	}

	ServerTransportConfig config_;
	// Declared before every I/O object so it is destroyed after them.
	std::shared_ptr<asio::io_context> io_;
	asio::ip::tcp::acceptor acceptor_;
	asio::steady_timer retryTimer_;
	detail::AcceptBackoff backoff_;
	detail::ServerLoopState state_;
	detail::ConnectionRegistry<ConnectionTCP> registry_;
	std::function<void(std::shared_ptr<scg::rpc::Connection>)> onConnectionHandler_;
};

} // namespace tcp
} // namespace scg
