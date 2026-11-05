#include "test_suite.h"
#include "scg/tcp/transport_server.h"
#include "scg/tcp/transport_server_tls.h"
#include "scg/tcp/transport_client.h"
#include "scg/tcp/transport_client_tls.h"

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
// TCP Transport Factory
// ============================================================================

TransportFactory createTCPTransportFactory() {
	TransportFactory factory;
	factory.name = "TCP";

	factory.createServerTransport = [](int id) -> std::shared_ptr<scg::rpc::ServerTransport> {
		scg::tcp::ServerTransportConfig transportConfig;
		transportConfig.port = 19000 + id;
		return std::make_shared<scg::tcp::ServerTransportTCP>(transportConfig);
	};

	factory.createClientTransport = [](int id) -> std::shared_ptr<scg::rpc::ClientTransport> {
		scg::tcp::ClientTransportConfig transportConfig;
		transportConfig.host = "127.0.0.1";
		transportConfig.port = 19000 + id;
		return std::make_shared<scg::tcp::ClientTransportTCP>(transportConfig);
	};

	factory.createLimitedClientTransport = [](int id) -> std::shared_ptr<scg::rpc::ClientTransport> {
		scg::tcp::ClientTransportConfig transportConfig;
		transportConfig.host = "127.0.0.1";
		transportConfig.port = 19000 + id;
		transportConfig.maxSendMessageSize = 1024;
		transportConfig.maxRecvMessageSize = 1024;
		return std::make_shared<scg::tcp::ClientTransportTCP>(transportConfig);
	};

	factory.createLimitedServerTransport = [](int id) -> std::shared_ptr<scg::rpc::ServerTransport> {
		scg::tcp::ServerTransportConfig transportConfig;
		transportConfig.port = 19000 + id;
		transportConfig.maxSendMessageSize = 1024;
		return std::make_shared<scg::tcp::ServerTransportTCP>(transportConfig);
	};

	return factory;
}

// ============================================================================
// TCP+TLS Transport Factory
// ============================================================================

TransportFactory createTCPTLSTransportFactory() {
	TransportFactory factory;
	factory.name = "TCP-TLS";

	factory.createServerTransport = [](int id) -> std::shared_ptr<scg::rpc::ServerTransport> {
		scg::tcp::ServerTransportTLSConfig transportConfig;
		transportConfig.port = 19100 + id;
		transportConfig.certFile = "../../server.crt";
		transportConfig.keyFile = "../../server.key";
		return std::make_shared<scg::tcp::ServerTransportTCPTLS>(transportConfig);
	};

	factory.createClientTransport = [](int id) -> std::shared_ptr<scg::rpc::ClientTransport> {
		scg::tcp::ClientTransportTLSConfig transportConfig;
		transportConfig.host = "127.0.0.1";
		transportConfig.port = 19100 + id;
		transportConfig.verifyPeer = false;  // Self-signed cert
		return std::make_shared<scg::tcp::ClientTransportTCPTLS>(transportConfig);
	};

	factory.createLimitedClientTransport = [](int id) -> std::shared_ptr<scg::rpc::ClientTransport> {
		scg::tcp::ClientTransportTLSConfig transportConfig;
		transportConfig.host = "127.0.0.1";
		transportConfig.port = 19100 + id;
		transportConfig.verifyPeer = false;  // Self-signed cert
		transportConfig.maxSendMessageSize = 1024;
		transportConfig.maxRecvMessageSize = 1024;
		return std::make_shared<scg::tcp::ClientTransportTCPTLS>(transportConfig);
	};

	factory.createLimitedServerTransport = [](int id) -> std::shared_ptr<scg::rpc::ServerTransport> {
		scg::tcp::ServerTransportTLSConfig transportConfig;
		transportConfig.port = 19100 + id;
		transportConfig.certFile = "../../server.crt";
		transportConfig.keyFile = "../../server.key";
		transportConfig.maxSendMessageSize = 1024;
		return std::make_shared<scg::tcp::ServerTransportTCPTLS>(transportConfig);
	};

	return factory;
}

// ============================================================================
// Test Suite Entry Points
//
// The adversarial/standalone tests (malformed frames, keepalive black-hole,
// server keepalive, no-leak) are transport-agnostic and live in the shared
// runTestSuite (test_suite.h), so they run identically here and over WebSocket.
// ============================================================================

