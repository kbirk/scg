#pragma once

#define ASIO_STANDALONE
#include <asio.hpp>

#include "scg/transport.h"
#include "scg/error.h"
#include "scg/logger.h"
#include <memory>
#include <string>
#include <thread>
#include <mutex>
#include <atomic>
#include <vector>
#include <deque>
#include <iostream>

namespace scg {
namespace tcp {

struct ClientTransportConfig {
	std::string host;
	int port;
	uint32_t maxSendMessageSize = 0; // 0 for no limit
	uint32_t maxRecvMessageSize = 0; // 0 for no limit
	log::LoggingConfig logging;
};

class ConnectionTCP : public scg::rpc::Connection, public std::enable_shared_from_this<ConnectionTCP> {
public:
	ConnectionTCP(asio::ip::tcp::socket socket, uint32_t maxSendMessageSize = 0, uint32_t maxRecvMessageSize = 0, log::LoggingConfig logging = {})
		: socket_(std::move(socket))
		, closed_(false)
		, maxSendMessageSize_(maxSendMessageSize)
		, maxRecvMessageSize_(maxRecvMessageSize)
		, logging_(logging)
	{
		socket_.set_option(asio::ip::tcp::no_delay(true));
		log(log::LogLevel::INFO, "Connection established");
	}

	error::Error send(const std::vector<uint8_t>& data) override
	{
		if (closed_) return error::Error("Connection closed");

		uint32_t len = static_cast<uint32_t>(data.size());

		if (maxSendMessageSize_ > 0 && len > maxSendMessageSize_) {
			return error::Error("Message size exceeds send limit");
		}

		std::vector<uint8_t> buffer;
		buffer.reserve(4 + len);
		// Big endian length prefix
		buffer.push_back((len >> 24) & 0xFF);
		buffer.push_back((len >> 16) & 0xFF);
		buffer.push_back((len >> 8) & 0xFF);
		buffer.push_back(len & 0xFF);
		buffer.insert(buffer.end(), data.begin(), data.end());

		auto self = shared_from_this();
		asio::post(socket_.get_executor(), [this, self, buffer = std::move(buffer)]() {
			bool write_in_progress = !write_queue_.empty();
			write_queue_.push_back(std::move(buffer));
			if (!write_in_progress) {
				do_write();
			}
		});

		return nullptr;
	}

	void setMessageHandler(std::function<void(const std::vector<uint8_t>&)> handler) override
	{
		messageHandler_ = handler;
		read_header();
	}

	void setFailHandler(std::function<void(const error::Error&)> handler) override
	{
		failHandler_ = handler;
	}

	void setCloseHandler(std::function<void()> handler) override
	{
		closeHandler_ = handler;
	}

	error::Error close() override
	{
		if (!closed_) {
			log(log::LogLevel::INFO, "Closing connection");
			closed_ = true;
			auto self = shared_from_this();
			asio::post(socket_.get_executor(), [this, self]() {
				if (socket_.is_open()) {
					socket_.close();
				}
				if (closeHandler_) closeHandler_();

				// Break potential reference cycles
				messageHandler_ = nullptr;
				failHandler_ = nullptr;
				closeHandler_ = nullptr;
				write_queue_.clear();
			});
		}
		return nullptr;
	}

private:
	void do_write()
	{
		auto self = shared_from_this();
		asio::async_write(socket_, asio::buffer(write_queue_.front()),
			[this, self](std::error_code ec, std::size_t /*length*/) {
				if (!ec) {
					write_queue_.pop_front();
					if (!write_queue_.empty()) {
						do_write();
					}
				} else {
					log(log::LogLevel::ERROR, "Write error: " + ec.message());
					if (failHandler_) failHandler_(error::Error(ec.message()));
					close();
				}
			});
	}

