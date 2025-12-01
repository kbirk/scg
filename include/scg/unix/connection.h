#pragma once

#define ASIO_STANDALONE 1
#include <asio.hpp>
#include <memory>
#include <vector>
#include <functional>
#include <mutex>
#include <deque>
#include <cstring>
#include <arpa/inet.h>

#include "scg/transport.h"

namespace scg {
namespace unix_socket {

struct UnixContext {
	asio::io_context io_context;
	asio::executor_work_guard<asio::io_context::executor_type> work_guard;

	UnixContext() : work_guard(asio::make_work_guard(io_context)) {}
};

class UnixConnection : public rpc::Connection, public std::enable_shared_from_this<UnixConnection> {
public:
	UnixConnection(asio::local::stream_protocol::socket socket)
		: socket_(std::move(socket)), closed_(false) {}

	~UnixConnection() = default;

	void start() {
		readHeader();
	}

	error::Error send(const std::vector<uint8_t>& data) override {
		if (!socket_.is_open()) {
			return error::Error("Connection is closed");
		}

		// Prepare message with length prefix
		uint32_t length = static_cast<uint32_t>(data.size());
		uint32_t net_length = htonl(length); // Convert to network byte order (Big Endian)

		std::vector<uint8_t> buffer;
		buffer.reserve(4 + length);

		const uint8_t* len_ptr = reinterpret_cast<const uint8_t*>(&net_length);
		buffer.insert(buffer.end(), len_ptr, len_ptr + 4);
		buffer.insert(buffer.end(), data.begin(), data.end());

		std::lock_guard<std::mutex> lock(write_mutex_);
		write_queue_.push_back(std::move(buffer));

		if (write_queue_.size() == 1) {
			doWriteLocked();
		}

		return nullptr;
	}

	void setMessageHandler(std::function<void(const std::vector<uint8_t>&)> handler) override {
		messageHandler_ = handler;
	}

	void setFailHandler(std::function<void(const error::Error&)> handler) override {
		failHandler_ = handler;
	}

	void setCloseHandler(std::function<void()> handler) override {
		closeHandler_ = handler;
	}

	error::Error close() override {
		asio::post(socket_.get_executor(), [self = shared_from_this()]() {
			self->doClose();
		});
		return nullptr;
	}

private:
	void readHeader() {
		auto self = shared_from_this();
		asio::async_read(socket_, asio::buffer(read_header_, 4),
			[this, self](std::error_code ec, std::size_t /*length*/) {
				if (!ec) {
					uint32_t net_len;
					std::memcpy(&net_len, read_header_, 4);
					uint32_t body_len = ntohl(net_len);
					readBody(body_len);
				} else {
					handleError(ec);
				}
			});
	}

	void readBody(uint32_t length) {
		read_buffer_.resize(length);
		auto self = shared_from_this();
		asio::async_read(socket_, asio::buffer(read_buffer_),
			[this, self](std::error_code ec, std::size_t /*length*/) {
				if (!ec) {
					if (messageHandler_) {
						messageHandler_(read_buffer_);
					}
					readHeader();
				} else {
					handleError(ec);
				}
			});
	}

	void doWriteLocked() {
		auto self = shared_from_this();
		asio::async_write(socket_, asio::buffer(write_queue_.front()),
			[this, self](std::error_code ec, std::size_t /*length*/) {
				if (!ec) {
					std::lock_guard<std::mutex> lock(write_mutex_);
					write_queue_.pop_front();
					if (!write_queue_.empty()) {
						doWriteLocked();
					}
				} else {
					handleError(ec);
				}
			});
	}

	void doClose() {
		if (closed_) return;
		closed_ = true;

		std::error_code ec;
		socket_.close(ec);
		if (closeHandler_) {
			closeHandler_();
		}
	}

	void handleError(const std::error_code& ec) {
		if (ec == asio::error::operation_aborted) return;

		if (failHandler_) {
			failHandler_(error::Error(ec.message()));
		}
		doClose();
	}

	asio::local::stream_protocol::socket socket_;
	bool closed_;
	uint8_t read_header_[4];
	std::vector<uint8_t> read_buffer_;

	std::deque<std::vector<uint8_t>> write_queue_;
	std::mutex write_mutex_;

	std::function<void(const std::vector<uint8_t>&)> messageHandler_;
	std::function<void(const error::Error&)> failHandler_;
	std::function<void()> closeHandler_;
};

} // namespace unix_socket
} // namespace scg