void test_tcp_suite() {
	signal(SIGSEGV, signalHandler);
	signal(SIGABRT, signalHandler);

	TestSuiteConfig config;
	config.factory = createTCPTransportFactory();
	config.startingId = 0;
	config.maxRetries = 10;
	runTestSuite(config);
}

void test_tcp_tls_suite() {
	signal(SIGSEGV, signalHandler);
	signal(SIGABRT, signalHandler);

	TestSuiteConfig config;
	config.factory = createTCPTLSTransportFactory();
	config.startingId = 0;
	config.maxRetries = 10;
	runTestSuite(config);
}


// ============================================================================
// TCP-specific tests: raw-socket peers, TLS verification, connect deadlines,
// write-queue bounds. Ports 19400+ (outside the suites' ranges).
// ============================================================================

namespace {

using rawtest::RawSocket;
using rawtest::framed;

std::shared_ptr<scg::rpc::Server> startTcpPingServer(int port, std::function<void(scg::tcp::ServerTransportConfig&)> configure = nullptr) {
	scg::tcp::ServerTransportConfig tcfg;
	tcfg.port = port;
	if (configure) {
		configure(tcfg);
	}
	scg::rpc::ServerConfig cfg;
	cfg.transport = std::make_shared<scg::tcp::ServerTransportTCP>(tcfg);
	cfg.errorHandler = [](const scg::error::Error&) {};
	auto server = std::make_shared<scg::rpc::Server>(cfg);
	pingpong::registerPingPongServer(server.get(), std::make_shared<PingPongServerImpl>());
	pingpong::registerChatServer(server.get(), std::make_shared<ChatServerImpl>());
	TEST_CHECK(!server->start());
	return server;
}

bool pingOverTcp(int port) {
	scg::tcp::ClientTransportConfig tcfg;
	tcfg.host = "127.0.0.1";
	tcfg.port = port;
	scg::rpc::ClientConfig cfg;
	cfg.transport = std::make_shared<scg::tcp::ClientTransportTCP>(tcfg);
	auto client = std::make_shared<scg::rpc::Client>(cfg);
	if (!connectWithRetries(client, 20)) {
		return false;
	}
	pingpong::PingPongClient pp(client);
	auto c = deadlineIn(5000);
	pingpong::PingRequest req;
	return pp.ping(c, req).second == nullptr;
}

} // namespace

// A frame header larger than maxRecvMessageSize closes the connection before
// any allocation; a truncated frame followed by FIN closes it too. The server
// keeps serving.
void test_tcp_hostile_frames() {
	const int port = 19400;
	auto server = startTcpPingServer(port);

	{
		RawSocket raw(port);
		TEST_CHECK(raw.connected());
		raw.write({0xFF, 0xFF, 0xFF, 0xFF});
		TEST_CHECK(raw.waitClosed(3000));
	}
	{
		RawSocket raw(port);
		TEST_CHECK(raw.connected());
		raw.write({0x00, 0x00, 0x00, 100, 1, 2, 3, 4, 5}); // promises 100 bytes, sends 5
		raw.shutdownWrite();
		TEST_CHECK(raw.waitClosed(3000));
	}
	TEST_CHECK(pingOverTcp(port));
	server->shutdown();
}

// A TLS server closes accepted sockets that never complete the handshake.
void test_tcp_tls_handshake_timeout() {
	const int port = 19401;
	scg::tcp::ServerTransportTLSConfig tcfg;
	tcfg.port = port;
	tcfg.certFile = "../../server.crt";
	tcfg.keyFile = "../../server.key";
	tcfg.handshakeTimeout = std::chrono::milliseconds(300);
	scg::rpc::ServerConfig cfg;
	cfg.transport = std::make_shared<scg::tcp::ServerTransportTCPTLS>(tcfg);
	auto server = std::make_shared<scg::rpc::Server>(cfg);
	pingpong::registerPingPongServer(server.get(), std::make_shared<PingPongServerImpl>());
	TEST_CHECK(!server->start());

	RawSocket raw(port);
	TEST_CHECK(raw.connected());
	auto start = std::chrono::steady_clock::now();
	TEST_CHECK(raw.waitClosed(5000)); // never send a ClientHello
	double ms = elapsedMs(start);
	printf("idle TLS socket closed after %.0fms\n", ms);
	TEST_CHECK(ms < 3000);
	server->shutdown();
}

