#pragma once

#include "scg/client.h"

namespace scg {
namespace rpc {

struct WSClientNoTLSConfig : public websocketpp::config::asio_client {
	// override logger
	typedef log::LoggerOverride elog_type;
	typedef log::LoggerOverride alog_type;

	struct transport_config : public websocketpp::config::asio_client::transport_config {
        typedef log::LoggerOverride elog_type;
        typedef log::LoggerOverride alog_type;
    };

    typedef websocketpp::transport::asio::endpoint<transport_config> transport_type;
};

typedef websocketpp::client<WSClientNoTLSConfig> WSClientNoTLS;

class ClientNoTLS : public Client {
public:

	ClientNoTLS(const ClientConfig& conf)
	{
		conf_ = conf;
		status_ = ConnectionStatus::NOT_CONNECTED;

		// set logging parameters
		registerLoggerMethods(client_.get_alog());
		registerLoggerMethods(client_.get_elog());

		client_.init_asio();

		// without this `run` exits once there are no active connections
		client_.start_perpetual();

		// start `run` in its own thread
		thread_ = std::make_shared<std::thread>(&WSClientNoTLS::run, &client_);

		// randomize the starting request id
		std::random_device rd;
		std::mt19937_64 gen(rd());
		std::uniform_int_distribution<uint64_t> dis;
		requestID_ = dis(gen);
	}

	~ClientNoTLS()
	{
		// this flags the `run` method to exit once all connections are closed
		client_.stop_perpetual();

		disconnect();

		// wait until the `run` method exits
		thread_->join();
	}

protected:

	error::Error connectUnsafe()
	{
		return connectUnsafeImpl(&client_, "ws://" + conf_.uri + "/rpc");
	}

	error::Error disconnectUnsafe()
	{
		return disconnectUnsafeImpl(&client_);
	}

	virtual error::Error sendBytesUnsafe(const std::vector<uint8_t>& msg)
	{
		return sendBytesUnsafeImpl(&client_, msg);
	}

	WSClientNoTLS client_;
};

}
}
