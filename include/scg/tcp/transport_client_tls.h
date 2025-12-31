#pragma once

#define ASIO_STANDALONE
#include <asio.hpp>
#include <asio/ssl.hpp>

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

struct ClientTransportTLSConfig {
	std::string host;
	int port;
	bool verifyPeer;
	std::string caFile;
	uint32_t maxSendMessageSize = 0; // 0 for no limit
	uint32_t maxRecvMessageSize = 0; // 0 for no limit
};

class ConnectionTLS : public scg::rpc::Connection, public std::enable_shared_from_this<ConnectionTLS> {
public:
	ConnectionTLS(asio::ssl::stream<asio::ip::tcp::socket> socket, uint32_t maxSendMessageSize = 0, uint32_t maxRecvMessageSize = 0)
		: socket_(std::move(socket))
		, closed_(false)
		, maxSendMessageSize_(maxSendMessageSize)
		, maxRecvMessageSize_(maxRecvMessageSize)
	{
		socket_.lowest_layer().set_option(asio::ip::tcp::no_delay(true));
		SCG_LOG_INFO("TCP TLS connection established");
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
			SCG_LOG_INFO("TCP TLS connection closing");
			closed_ = true;
			auto self = shared_from_this();
			asio::post(socket_.get_executor(), [this, self]() {
				if (socket_.lowest_layer().is_open()) {
					socket_.lowest_layer().close();
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
					if (failHandler_) {
						failHandler_(error::Error(ec.message()));
					}
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
						if (failHandler_) failHandler_(error::Error("Message size exceeds receive limit"));
						close();
						return;
					}
					read_body(len);
				} else {
					if (ec != asio::error::eof && failHandler_) failHandler_(error::Error(ec.message()));
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
					if (failHandler_) {
						failHandler_(error::Error(ec.message()));
					}
					close();
				}
			});
	}

	asio::ssl::stream<asio::ip::tcp::socket> socket_;
	std::function<void(const std::vector<uint8_t>&)> messageHandler_;
	std::function<void(const error::Error&)> failHandler_;
	std::function<void()> closeHandler_;
	std::atomic<bool> closed_;
	std::deque<std::vector<uint8_t>> write_queue_;
	uint8_t read_buffer_[4];
	std::vector<uint8_t> body_buffer_;
	uint32_t maxSendMessageSize_;
	uint32_t maxRecvMessageSize_;
};


class ClientTransportTCPTLS : public scg::rpc::ClientTransport
{
public:
	ClientTransportTCPTLS(const ClientTransportTLSConfig& config)
		: config_(config)
		, ssl_context_(asio::ssl::context::tls_client)
		, work_guard_(asio::make_work_guard(io_context_))
	{

		if (!config_.verifyPeer) {
			ssl_context_.set_verify_mode(asio::ssl::verify_none);
		} else {
			ssl_context_.set_verify_mode(asio::ssl::verify_peer);
			if (!config_.caFile.empty()) {
				ssl_context_.load_verify_file(config_.caFile);
			} else {
				ssl_context_.set_default_verify_paths();
			}
		}

		thread_ = std::thread([this]() {
			io_context_.run();
		});
	}

	~ClientTransportTCPTLS()
	{
		shutdown();
	}

	std::pair<std::shared_ptr<scg::rpc::Connection>, error::Error> connect() override
	{
		try {
			SCG_LOG_INFO("Connecting to TCP TLS server at " + config_.host + ":" + std::to_string(config_.port));
			asio::ip::tcp::resolver resolver(io_context_);
			auto endpoints = resolver.resolve(config_.host, std::to_string(config_.port));

			asio::ssl::stream<asio::ip::tcp::socket> socket(io_context_, ssl_context_);

			if (config_.verifyPeer) {
				 socket.set_verify_callback(asio::ssl::host_name_verification(config_.host));
			}

			asio::connect(socket.lowest_layer(), endpoints);
			socket.handshake(asio::ssl::stream_base::client);

			return {std::make_shared<ConnectionTLS>(std::move(socket), config_.maxSendMessageSize, config_.maxRecvMessageSize), nullptr};
		} catch (const std::exception& e) {
			SCG_LOG_ERROR("TCP TLS connection failed: " + std::string(e.what()));
			return {nullptr, error::Error(e.what())};
		}
	}

	void shutdown() override
	{
		SCG_LOG_INFO("Shutting down TCP TLS client transport");
		io_context_.stop();
		if (thread_.joinable()) {
			thread_.join();
		}
	}

private:
	ClientTransportTLSConfig config_;
	asio::io_context io_context_;
	asio::ssl::context ssl_context_;
	asio::executor_work_guard<asio::io_context::executor_type> work_guard_;
	std::thread thread_;
};

} // namespace tcp
} // namespace scg
