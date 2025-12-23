#pragma once

#include <cstdint>
#include <functional>
#include <future>
#include <iostream>
#include <map>
#include <memory>
#include <mutex>
#include <random>
#include <vector>

#include "scg/const.h"
#include "scg/context.h"
#include "scg/error.h"
#include "scg/reader.h"
#include "scg/serialize.h"
#include "scg/transport.h"
#include "scg/writer.h"

namespace scg {
namespace rpc {

// Forward declaration
class Stream;

// Message processor interface for handling incoming stream messages
class MessageProcessor {
public:
	virtual ~MessageProcessor() = default;

	// Process an incoming message and return a response
	// Returns (response bytes, error) - response bytes contain the serialized response message
	virtual std::pair<std::vector<uint8_t>, error::Error> processMessage(uint64_t methodID, serialize::Reader& reader) = 0;
};

using StreamErrorHandler = std::function<void(const error::Error&)>;

class Stream {
public:
	Stream(uint64_t streamID, uint64_t serviceID, std::shared_ptr<Connection> connection, StreamErrorHandler errorHandler)
		: streamID_(streamID)
		, serviceID_(serviceID)
		, connection_(connection)
		, errorHandler_(errorHandler)
		, closed_(false)
	{
		// Initialize request ID with randomness
		std::random_device rd;
		std::mt19937_64 gen(rd());
		std::uniform_int_distribution<uint64_t> dis;
		requestID_ = dis(gen);
	}

	virtual ~Stream() {
		close();
	}

	uint64_t id() const {
		return streamID_;
	}

	// Send a message on the stream and wait for response
	template <typename T>
	std::pair<serialize::Reader, error::Error> sendMessage(const context::Context& ctx, uint64_t methodID, const T& msg) {
		std::cerr << "[ClientStream] sendMessage called, methodID=" << methodID << std::endl;
		std::lock_guard<std::mutex> lock(mu_);

		if (closed_) {
			return std::make_pair(serialize::Reader({}), error::Error("Stream is closed"));
		}

		uint64_t requestID = requestID_++;

		using scg::serialize::bit_size;

		serialize::FixedSizeWriter writer(
			scg::serialize::bits_to_bytes(
				bit_size(STREAM_MESSAGE_PREFIX) +
				bit_size(ctx) +
				bit_size(streamID_) +
				bit_size(requestID) +
				bit_size(methodID) +
				bit_size(msg)));

		writer.write(STREAM_MESSAGE_PREFIX);
		writer.write(ctx);
		writer.write(streamID_);
		writer.write(requestID);
		writer.write(methodID);
		writer.write(msg);

		auto promise = std::make_shared<std::promise<serialize::Reader>>();
		requests_[requestID] = promise;

		std::cerr << "[ClientStream] Sending " << writer.bytes().size() << " bytes" << std::endl;
		auto err = connection_->send(writer.bytes());
		if (err) {
			std::cerr << "[ClientStream] ERROR: send failed: " << err.message << std::endl;
			requests_.erase(requestID);
			return std::make_pair(serialize::Reader({}), err);
		}

		std::cerr << "[ClientStream] Sent, waiting for response..." << std::endl;
		auto future = promise->get_future();

		// Release lock before waiting
		mu_.unlock();

		if (ctx.hasDeadline()) {
			auto status = future.wait_until(ctx.getDeadline());
			if (status == std::future_status::timeout) {
				mu_.lock();
				requests_.erase(requestID);
				return std::make_pair(serialize::Reader({}), error::Error("Stream message timed out"));
			}
		}

		auto reader = future.get();

		// Reacquire lock
		mu_.lock();

		uint8_t responseType = 0;
		serialize::deserialize(responseType, reader);

		if (responseType == MESSAGE_RESPONSE) {
			return std::make_pair(reader, nullptr);
		}

		std::string errMsg;
		serialize::deserialize(errMsg, reader);

		if (errMsg.empty()) {
			errMsg = "Unknown error";
		}
		return std::make_pair(serialize::Reader({}), error::Error(errMsg));
	}

	// Handle incoming message response
	void handleMessageResponse(uint64_t requestID, serialize::Reader& reader) {
		std::lock_guard<std::mutex> lock(mu_);

		auto iter = requests_.find(requestID);
		if (iter != requests_.end()) {
			iter->second->set_value(reader);
			requests_.erase(requestID);
		}
	}

	// Handle incoming unsolicited message from peer
	void handleIncomingMessage(uint64_t methodID, uint64_t requestID, serialize::Reader& reader) {
		std::cerr << "[Stream] handleIncomingMessage called, methodID=" << methodID << ", requestID=" << requestID << std::endl;
		MessageProcessor* proc = nullptr;
		{
			std::lock_guard<std::mutex> lock(mu_);
			proc = processor_;
		}

		if (!proc) {
			std::cerr << "[Stream] ERROR: No processor registered!" << std::endl;
			// No processor registered - send error response
			auto response = respondWithStreamError(requestID, error::Error("No message processor registered"));
			connection_->send(response);
			return;
		}

		std::cerr << "[Stream] Calling processor->processMessage" << std::endl;
		// Call processor to handle the message (outside lock)
		auto [responseBytes, err] = proc->processMessage(methodID, reader);

		if (err) {
			std::cerr << "[Stream] Processor returned error: " << err.message << std::endl;
			// Send error response
			auto response = respondWithStreamError(requestID, err);
			connection_->send(response);
		} else {
			std::cerr << "[Stream] Processor succeeded, sending response" << std::endl;
			// Send success response
			auto response = respondWithStreamMessage(requestID, responseBytes);
			connection_->send(response);
		}
	}

