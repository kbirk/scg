#include "test_suite.h"
#include "scg/ws/transport_server.h"
#include "scg/ws/transport_server_tls.h"
#include "scg/ws/transport_client.h"
#include "scg/ws/transport_client_tls.h"

#include <csignal>
#include <iostream>
#include <execinfo.h>
#include <cxxabi.h>
#include <unistd.h>

#include "raw_socket.h"

#include "test_certs.h"

void signalHandler(int sig) {
	const int max_frames = 128;
	void* buffer[max_frames];

	std::cerr << "\n=== SIGNAL " << sig << " CAUGHT ===" << std::endl;

	int nptrs = backtrace(buffer, max_frames);
	std::cerr << "Backtrace (" << nptrs << " frames):" << std::endl;

	char** symbols = backtrace_symbols(buffer, nptrs);
	if (symbols) {
		for (int i = 0; i < nptrs; i++) {
			std::cerr << symbols[i] << std::endl;
		}
		free(symbols);
	}

	std::cerr << "=== END BACKTRACE ===" << std::endl;
	_exit(1);
}

// ============================================================================
// WebSocket (No TLS) Transport Factory
// ============================================================================

TransportFactory createWebSocketTransportFactory() {
	TransportFactory factory;
	factory.name = "WebSocket";

	factory.createServerTransport = [](int id) -> std::shared_ptr<scg::rpc::ServerTransport> {
		scg::ws::ServerTransportConfig transportConfig;
		transportConfig.port = 18000 + id;
		return std::make_shared<scg::ws::ServerTransportWS>(transportConfig);
	};

	factory.createClientTransport = [](int id) -> std::shared_ptr<scg::rpc::ClientTransport> {
		scg::ws::ClientTransportConfig transportConfig;
		transportConfig.host = "localhost";
		transportConfig.port = 18000 + id;
		return std::make_shared<scg::ws::ClientTransportWS>(transportConfig);
	};

	factory.createLimitedClientTransport = [](int id) -> std::shared_ptr<scg::rpc::ClientTransport> {
		scg::ws::ClientTransportConfig transportConfig;
		transportConfig.host = "localhost";
		transportConfig.port = 18000 + id;
		transportConfig.maxSendMessageSize = 1024;
		transportConfig.maxRecvMessageSize = 1024;
		return std::make_shared<scg::ws::ClientTransportWS>(transportConfig);
	};

	factory.createLimitedServerTransport = [](int id) -> std::shared_ptr<scg::rpc::ServerTransport> {
		scg::ws::ServerTransportConfig transportConfig;
		transportConfig.port = 18000 + id;
		transportConfig.maxSendMessageSize = 1024;
		return std::make_shared<scg::ws::ServerTransportWS>(transportConfig);
	};

	return factory;
}

// ============================================================================
// WebSocket TLS Transport Factory
// ============================================================================

TransportFactory createWebSocketTLSTransportFactory() {
	TransportFactory factory;
	factory.name = "WebSocket-TLS";

	factory.createServerTransport = [](int id) -> std::shared_ptr<scg::rpc::ServerTransport> {
		scg::ws::ServerTransportTLSConfig transportConfig;
		transportConfig.port = 18100 + id;
		transportConfig.certFile = "../../server.crt";
		transportConfig.keyFile = "../../server.key";
		return std::make_shared<scg::ws::ServerTransportWSTLS>(transportConfig);
	};

	factory.createClientTransport = [](int id) -> std::shared_ptr<scg::rpc::ClientTransport> {
		scg::ws::ClientTransportTLSConfig transportConfig;
		transportConfig.host = "localhost";
		transportConfig.port = 18100 + id;
		transportConfig.verifyPeer = false;
		return std::make_shared<scg::ws::ClientTransportWSTLS>(transportConfig);
	};

	factory.createLimitedClientTransport = [](int id) -> std::shared_ptr<scg::rpc::ClientTransport> {
		scg::ws::ClientTransportTLSConfig transportConfig;
		transportConfig.host = "localhost";
		transportConfig.port = 18100 + id;
		transportConfig.maxSendMessageSize = 1024;
		transportConfig.maxRecvMessageSize = 1024;
		transportConfig.verifyPeer = false;
		return std::make_shared<scg::ws::ClientTransportWSTLS>(transportConfig);
	};

	factory.createLimitedServerTransport = [](int id) -> std::shared_ptr<scg::rpc::ServerTransport> {
		scg::ws::ServerTransportTLSConfig transportConfig;
		transportConfig.port = 18100 + id;
		transportConfig.certFile = "../../server.crt";
		transportConfig.keyFile = "../../server.key";
		transportConfig.maxSendMessageSize = 1024;
		return std::make_shared<scg::ws::ServerTransportWSTLS>(transportConfig);
	};

	return factory;
}