	void read_header()
	{
		auto self = shared_from_this();
		asio::async_read(socket_, asio::buffer(read_buffer_, 4),
			[this, self](std::error_code ec, std::size_t /*length*/) {
				if (!ec) {
					uint32_t len = (read_buffer_[0] << 24) | (read_buffer_[1] << 16) | (read_buffer_[2] << 8) | read_buffer_[3];
					if (maxRecvMessageSize_ > 0 && len > maxRecvMessageSize_) {
						log(log::LogLevel::ERROR, "Message size exceeds receive limit");
						if (failHandler_) failHandler_(error::Error("Message size exceeds receive limit"));
						close();
						return;
					}
					read_body(len);
				} else {
					if (ec != asio::error::eof) {
						log(log::LogLevel::ERROR, "Read header error: " + ec.message());
						if (failHandler_) failHandler_(error::Error(ec.message()));
					}
					close();
				}
			});
	}

	void read_body(uint32_t length)
	{
		auto self = shared_from_this();
		body_buffer_.resize(length);
		asio::async_read(socket_, asio::buffer(body_buffer_),
			[this, self](std::error_code ec, std::size_t /*length*/) {
				if (!ec) {
					if (messageHandler_) {
						messageHandler_(body_buffer_);
					}
					read_header();
				} else {
					log(log::LogLevel::ERROR, "Read body error: " + ec.message());
					if (failHandler_) {
						failHandler_(error::Error(ec.message()));
					}
					close();
				}
			});
	}

	asio::ip::tcp::socket socket_;
	std::function<void(const std::vector<uint8_t>&)> messageHandler_;
	std::function<void(const error::Error&)> failHandler_;
	std::function<void()> closeHandler_;
	std::atomic<bool> closed_;
	std::deque<std::vector<uint8_t>> write_queue_;
	uint8_t read_buffer_[4];
	std::vector<uint8_t> body_buffer_;
	uint32_t maxSendMessageSize_;
	uint32_t maxRecvMessageSize_;
	log::LoggingConfig logging_;

	void log(log::LogLevel level, const std::string& msg) {
		if (level < logging_.level) return;
		switch (level) {
			case log::LogLevel::DEBUG: if (logging_.debugLogger) logging_.debugLogger(msg); break;
			case log::LogLevel::INFO: if (logging_.infoLogger) logging_.infoLogger(msg); break;
			case log::LogLevel::WARN: if (logging_.warnLogger) logging_.warnLogger(msg); break;
			case log::LogLevel::ERROR: if (logging_.errorLogger) logging_.errorLogger(msg); break;
			default: break;
		}
	}
};

class ClientTransportTCP : public scg::rpc::ClientTransport
{
public:
	ClientTransportTCP(const ClientTransportConfig& config)
		: config_(config)
		, work_guard_(asio::make_work_guard(io_context_))
	{
		thread_ = std::thread([this]() {
			io_context_.run();
		});
	}

	~ClientTransportTCP()
	{
		shutdown();
	}

	std::pair<std::shared_ptr<scg::rpc::Connection>, error::Error> connect() override
	{
		try {
			asio::ip::tcp::resolver resolver(io_context_);
			auto endpoints = resolver.resolve(config_.host, std::to_string(config_.port));
			asio::ip::tcp::socket socket(io_context_);
			asio::connect(socket, endpoints);
			return {std::make_shared<ConnectionTCP>(std::move(socket), config_.maxSendMessageSize, config_.maxRecvMessageSize, config_.logging), nullptr};
		} catch (const std::exception& e) {
			if (config_.logging.errorLogger) config_.logging.errorLogger("Connect failed: " + std::string(e.what()));
			return {nullptr, error::Error(e.what())};
		}
	}

	void shutdown() override
	{
		io_context_.stop();
		if (thread_.joinable()) {
			thread_.join();
		}
	}

private:
	ClientTransportConfig config_;
	asio::io_context io_context_;
	asio::executor_work_guard<asio::io_context::executor_type> work_guard_;
	std::thread thread_;
};

} // namespace tcp
} // namespace scg
