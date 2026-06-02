#include "test_suite.h"
#include "scg/tcp/transport_server.h"
#include "scg/tcp/transport_server_tls.h"
#include "scg/tcp/transport_client.h"
#include "scg/tcp/transport_client_tls.h"

#include <csignal>
#include <execinfo.h>
#include <cxxabi.h>
#include <unistd.h>
#include <fstream>

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

	return factory;
}

// ============================================================================
// Test Suite Entry Points
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

// ============================================================================
// Keepalive reconnect test: a raw black-hole acceptor that accepts and reads but
// never replies. With keepalive enabled, the first connection times out; a
// reconnect on the same Client must ALSO time out — which only happens if
// keepalive resumes after reconnect (the persistent keepalive thread). Under the
// old per-connection design the second Recv would hang forever.
// ============================================================================

class BlackHoleServer {
public:
	explicit BlackHoleServer(int port)
		: acceptor_(ioc_, asio::ip::tcp::endpoint(asio::ip::tcp::v4(), port))
	{
		doAccept();
		thread_ = std::thread([this]() { ioc_.run(); });
	}

	~BlackHoleServer() {
		ioc_.stop();
		if (thread_.joinable()) {
			thread_.join();
		}
	}

private:
	void doAccept() {
		acceptor_.async_accept([this](std::error_code ec, asio::ip::tcp::socket sock) {
			if (!ec) {
				auto s = std::make_shared<asio::ip::tcp::socket>(std::move(sock));
				doRead(s);
			}
			if (acceptor_.is_open()) {
				doAccept();
			}
		});
	}

	void doRead(std::shared_ptr<asio::ip::tcp::socket> s) {
		auto buf = std::make_shared<std::array<uint8_t, 1024>>();
		s->async_read_some(asio::buffer(*buf), [this, s, buf](std::error_code ec, std::size_t) {
			if (!ec) {
				doRead(s); // discard and keep reading; never write back
			}
		});
	}

	asio::io_context ioc_;
	asio::ip::tcp::acceptor acceptor_;
	std::thread thread_;
};

static void expectKeepaliveTimeout(pingpong::ChatClient& chatClient) {
	scg::context::Context ctx;
	auto cr = chatClient.connect(ctx);
	TEST_CHECK(cr.second == nullptr);
	if (cr.second) {
		return;
	}
	auto r = cr.first->recv();
	TEST_CHECK(r.state == scg::rpc::StreamRecvState::Closed);
	TEST_CHECK(r.error != nullptr);
	if (r.error) {
		TEST_CHECK(r.error.message().find("keepalive timeout") != std::string::npos);
	}
}

// ============================================================================
// Malformed-frame robustness: feed a real scg server a series of hostile frames
// over a raw socket and verify it neither crashes nor wedges — a normal client
// must still work afterward.
// ============================================================================

static void sendRawFrame(asio::ip::tcp::socket& sock, const std::vector<uint8_t>& payload) {
	uint32_t len = static_cast<uint32_t>(payload.size());
	uint8_t hdr[4] = {
		static_cast<uint8_t>((len >> 24) & 0xFF),
		static_cast<uint8_t>((len >> 16) & 0xFF),
		static_cast<uint8_t>((len >> 8) & 0xFF),
		static_cast<uint8_t>(len & 0xFF)};
	asio::write(sock, asio::buffer(hdr, 4));
	if (!payload.empty()) {
		asio::write(sock, asio::buffer(payload));
	}
}

static std::vector<uint8_t> streamFrameBytes(uint64_t streamID, uint8_t frameKind, std::vector<uint8_t> tail) {
	scg::serialize::Writer w(64);
	w.write(scg::rpc::STREAM_PREFIX);
	w.write(streamID);
	w.write(frameKind);
	auto b = w.bytes();
	b.insert(b.end(), tail.begin(), tail.end());
	return b;
}