// ============================================================================
// Test Suite Entry Points
// ============================================================================

void test_websocket_suite() {
	signal(SIGSEGV, signalHandler);
	signal(SIGABRT, signalHandler);

	TestSuiteConfig config;
	config.factory = createWebSocketTransportFactory();
	config.startingId = 0;
	config.maxRetries = 10;
	runTestSuite(config);
}

void test_websocket_tls_suite() {
	signal(SIGSEGV, signalHandler);
	signal(SIGABRT, signalHandler);

	TestSuiteConfig config;
	config.factory = createWebSocketTLSTransportFactory();
	config.startingId = 0;
	config.maxRetries = 10;
	runTestSuite(config);
}


// ============================================================================
// WebSocket-specific tests. Ports 18400+ (outside the suites' ranges).
// ============================================================================

namespace {

scg::error::Error wssConnect(const scg::ws::ClientTransportTLSConfig& cfg) {
	scg::ws::ClientTransportWSTLS transport(cfg);
	auto res = transport.connect();
	if (res.first) {
		res.first->close();
	}
	return res.second;
}

} // namespace

// WSS must verify that the certificate matches the host, not only that a
// trusted CA signed it.
void test_wss_hostname_verification() {
	// Generated at run time (CN/SAN localhost); no key is checked in.
	const auto& localhost = testcerts::localhostCert();
	const int port = 18400;

	scg::ws::ServerTransportTLSConfig tcfg;
	tcfg.port = port;
	tcfg.certFile = localhost.certFile;
	tcfg.keyFile = localhost.keyFile;
	scg::rpc::ServerConfig cfg;
	cfg.transport = std::make_shared<scg::ws::ServerTransportWSTLS>(tcfg);
	auto server = std::make_shared<scg::rpc::Server>(cfg);
	pingpong::registerPingPongServer(server.get(), std::make_shared<PingPongServerImpl>());
	TEST_CHECK(!server->start());
	std::this_thread::sleep_for(std::chrono::milliseconds(100));

	scg::ws::ClientTransportTLSConfig good;
	good.host = "localhost";
	good.port = port;
	good.caFile = localhost.certFile;
	TEST_CHECK(good.verifyPeer);
	auto err = wssConnect(good);
	TEST_CHECK(err == nullptr);
	TEST_MSG("matching host failed: %s", err.message().c_str());

	scg::ws::ClientTransportTLSConfig mismatch = good;
	mismatch.host = "127.0.0.1"; // trusted CA, wrong name
	err = wssConnect(mismatch);
	TEST_CHECK(err != nullptr);
	printf("WSS host mismatch: %s\n", err.message().c_str());

	// Self-signed (untrusted) certificate with a default config.
	scg::ws::ClientTransportTLSConfig untrusted;
	untrusted.host = "localhost";
	untrusted.port = port;
	err = wssConnect(untrusted);
	TEST_CHECK(err != nullptr);

	server->shutdown();
}


