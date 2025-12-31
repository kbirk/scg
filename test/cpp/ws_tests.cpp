#include "test_suite.h"
#include "scg/ws/transport_server.h"
#include "scg/ws/transport_server_tls.h"
#include "scg/ws/transport_client.h"
#include "scg/ws/transport_client_tls.h"

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

// helper method to reduce redundant test typing
#define TEST(x) {#x, x}

TEST_LIST = {
	TEST(test_websocket_suite),
	TEST(test_websocket_tls_suite),
	{ NULL, NULL }
};
