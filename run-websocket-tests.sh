#!/bin/bash

set -e

TIMEOUT_SECONDS=${SCG_TEST_TIMEOUT:-120}

run_with_timeout() {
	local description="$1"
	shift
	if timeout --foreground "${TIMEOUT_SECONDS}" "$@"; then
		return 0
	fi
	status=$?
	if [ $status -eq 124 ]; then
		echo -e "${RED}${description} timed out after ${TIMEOUT_SECONDS}s${NC}"
	else
		echo -e "${RED}${description} failed (exit $status)${NC}"
	fi
	return 1
}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to cleanup background processes
cleanup() {
	if [ ! -z "$pid" ] && kill -0 $pid 2>/dev/null; then
		echo -e "\n${YELLOW}Stopping server...${NC}"
		kill $pid 2>/dev/null || true
		wait $pid 2>/dev/null || true
	fi
}

# Register cleanup function to run on script exit
trap cleanup EXIT INT TERM

# ========================================
# Go WebSocket Tests (Go Client + Go Server)
# ========================================
echo -e "${YELLOW}Running Go WebSocket tests (Go Client + Go Server)...${NC}"
run_with_timeout "Go WebSocket tests" go test -v -count=1 -run "^(TestWebSocket|TestWebSocketTLS)$" ./test/test_go/service_websocket_test.go ./test/test_go/service_test_suite.go ./test/test_go/test_utils.go
if [ $? -eq 0 ]; then
	echo -e "${GREEN}Go WebSocket tests passed${NC}"
else
	echo -e "${RED}Go WebSocket tests failed${NC}"
	exit 1
fi

# ========================================
# Build C++ WebSocket Tests
# ========================================
echo -e "\n${YELLOW}Building C++ WebSocket tests...${NC}"
mkdir -p .build
cd .build
cmake ../test/test_cpp
# Build only WebSocket-related targets
run_with_timeout "C++ WebSocket build" make ws_tests server_ws_test server_ws_tls_test client_ws_tests client_ws_tls_tests
if [ $? -eq 0 ]; then
	echo -e "${GREEN}C++ WebSocket tests built successfully${NC}"
else
	echo -e "${RED}Failed to build C++ WebSocket tests${NC}"
	exit 1
fi

# ========================================
# C++ WebSocket Tests (C++ Client + C++ Server)
# Covers both WebSocket and WebSocket-TLS
# ========================================
echo -e "\n${YELLOW}Running C++ WebSocket tests (C++ Client + C++ Server)...${NC}"

if run_with_timeout "C++ WebSocket tests" ./ws_tests; then
	echo -e "${GREEN}C++ WebSocket tests passed${NC}"
else
	echo -e "${RED}C++ WebSocket tests failed${NC}"
	exit 1
fi

sleep 1

# ========================================
# Cross-Language Tests: Go Client + C++ Server (WebSocket)
# ========================================
echo -e "\n${YELLOW}Running WebSocket tests (Go Client + C++ Server)...${NC}"

# Start C++ WebSocket server
./server_ws_test > output.txt 2>&1 &
pid=$!
sleep 1

if ! kill -0 $pid 2>/dev/null; then
	echo -e "${RED}Failed to start C++ server${NC}"
	exit 1
fi

echo -e "${GREEN}C++ WebSocket server started (pid: $pid)${NC}"

# Run Go client tests with external server option
if run_with_timeout "Go Client + C++ Server tests" go test -v -count=1 -run TestWebSocketExternalServer ../test/test_go/service_websocket_test.go ../test/test_go/service_test_suite.go ../test/test_go/test_utils.go; then
	echo -e "${GREEN}Go Client + C++ Server tests passed${NC}"
else
	echo -e "${RED}Go Client + C++ Server tests failed${NC}"
	kill $pid 2>/dev/null || true
	exit 1
fi

# Stop server
kill $pid 2>/dev/null || true
wait $pid 2>/dev/null || true
pid=""
sleep 1

# ========================================
# Cross-Language Tests: Go Client + C++ Server (WebSocket TLS)
# ========================================
echo -e "\n${YELLOW}Running WebSocket TLS tests (Go Client + C++ Server)...${NC}"

# Start C++ WebSocket TLS server
./server_ws_tls_test > output.txt 2>&1 &
pid=$!
sleep 1

if ! kill -0 $pid 2>/dev/null; then
	echo -e "${RED}Failed to start C++ TLS server${NC}"
	exit 1
fi

echo -e "${GREEN}C++ WebSocket TLS server started (pid: $pid)${NC}"

# Run Go client TLS tests with external server option
if run_with_timeout "Go Client + C++ Server TLS tests" go test -v -count=1 -run TestWebSocketTLSExternalServer ../test/test_go/service_websocket_test.go ../test/test_go/service_test_suite.go ../test/test_go/test_utils.go; then
	echo -e "${GREEN}Go Client + C++ Server TLS tests passed${NC}"
else
	echo -e "${RED}Go Client + C++ Server TLS tests failed${NC}"
	kill $pid 2>/dev/null || true
	exit 1
fi

# Stop server
kill $pid 2>/dev/null || true
wait $pid 2>/dev/null || true
pid=""
sleep 1

# ========================================
# Cross-Language Tests: C++ Client + Go Server (WebSocket)
# ========================================
echo -e "\n${YELLOW}Running WebSocket tests (C++ Client + Go Server)...${NC}"

# Build and start Go WebSocket server
go build -o pingpong_ws ../test/pingpong_server_ws/main.go
./pingpong_ws > server.log 2>&1 &
pid=$!
echo "Started Go WebSocket server (PID: $pid)"

# Wait for server to start
sleep 1

if ! kill -0 $pid 2>/dev/null; then
	echo -e "${RED}Failed to start Go WebSocket server${NC}"
	exit 1
fi

# Run C++ client tests
run_with_timeout "C++ Client + Go Server tests" ./client_ws_tests
status=$?

# Stop server
kill $pid 2>/dev/null || true
wait $pid 2>/dev/null || true
pid=""

if [ $status -eq 0 ]; then
	echo -e "${GREEN}WebSocket tests (C++ Client + Go Server) passed${NC}"
else
	echo -e "${RED}WebSocket tests (C++ Client + Go Server) failed${NC}"
	exit 1
fi

sleep 1

# ========================================
# Cross-Language Tests: C++ Client + Go Server (WebSocket TLS)
# ========================================
echo -e "\n${YELLOW}Running WebSocket TLS tests (C++ Client + Go Server)...${NC}"

# Build and start Go WebSocket TLS server
go build -o pingpong_ws_tls ../test/pingpong_server_ws_tls/main.go
./pingpong_ws_tls --cert=../test/server.crt --key=../test/server.key > server.log 2>&1 &
pid=$!
echo "Started Go WebSocket TLS server (PID: $pid)"

# Wait for server to start
sleep 1

if ! kill -0 $pid 2>/dev/null; then
	echo -e "${RED}Failed to start Go WebSocket TLS server${NC}"
	exit 1
fi

# Run C++ client TLS tests
run_with_timeout "C++ Client + Go Server TLS tests" ./client_ws_tls_tests
status=$?

# Stop server
kill $pid 2>/dev/null || true
wait $pid 2>/dev/null || true
pid=""

if [ $status -eq 0 ]; then
	echo -e "${GREEN}WebSocket TLS tests (C++ Client + Go Server) passed${NC}"
else
	echo -e "${RED}WebSocket TLS tests (C++ Client + Go Server) failed${NC}"
	exit 1
fi

sleep 1

# ========================================
# All Tests Complete
# ========================================
echo -e "\n${GREEN}All WebSocket tests passed!${NC}"
