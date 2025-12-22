#pragma once

#include <cstdint>
#include <functional>
#include <future>
#include <map>
#include <memory>
#include <mutex>
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

using StreamErrorHandler = std::function<void(const error::Error&)>;

class Stream {
public:
	Stream(uint64_t streamID, std::shared_ptr<Connection> connection, StreamErrorHandler errorHandler)
		: streamID_(streamID)
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
		std::lock_guard<std::mutex> lock(mu_);

		if (closed_) {
			return std::make_pair(serialize::Reader({}), error::Error("Stream is closed"));
		}

		uint64_t requestID = requestID_++;

		using scg::serialize::bit_size;

		serialize::FixedSizeWriter writer(
			scg::serialize::bits_to_bytes(
				bit_size(STREAM_MESSAGE_PREFIX) +
				bit_size(streamID_) +
				bit_size(requestID) +
				bit_size(methodID) +
				bit_size(msg)));

		writer.write(STREAM_MESSAGE_PREFIX);
		writer.write(streamID_);
		writer.write(requestID);
		writer.write(methodID);
		writer.write(msg);

		auto promise = std::make_shared<std::promise<serialize::Reader>>();
		requests_[requestID] = promise;

		auto err = connection_->send(writer.bytes());
		if (err) {
			requests_.erase(requestID);
			return std::make_pair(serialize::Reader({}), err);
		}

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
	void handleMessage(uint64_t requestID, serialize::Reader& reader) {
		std::lock_guard<std::mutex> lock(mu_);

		auto iter = requests_.find(requestID);
		if (iter != requests_.end()) {
			iter->second->set_value(reader);
			requests_.erase(requestID);
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

private:
	mutable std::mutex mu_;
	uint64_t streamID_;
	std::shared_ptr<Connection> connection_;
	StreamErrorHandler errorHandler_;
	bool closed_;
	std::promise<void> closedPromise_;

	uint64_t requestID_;
	std::map<uint64_t, std::shared_ptr<std::promise<serialize::Reader>>> requests_;
};

}
}
