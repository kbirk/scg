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
	{ NULL, NULL }
};
