#pragma once

#define ASIO_STANDALONE 1
#include <asio.hpp>
#include <memory>
#include <mutex>
#include <queue>
#include <cstdio>

#include "scg/transport.h"
#include "scg/unix/connection.h"

namespace scg {
namespace unix_socket {

struct ServerTransportConfig {
	std::string socketPath = "/tmp/scg.sock";
};

class ServerTransportUnix : public rpc::ServerTransport {
public:
	ServerTransportUnix(const ServerTransportConfig& config)
		: config_(config) {
		ctx_ = std::make_shared<UnixContext>();
		acceptor_ = std::make_unique<asio::local::stream_protocol::acceptor>(ctx_->io_context);
	}

	~ServerTransportUnix() {
		close();
	}

	error::Error listen() override {
		if (acceptor_->is_open()) {
			return error::Error("Server is already listening");
		}

		try {
			// Remove existing socket file if it exists
			std::remove(config_.socketPath.c_str());

			asio::local::stream_protocol::endpoint endpoint(config_.socketPath);
			acceptor_->open(endpoint.protocol());
			acceptor_->bind(endpoint);
			acceptor_->listen();

			startAccept();
		} catch (const std::exception& e) {
			return error::Error("Failed to listen: " + std::string(e.what()));
		}

		return nullptr;
	}

	std::pair<std::shared_ptr<rpc::Connection>, error::Error> accept() override {
		// Drive the IO context to process events
		ctx_->io_context.poll();

		// Reset the io_context so it can be polled again if it ran out of work
		if (ctx_->io_context.stopped()) {
			ctx_->io_context.restart();
		}

		std::lock_guard<std::mutex> lock(mutex_);
		if (pending_connections_.empty()) {
			return {nullptr, nullptr};
		}

		auto conn = pending_connections_.front();
		pending_connections_.pop();
		return {conn, nullptr};
	}

	error::Error close() override {
		if (acceptor_ && acceptor_->is_open()) {
			acceptor_->close();
		}
		// Clean up socket file
		std::remove(config_.socketPath.c_str());
		return nullptr;
	}

private:
	void startAccept() {
		auto socket = std::make_shared<asio::local::stream_protocol::socket>(ctx_->io_context);
		acceptor_->async_accept(*socket, [this, socket](const std::error_code& ec) {
			if (!ec) {
				auto conn = std::make_shared<UnixConnection>(std::move(*socket));
				conn->start();

				{
					std::lock_guard<std::mutex> lock(mutex_);
					pending_connections_.push(conn);
				}
			}

			if (acceptor_->is_open()) {
				startAccept();
			}
		});
	}

	ServerTransportConfig config_;
	std::shared_ptr<UnixContext> ctx_;
	std::unique_ptr<asio::local::stream_protocol::acceptor> acceptor_;

	std::queue<std::shared_ptr<UnixConnection>> pending_connections_;
	std::mutex mutex_;
};

} // namespace unix_socket
} // namespace scg