namespace {

std::shared_ptr<scg::rpc::Server> startTlsServer(int port, const std::string& cert, const std::string& key) {
	scg::tcp::ServerTransportTLSConfig tcfg;
	tcfg.port = port;
	tcfg.certFile = cert;
	tcfg.keyFile = key;
	scg::rpc::ServerConfig cfg;
	cfg.transport = std::make_shared<scg::tcp::ServerTransportTCPTLS>(tcfg);
	auto server = std::make_shared<scg::rpc::Server>(cfg);
	pingpong::registerPingPongServer(server.get(), std::make_shared<PingPongServerImpl>());
	TEST_CHECK(!server->start());
	std::this_thread::sleep_for(std::chrono::milliseconds(100));
	return server;
}

scg::error::Error tlsConnect(const scg::tcp::ClientTransportTLSConfig& cfg) {
	scg::tcp::ClientTransportTCPTLS transport(cfg);
	auto res = transport.connect();
	if (res.first) {
		res.first->close();
	}
	return res.second;
}

} // namespace

// TLS verification: a default-constructed config verifies the peer, and the
// certificate must match the host name.
void test_tcp_tls_verification() {
	// Generated at run time (CN/SAN localhost); no key is checked in.
	const auto& localhost = testcerts::localhostCert();

	// Self-signed server certificate: a default config must refuse it.
	{
		const int port = 19402;
		auto server = startTlsServer(port, "../../server.crt", "../../server.key");
		scg::tcp::ClientTransportTLSConfig cfg; // verifyPeer defaults to true
		cfg.host = "127.0.0.1";
		cfg.port = port;
		TEST_CHECK(cfg.verifyPeer);
		auto err = tlsConnect(cfg);
		TEST_CHECK(err != nullptr);
		printf("default config vs self-signed: %s\n", err.message().c_str());
		server->shutdown();
	}

	// localhost certificate, trusted via caFile.
	{
		const int port = 19403;
		auto server = startTlsServer(port, localhost.certFile, localhost.keyFile);

		scg::tcp::ClientTransportTLSConfig good;
		good.host = "localhost";
		good.port = port;
		good.caFile = localhost.certFile;
		auto err = tlsConnect(good);
		TEST_CHECK(err == nullptr);
		TEST_MSG("matching host failed: %s", err.message().c_str());

		scg::tcp::ClientTransportTLSConfig mismatch = good;
		mismatch.host = "127.0.0.1"; // not in the certificate
		err = tlsConnect(mismatch);
		TEST_CHECK(err != nullptr);
		printf("host mismatch: %s\n", err.message().c_str());

		server->shutdown();
	}
}

// Connecting is bounded by the call deadline and does not hold the client
// lock: other threads are not blocked behind a slow connect.
void test_tcp_connect_deadline() {
	scg::tcp::ClientTransportConfig tcfg;
	tcfg.host = "10.255.255.1"; // non-routable: the connect hangs or fails
	tcfg.port = 9;
	scg::rpc::ClientConfig cfg;
	cfg.transport = std::make_shared<scg::tcp::ClientTransportTCP>(tcfg);
	auto client = std::make_shared<scg::rpc::Client>(cfg);
	pingpong::PingPongClient pp(client);

	// acutest's checks are not thread-safe: record in the thread, check after join.
	scg::error::Error callErr;
	double callMs = 0;
	std::thread caller([&]() {
		auto c = deadlineIn(300);
		pingpong::PingRequest req;
		auto start = std::chrono::steady_clock::now();
		callErr = pp.ping(c, req).second;
		callMs = elapsedMs(start);
	});
	std::this_thread::sleep_for(std::chrono::milliseconds(50));
	auto start = std::chrono::steady_clock::now();
	client->disconnect(); // must not wait for the connect
	double disconnectMs = elapsedMs(start);
	caller.join();
	printf("call to unreachable host: err=\"%s\" after %.0fms\n", callErr.message().c_str(), callMs);
	TEST_CHECK(callErr != nullptr);
	TEST_CHECK(callMs < 2000);
	TEST_CHECK(disconnectMs < 200);
}