	// Handle stream close from remote
	void handleClose() {
		std::lock_guard<std::mutex> lock(mu_);

		if (!closed_) {
			closed_ = true;
			closedPromise_.set_value();

			// Fail all pending requests
			for (auto& pair : requests_) {
				serialize::FixedSizeWriter errorWriter(
					scg::serialize::bits_to_bytes(
						scg::serialize::bit_size(ERROR_RESPONSE) +
						scg::serialize::bit_size(std::string("Stream closed"))));
				errorWriter.write(ERROR_RESPONSE);
				errorWriter.write(std::string("Stream closed"));
				pair.second->set_value(serialize::Reader(errorWriter.bytes()));
			}
			requests_.clear();
		}
	}

	// Close the stream locally
	error::Error close() {
		std::lock_guard<std::mutex> lock(mu_);

		if (closed_) {
			return nullptr;
		}

		closed_ = true;
		closedPromise_.set_value();

		// Send close message
		using scg::serialize::bit_size;

		serialize::FixedSizeWriter writer(
			scg::serialize::bits_to_bytes(
				bit_size(STREAM_CLOSE_PREFIX) +
				bit_size(streamID_)));

		writer.write(STREAM_CLOSE_PREFIX);
		writer.write(streamID_);

		auto err = connection_->send(writer.bytes());

		// Fail all pending requests
		for (auto& pair : requests_) {
			serialize::FixedSizeWriter errorWriter(
				scg::serialize::bits_to_bytes(
					scg::serialize::bit_size(ERROR_RESPONSE) +
					scg::serialize::bit_size(std::string("Stream closed"))));
			errorWriter.write(ERROR_RESPONSE);
			errorWriter.write(std::string("Stream closed"));
			pair.second->set_value(serialize::Reader(errorWriter.bytes()));
		}
		requests_.clear();

		return err;
	}

	// Wait for stream to close
	std::future<void> wait() {
		return closedPromise_.get_future();
	}

	bool isClosed() const {
		std::lock_guard<std::mutex> lock(mu_);
		return closed_;
	}

	// Set message processor for handling incoming messages
	void setProcessor(MessageProcessor* processor) {
		std::lock_guard<std::mutex> lock(mu_);
		processor_ = processor;
	}

	// Store user data (for keeping objects alive)
	template<typename T>
	void setUserData(std::shared_ptr<T> data) {
		std::lock_guard<std::mutex> lock(mu_);
		userData_ = data;
	}

	// Get the context (for server-side handlers)
	const context::Context& context() const {
		return ctx_;
	}

private:
	// Create error response for stream messages
	std::vector<uint8_t> respondWithStreamError(uint64_t requestID, const error::Error& err) {
		using scg::serialize::bit_size;

		std::string errMsg = err ? err.message : "Unknown error";

		size_t bitSize = bit_size(STREAM_RESPONSE_PREFIX) +
						 bit_size(streamID_) +
						 bit_size(requestID) +
						 bit_size(ERROR_RESPONSE) +
						 bit_size(errMsg);

		serialize::FixedSizeWriter writer(serialize::bits_to_bytes(bitSize));
		writer.write(STREAM_RESPONSE_PREFIX);
		writer.write(streamID_);
		writer.write(requestID);
		writer.write(ERROR_RESPONSE);
		writer.write(errMsg);

		return writer.bytes();
	}

	// Create message response for stream messages
	std::vector<uint8_t> respondWithStreamMessage(uint64_t requestID, const std::vector<uint8_t>& msgBytes) {
		using scg::serialize::bit_size;

		size_t bitSize = bit_size(STREAM_RESPONSE_PREFIX) +
						 bit_size(streamID_) +
						 bit_size(requestID) +
						 bit_size(MESSAGE_RESPONSE) +
						 (msgBytes.size() * 8); // Message data in bits

		serialize::FixedSizeWriter writer(serialize::bits_to_bytes(bitSize));
		writer.write(STREAM_RESPONSE_PREFIX);
		writer.write(streamID_);
		writer.write(requestID);
		writer.write(MESSAGE_RESPONSE);

		// Write the message bytes
		for (uint8_t byte : msgBytes) {
			writer.write(byte);
		}

		return writer.bytes();
	}

private:
	mutable std::mutex mu_;
	uint64_t streamID_;
	uint64_t serviceID_;
	std::shared_ptr<Connection> connection_;
	StreamErrorHandler errorHandler_;
	bool closed_;
	std::promise<void> closedPromise_;
	context::Context ctx_; // Stream context
	MessageProcessor* processor_ = nullptr; // Message processor for incoming messages
	std::shared_ptr<void> userData_; // User data storage to keep objects alive

	uint64_t requestID_;
	std::map<uint64_t, std::shared_ptr<std::promise<serialize::Reader>>> requests_;
};

}
}
