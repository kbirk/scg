#!/bin/bash

set -e

TIMEOUT_SECONDS=${SCG_TEST_TIMEOUT:-120}

run_with_timeout() {
	local description="$1"
	shift
	timeout --foreground "${TIMEOUT_SECONDS}" "$@"
	local status=$?
	if [ $status -eq 0 ]; then
		return 0
	fi
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
# Go TCP Tests (Go Client + Go Server)
# ========================================
echo -e "${YELLOW}Running Go TCP tests (Go Client + Go Server)...${NC}"
run_with_timeout "Go TCP tests" go test -v -count=1 -run "^(TestTCP|TestTCPTLS)$" ./test/go/service_tcp_test.go ./test/go/service_test_suite.go ./test/go/test_utils.go
if [ $? -eq 0 ]; then
	echo -e "${GREEN}Go TCP tests passed${NC}"
else
	echo -e "${RED}Go TCP tests failed${NC}"
	exit 1
fi

# ========================================
# Build C++ Tests
# ========================================
echo -e "\n${YELLOW}Building C++ TCP tests...${NC}"
mkdir -p ./test/cpp/build
cd ./test/cpp/build
cmake ../
# Build only TCP-related targets
make tcp_tests server_tcp_test server_tcp_tls_test client_tcp_tests client_tcp_tls_tests
if [ $? -eq 0 ]; then
	echo -e "${GREEN}C++ TCP tests built successfully${NC}"
else
	echo -e "${RED}Failed to build C++ TCP tests${NC}"
	exit 1
fi

# ========================================
# C++ TCP Tests  (C++ Client + C++ Server)
# ========================================
echo -e "\n${YELLOW}Running C++ TCP tests (C++ Client + C++ Server)...${NC}"

run_with_timeout "C++ TCP tests" ./tcp_tests
status=$?

if [ $status -eq 0 ]; then
	echo -e "${GREEN}C++ TCP tests passed${NC}"
else
	echo -e "${RED}C++ TCP tests failed${NC}"
	exit 1
fi

sleep 1

# ========================================
# Cross-Language Tests: Go Client + C++ Server (TCP)
# ========================================
echo -e "\n${YELLOW}Running TCP tests (Go Client + C++ Server)...${NC}"

# Start C++ TCP server
./server_tcp_test > server.log 2>&1 &
pid=$!
echo "Started C++ TCP server (PID: $pid)"

# Wait for server to start
sleep 1

if ! kill -0 $pid 2>/dev/null; then
	echo -e "${RED}Failed to start C++ TCP server${NC}"
	exit 1
fi

# Run Go client tests with external server option
run_with_timeout "Go Client + C++ Server tests" go test -v -count=1 -run TestTCPExternalServer ../../../test/go/service_tcp_test.go ../../../test/go/service_test_suite.go ../../../test/go/test_utils.go
status=$?

# Stop server
kill $pid 2>/dev/null || true
wait $pid 2>/dev/null || true
pid=""

if [ $status -eq 0 ]; then
	echo -e "${GREEN}TCP tests (Go Client + C++ Server) passed${NC}"
else
	echo -e "${RED}TCP tests (Go Client + C++ Server) failed${NC}"
	exit 1
fi

sleep 1

# ========================================
# Cross-Language Tests: Go Client + C++ Server (TCP TLS)
# ========================================
echo -e "\n${YELLOW}Running TCP TLS tests (Go Client + C++ Server)...${NC}"

# Start C++ TCP TLS server
./server_tcp_tls_test > server.log 2>&1 &
pid=$!
echo "Started C++ TCP TLS server (PID: $pid)"

# Wait for server to start
sleep 1

if ! kill -0 $pid 2>/dev/null; then
	echo -e "${RED}Failed to start C++ TCP TLS server${NC}"
	exit 1
fi

# Run Go client tests with external server option
run_with_timeout "Go Client + C++ TLS Server tests" go test -v -count=1 -run TestTCPTLSExternalServer ../../../test/go/service_tcp_test.go ../../../test/go/service_test_suite.go ../../../test/go/test_utils.go
status=$?

# Stop server
kill $pid 2>/dev/null || true
wait $pid 2>/dev/null || true
pid=""

if [ $status -eq 0 ]; then
	echo -e "${GREEN}TCP TLS tests (Go Client + C++ Server) passed${NC}"
else
	echo -e "${RED}TCP TLS tests (Go Client + C++ Server) failed${NC}"
	exit 1
fi

# ========================================
# TCP Tests (C++ Client + Go Server)
# ========================================
echo -e "\n${YELLOW}Running TCP tests (C++ Client + Go Server)...${NC}"

# Build and start Go TCP server
go build -o pingpong_tcp ../../../test/go/pingpong_server_tcp/main.go
./pingpong_tcp > server.log 2>&1 &
pid=$!
echo "Started Go TCP server (PID: $pid)"

# Wait for server to start
sleep 1

if ! kill -0 $pid 2>/dev/null; then
	echo -e "${RED}Failed to start Go TCP server${NC}"
	exit 1
fi

# Run C++ client tests
run_with_timeout "C++ Client + Go Server tests" ./client_tcp_tests
status=$?

# Stop server
kill $pid 2>/dev/null || true
wait $pid 2>/dev/null || true
pid=""

if [ $status -eq 0 ]; then
	echo -e "${GREEN}TCP tests (C++ Client + Go Server) passed${NC}"
else
	echo -e "${RED}TCP tests (C++ Client + Go Server) failed${NC}"
	exit 1
fi

sleep 1

# ========================================
# TCP TLS Tests (C++ Client + Go Server)
# ========================================
echo -e "\n${YELLOW}Running TCP TLS tests (C++ Client + Go Server)...${NC}"

# Build and start Go TCP TLS server
go build -o pingpong_tcp_tls ../../../test/go/pingpong_server_tcp_tls/main.go
./pingpong_tcp_tls --cert=../../../test/server.crt --key=../../../test/server.key > server.log 2>&1 &
pid=$!
echo "Started Go TCP TLS server (PID: $pid)"

# Wait for server to start
sleep 1

if ! kill -0 $pid 2>/dev/null; then
	echo -e "${RED}Failed to start Go TCP TLS server${NC}"
	exit 1
fi

# Run C++ client TLS tests
run_with_timeout "C++ Client + Go Server TLS tests" ./client_tcp_tls_tests
status=$?

# Stop server
kill $pid 2>/dev/null || true
wait $pid 2>/dev/null || true
pid=""

if [ $status -eq 0 ]; then
	echo -e "${GREEN}TCP TLS tests (C++ Client + Go Server) passed${NC}"
else
	echo -e "${RED}TCP TLS tests (C++ Client + Go Server) failed${NC}"
	exit 1
fi

# ========================================
# All Tests Complete
# ========================================
echo -e "\n${GREEN}All TCP tests passed!${NC}"