// A peer that stops reading cannot make the server buffer without bound: once
// the write queue passes maxWriteQueueBytes the connection is failed and the
// stream handler exits.
void test_tcp_write_queue_bound() {
	const int port = 19404;
	auto server = startTcpPingServer(port, [](scg::tcp::ServerTransportConfig& c) {
		c.maxWriteQueueBytes = 256 * 1024;
	});
	std::this_thread::sleep_for(std::chrono::milliseconds(100));
	int baseline = currentThreadCount();

	RawSocket raw(port);
	raw.setRecvBuffer(4096);
	TEST_CHECK(raw.connected());

	// OPEN a subscribe stream asking for a huge number of events, then never read.
	const uint64_t streamID = 1;
	{
		scg::serialize::Writer w(64);
		w.write(scg::rpc::STREAM_PREFIX);
		w.write(streamID);
		w.write(scg::rpc::STREAM_FRAME_OPEN);
		w.write(scg::context::Context());
		w.write(pingpong::chatID);
		w.write(pingpong::chat_SubscribeID);
		TEST_CHECK(raw.write(framed(w.bytes())));
	}
	{
		pingpong::SubscribeRequest req;
		req.count = 10000000;
		TEST_CHECK(raw.write(framed(scg::rpc::serializeStreamMessage(streamID, req))));
		TEST_CHECK(raw.write(framed(scg::rpc::serializeStreamHalfClose(streamID))));
	}

	int final = 0;
	for (int i = 0; i < 100; i++) {
		std::this_thread::sleep_for(std::chrono::milliseconds(50));
		final = currentThreadCount();
		if (final <= baseline) {
			break;
		}
	}
	printf("threads: baseline=%d final=%d\n", baseline, final);
	TEST_CHECK(final <= baseline); // the handler thread exited
	TEST_CHECK(raw.waitClosed(3000));
	TEST_CHECK(pingOverTcp(port));
	server->shutdown();
}


// connectTimeout = 0 means "no timeout" (it used to make every connect fail
// instantly) and milliseconds::max() must not overflow the deadline.
void test_tcp_connect_timeout_zero_and_max() {
	const int port = 19405;
	auto server = startTcpPingServer(port);
	std::this_thread::sleep_for(std::chrono::milliseconds(100));

	for (auto timeout : {std::chrono::milliseconds(0), std::chrono::milliseconds::max()}) {
		scg::tcp::ClientTransportConfig tcfg;
		tcfg.host = "127.0.0.1";
		tcfg.port = port;
		tcfg.connectTimeout = timeout;
		scg::tcp::ClientTransportTCP transport(tcfg);
		auto res = transport.connect();
		TEST_CHECK(res.second == nullptr);
		TEST_MSG("connectTimeout=%lld: %s", (long long)timeout.count(), res.second.message().c_str());
		if (res.first) {
			res.first->close();
		}

		// Through the Client too (its own connectTimeout).
		scg::rpc::ClientConfig cfg;
		cfg.transport = std::make_shared<scg::tcp::ClientTransportTCP>(tcfg);
		cfg.connectTimeout = timeout;
		auto client = std::make_shared<scg::rpc::Client>(cfg);
		TEST_CHECK(client->connect() == nullptr);
		pingpong::PingPongClient pp(client);
		scg::context::Context c;
		c.setDeadline(std::chrono::system_clock::time_point::max()); // must not overflow
		pingpong::PingRequest req;
		TEST_CHECK(pp.ping(c, req).second == nullptr);
	}
	server->shutdown();
}

// A transport stopped before its loop starts must not run the loop
// (start() immediately followed by shutdown() raced stop() against run()).
void test_tcp_loop_state_contract() {
	asio::io_context io;
	scg::tcp::detail::ServerLoopState state;
	TEST_CHECK(state.beginRun(io));
	state.endRun();
	TEST_CHECK(!state.markStopping()); // not running any more
	TEST_CHECK(!state.beginRun(io));   // stopped before the loop starts
	state.clearStopping();
	TEST_CHECK(state.beginRun(io));
	TEST_CHECK(state.markStopping()); // running: caller must post its close
	state.endRun();

	// start() + immediate shutdown(), repeatedly, must neither hang nor crash.
	for (int i = 0; i < 30; i++) {
		auto server = startTcpPingServer(19406);
		server->shutdown();
	}
}

// helper method to reduce redundant test typing
#define TEST(x) {#x, x}

TEST_LIST = {
	TEST(test_tcp_suite),
	TEST(test_tcp_tls_suite),
	TEST(test_tcp_hostile_frames),
	TEST(test_tcp_tls_handshake_timeout),
	TEST(test_tcp_tls_verification),
	TEST(test_tcp_connect_deadline),
	TEST(test_tcp_write_queue_bound),
	TEST(test_tcp_connect_timeout_zero_and_max),
	TEST(test_tcp_loop_state_contract),
	{ NULL, NULL }
};
