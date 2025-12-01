#pragma once

#define ASIO_STANDALONE 1
#include <asio.hpp>
#include <thread>
#include <memory>
#include <future>

#include "scg/unix/connection.h"

namespace scg {
namespace unix_socket {

struct ClientTransportConfig {
	std::string socketPath = "/tmp/scg.sock";
};

class ClientTransportUnix : public rpc::ClientTransport {
public:
	ClientTransportUnix(const ClientTransportConfig& config)
		: config_(config) {

		ctx_ = std::make_shared<UnixContext>();
		thread_ = std::thread([this]() {
			ctx_->io_context.run();
		});
	}

	~ClientTransportUnix() {
		shutdown();
	}

	std::pair<std::shared_ptr<rpc::Connection>, error::Error> connect() override {
		auto socket = std::make_shared<asio::local::stream_protocol::socket>(ctx_->io_context);

		std::promise<error::Error> promise;
		auto future = promise.get_future();

		asio::local::stream_protocol::endpoint endpoint(config_.socketPath);

		socket->async_connect(endpoint,
			[&promise](const std::error_code& ec) {
				if (!ec) {
					promise.set_value(nullptr);
				} else {
					promise.set_value(error::Error(ec.message()));
				}
			});

		auto err = future.get();
		if (err) {
			return {nullptr, err};
		}

		auto conn = std::make_shared<UnixConnection>(std::move(*socket));
		conn->start();
		return {conn, nullptr};
	}

	void shutdown() override {
		if (ctx_ && !ctx_->io_context.stopped()) {
			ctx_->work_guard.reset();
			ctx_->io_context.stop();
		}
		if (thread_.joinable()) {
			thread_.join();
		}
	}

private:
	ClientTransportConfig config_;
	std::shared_ptr<UnixContext> ctx_;
	std::thread thread_;
};

} // namespace unix_socket
} // namespace scg