// A peer that stops reading must not pin its connection: once websocketpp's
// buffered amount passes maxWriteQueueBytes the server fails the connection
// AND closes the socket, so the peer sees EOF (the socket used to stay open
// after the failure was reported).
void test_ws_write_queue_bound_closes_socket() {
	const int port = 18401;
	scg::ws::ServerTransportConfig tcfg;
	tcfg.port = port;
	tcfg.maxWriteQueueBytes = 256 * 1024;
	scg::rpc::ServerConfig cfg;
	cfg.transport = std::make_shared<scg::ws::ServerTransportWS>(tcfg);
	cfg.errorHandler = [](const scg::error::Error&) {};
	auto server = std::make_shared<scg::rpc::Server>(cfg);
	pingpong::registerPingPongServer(server.get(), std::make_shared<PingPongServerImpl>());
	pingpong::registerChatServer(server.get(), std::make_shared<ChatServerImpl>());
	TEST_CHECK(!server->start());

	rawtest::RawSocket raw(port);
	raw.setRecvBuffer(4096);
	TEST_CHECK(raw.connected());

	// Minimal WebSocket client handshake.
	std::string upgrade =
		"GET / HTTP/1.1\r\n"
		"Host: localhost\r\n"
		"Upgrade: websocket\r\n"
		"Connection: Upgrade\r\n"
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n"
		"Sec-WebSocket-Version: 13\r\n\r\n";
	TEST_CHECK(raw.write(std::vector<uint8_t>(upgrade.begin(), upgrade.end())));
	std::string response = rawtest::readHttpHeaders(raw.fd(), 3000);
	TEST_CHECK(response.find(" 101 ") != std::string::npos);
	TEST_MSG("handshake response: %s", response.c_str());

	// OPEN a subscribe stream for a huge number of events, then never read.
	const uint64_t streamID = 1;
	{
		scg::serialize::Writer w(64);
		w.write(scg::rpc::STREAM_PREFIX);
		w.write(streamID);
		w.write(scg::rpc::STREAM_FRAME_OPEN);
		w.write(scg::context::Context());
		w.write(pingpong::chatID);
		w.write(pingpong::chat_SubscribeID);
		TEST_CHECK(raw.write(rawtest::wsClientFrame(w.bytes())));
	}
	pingpong::SubscribeRequest req;
	req.count = 10000000;
	TEST_CHECK(raw.write(rawtest::wsClientFrame(scg::rpc::serializeStreamMessage(streamID, req))));
	TEST_CHECK(raw.write(rawtest::wsClientFrame(scg::rpc::serializeStreamHalfClose(streamID))));

	// Let the server fill the socket buffers and its queue, then expect EOF.
	std::this_thread::sleep_for(std::chrono::milliseconds(1000));
	TEST_CHECK(raw.waitClosed(10000));

	// The server keeps serving.
	scg::ws::ClientTransportConfig ccfg;
	ccfg.host = "localhost";
	ccfg.port = port;
	scg::rpc::ClientConfig clientConfig;
	clientConfig.transport = std::make_shared<scg::ws::ClientTransportWS>(ccfg);
	auto client = std::make_shared<scg::rpc::Client>(clientConfig);
	TEST_CHECK(connectWithRetries(client, 20));
	pingpong::PingPongClient pp(client);
	auto c = deadlineIn(5000);
	pingpong::PingRequest ping;
	TEST_CHECK(pp.ping(c, ping).second == nullptr);
	client->disconnect();
	server->shutdown();
}

// connectTimeout = 0 means "no timeout" (it used to make every connect fail
// instantly) and a huge timeout must not overflow the deadline.
void test_ws_connect_timeout_zero_and_max() {
	const int port = 18402;
	scg::ws::ServerTransportConfig tcfg;
	tcfg.port = port;
	scg::rpc::ServerConfig cfg;
	cfg.transport = std::make_shared<scg::ws::ServerTransportWS>(tcfg);
	auto server = std::make_shared<scg::rpc::Server>(cfg);
	pingpong::registerPingPongServer(server.get(), std::make_shared<PingPongServerImpl>());
	TEST_CHECK(!server->start());
	std::this_thread::sleep_for(std::chrono::milliseconds(100));

	for (auto timeout : {std::chrono::milliseconds(0), std::chrono::milliseconds::max()}) {
		scg::ws::ClientTransportConfig ccfg;
		ccfg.host = "localhost";
		ccfg.port = port;
		ccfg.connectTimeout = timeout;
		scg::ws::ClientTransportWS transport(ccfg);
		auto res = transport.connect();
		TEST_CHECK(res.second == nullptr);
		TEST_MSG("connectTimeout=%lld: %s", (long long)timeout.count(), res.second.message().c_str());
		if (res.first) {
			res.first->close();
		}
	}
	server->shutdown();
}

// helper method to reduce redundant test typing
#define TEST(x) {#x, x}

TEST_LIST = {
	TEST(test_websocket_suite),
	TEST(test_websocket_tls_suite),
	TEST(test_wss_hostname_verification),
	TEST(test_ws_write_queue_bound_closes_socket),
	TEST(test_ws_connect_timeout_zero_and_max),
	{ NULL, NULL }
};
