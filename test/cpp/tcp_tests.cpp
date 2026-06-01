#include "test_suite.h"
#include "scg/tcp/transport_server.h"
#include "scg/tcp/transport_server_tls.h"
#include "scg/tcp/transport_client.h"
#include "scg/tcp/transport_client_tls.h"

#include <csignal>
#include <execinfo.h>
#include <cxxabi.h>
#include <unistd.h>

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

// helper method to reduce redundant test typing
#define TEST(x) {#x, x}

TEST_LIST = {
	TEST(test_tcp_suite),
	TEST(test_tcp_tls_suite),
	TEST(test_keepalive_reconnect),
	{ NULL, NULL }
};