void test_malformed_frames() {
	printf("Running Malformed Frames test...\n");

	const int port = 18760;

	scg::rpc::ServerConfig cfg;
	scg::tcp::ServerTransportConfig tcfg;
	tcfg.port = port;
	cfg.transport = std::make_shared<scg::tcp::ServerTransportTCP>(tcfg);
	cfg.errorHandler = [](const scg::error::Error&) {}; // malformed input is expected to error
	auto server = std::make_shared<scg::rpc::Server>(cfg);
	pingpong::registerPingPongServer(server.get(), std::make_shared<PingPongServerImpl>());
	pingpong::registerChatServer(server.get(), std::make_shared<ChatServerImpl>());
	TEST_CHECK(!server->start());

	std::this_thread::sleep_for(std::chrono::milliseconds(150));

	// Raw socket: send a grab-bag of malformed frames, then close.
	{
		asio::io_context ioc;
		asio::ip::tcp::socket sock(ioc);
		sock.connect(asio::ip::tcp::endpoint(asio::ip::make_address("127.0.0.1"), port));

		std::vector<std::vector<uint8_t>> malformed;
		// Junk with no recognizable prefix.
		malformed.push_back({'g', 'a', 'r', 'b', 'a', 'g', 'e', '!', '!', '!', '!', '!', '!', '!', '!', '!', '!', '!'});
		// Empty frame.
		malformed.push_back({});
		// Valid stream prefix but truncated.
		{
			scg::serialize::Writer w(16);
			w.write(scg::rpc::STREAM_PREFIX);
			malformed.push_back(w.bytes());
		}
		// Unknown frame kind.
		malformed.push_back(streamFrameBytes(7, 0xFF, {}));
		// MSG / HALF_CLOSE / CLOSE for a stream that was never opened.
		malformed.push_back(streamFrameBytes(123, scg::rpc::STREAM_FRAME_MESSAGE, {0x01, 0x02, 0x03}));
		malformed.push_back(streamFrameBytes(124, scg::rpc::STREAM_FRAME_HALF_CLOSE, {}));
		malformed.push_back(streamFrameBytes(125, scg::rpc::STREAM_FRAME_CLOSE, {0x00}));
		// OPEN with garbage tail.
		malformed.push_back(streamFrameBytes(200, scg::rpc::STREAM_FRAME_OPEN, {0xDE, 0xAD, 0xBE, 0xEF, 0x00, 0x11}));
		// Connection-level PING/PONG — server must tolerate.
		malformed.push_back(streamFrameBytes(0, scg::rpc::STREAM_FRAME_PING, {}));
		malformed.push_back(streamFrameBytes(0, scg::rpc::STREAM_FRAME_PONG, {}));

		for (const auto& m : malformed) {
			sendRawFrame(sock, m);
		}
		sock.close();
	}

	std::this_thread::sleep_for(std::chrono::milliseconds(100));

	// The server must still be alive and serving.
	scg::rpc::ClientConfig ccfg;
	scg::tcp::ClientTransportConfig ctcfg;
	ctcfg.host = "127.0.0.1";
	ctcfg.port = port;
	ccfg.transport = std::make_shared<scg::tcp::ClientTransportTCP>(ctcfg);
	auto client = std::make_shared<scg::rpc::Client>(ccfg);
	TEST_CHECK(connectWithRetries(client, 10));

	pingpong::PingPongClient pp(client);
	scg::context::Context ctx;
	pingpong::PingRequest req;
	req.ping.count = 41;
	auto [res, err] = pp.ping(ctx, req);
	TEST_CHECK(err == nullptr);
	TEST_CHECK(res.pong.count == 42);

	pingpong::ChatClient chat(client);
	auto cr = chat.connect(ctx);
	TEST_CHECK(cr.second == nullptr);
	if (!cr.second) {
		auto w = cr.first->recv();
		TEST_CHECK(w.state == scg::rpc::StreamRecvState::Message);
		TEST_CHECK(w.message.text == "welcome");
	}

	client->disconnect();
	server->shutdown();
	printf("Malformed Frames test passed\n");
}

void test_keepalive_reconnect() {
	printf("Running Keepalive Reconnect test...\n");

	const int port = 19500;
	BlackHoleServer blackhole(port);

	scg::rpc::ClientConfig cfg;
	scg::tcp::ClientTransportConfig tcfg;
	tcfg.host = "127.0.0.1";
	tcfg.port = port;
	cfg.transport = std::make_shared<scg::tcp::ClientTransportTCP>(tcfg);
	cfg.keepaliveInterval = std::chrono::milliseconds(40);
	cfg.keepaliveTimeout = std::chrono::milliseconds(150);
	auto client = std::make_shared<scg::rpc::Client>(cfg);

	pingpong::ChatClient chatClient(client);

	// Connection 1 times out (peer never replies).
	expectKeepaliveTimeout(chatClient);

	// Connection 2 (a reconnect on the same Client) must ALSO time out — proving
	// keepalive resumed after the reconnect.
	expectKeepaliveTimeout(chatClient);

	client->disconnect();
	printf("Keepalive Reconnect test passed\n");
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
// Thread-leak test: open and close many streams and assert the process thread
// count returns to baseline — every per-stream handler thread must exit.
// ============================================================================

static int currentThreadCount() {
	std::ifstream f("/proc/self/status");
	std::string line;
	while (std::getline(f, line)) {
		if (line.rfind("Threads:", 0) == 0) {
			try {
				return std::stoi(line.substr(8));
			} catch (...) {
				return -1;
			}
		}
	}
	return -1;
}

void test_stream_thread_no_leak() {
	printf("Running Stream Thread Leak test...\n");

	const int port = 18780;

	scg::rpc::ServerConfig cfg;
	scg::tcp::ServerTransportConfig tcfg;
	tcfg.port = port;
	cfg.transport = std::make_shared<scg::tcp::ServerTransportTCP>(tcfg);
	cfg.errorHandler = [](const scg::error::Error&) {};
	auto server = std::make_shared<scg::rpc::Server>(cfg);
	pingpong::registerPingPongServer(server.get(), std::make_shared<PingPongServerImpl>());
	pingpong::registerChatServer(server.get(), std::make_shared<ChatServerImpl>());
	TEST_CHECK(!server->start());
	std::this_thread::sleep_for(std::chrono::milliseconds(150));

	scg::rpc::ClientConfig ccfg;
	scg::tcp::ClientTransportConfig ctcfg;
	ctcfg.host = "127.0.0.1";
	ctcfg.port = port;
	ccfg.transport = std::make_shared<scg::tcp::ClientTransportTCP>(ctcfg);
	auto client = std::make_shared<scg::rpc::Client>(ccfg);
	TEST_CHECK(connectWithRetries(client, 10));

	pingpong::ChatClient chat(client);

	auto oneStream = [&]() {
		scg::context::Context ctx;
		auto cr = chat.connect(ctx);
		if (cr.second) {
			return;
		}
		auto stream = cr.first;
		stream->recv(); // welcome
		pingpong::ChatMessage m;
		m.text = "x";
		m.seq = 1;
		stream->send(m);
		stream->recv(); // echo
		stream->closeSend();
		stream->recv(); // summary
		stream->recv(); // closed
	};

	// Warm up, then record the baseline thread count.
	oneStream();
	std::this_thread::sleep_for(std::chrono::milliseconds(200));
	int baseline = currentThreadCount();

	const int N = 300;
	for (int i = 0; i < N; i++) {
		oneStream();
	}

	std::this_thread::sleep_for(std::chrono::milliseconds(400));
	int final = currentThreadCount();

	printf("threads: baseline=%d final=%d after %d streams\n", baseline, final, N);
	TEST_CHECK(baseline > 0);
	TEST_CHECK(final > 0);
	// A per-stream leak would add ~N threads; allow small slack for in-flight teardown.
	TEST_CHECK(final <= baseline + 5);

	client->disconnect();
	server->shutdown();
	printf("Stream Thread Leak test passed\n");
}

// helper method to reduce redundant test typing
#define TEST(x) {#x, x}

TEST_LIST = {
	TEST(test_tcp_suite),
	TEST(test_tcp_tls_suite),
	TEST(test_keepalive_reconnect),
	TEST(test_malformed_frames),
	TEST(test_stream_thread_no_leak),
	{ NULL, NULL }
};
