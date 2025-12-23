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
	# Clean up socket files
	rm -f /tmp/scg_test_unix*.sock
}

# Register cleanup function to run on script exit
trap cleanup EXIT INT TERM

# ========================================
# Go Unix Socket Tests (Go Client + Go Server)
# ========================================
echo -e "${YELLOW}Running Go Unix socket tests (Go Client + Go Server)...${NC}"
run_with_timeout "Go Unix socket tests" go test -v -count=1 -run "^TestUnix$" ./test/test_go/service_unix_test.go ./test/test_go/service_test_suite.go ./test/test_go/test_utils.go
if [ $? -eq 0 ]; then
	echo -e "${GREEN}Go Unix socket tests passed${NC}"
else
	echo -e "${RED}Go Unix socket tests failed${NC}"
	exit 1
fi

# ========================================
# Build C++ Tests
# ========================================
echo -e "\n${YELLOW}Building C++ tests...${NC}"
mkdir -p .build && cd .build && cmake ../test/test_cpp && make
if [ $? -eq 0 ]; then
	echo -e "${GREEN}C++ tests built successfully${NC}"
else
	echo -e "${RED}Failed to build C++ tests${NC}"
	exit 1
fi

# ========================================
# C++ Unix Socket Tests (C++ Client + C++ Server)
# ========================================
echo -e "\n${YELLOW}Running C++ Unix socket tests (C++ Client + C++ Server)...${NC}"

run_with_timeout "C++ Unix socket tests" ./unix_tests
status=$?

if [ $status -eq 0 ]; then
	echo -e "${GREEN}C++ Unix socket tests passed${NC}"
else
	echo -e "${RED}C++ Unix socket tests failed${NC}"
	exit 1
fi

sleep 1

# ========================================# C++ Unix Streaming Tests (C++ Client + C++ Server)
# ========================================
echo -e "\n${YELLOW}Running C++ Unix Streaming tests (C++ Client + C++ Server)...${NC}"

run_with_timeout "C++ Unix Streaming tests" ./streaming_unix_tests
status=$?

if [ $status -eq 0 ]; then
	echo -e "${GREEN}C++ Unix Streaming tests passed${NC}"
else
	echo -e "${RED}C++ Unix Streaming tests failed${NC}"
	exit 1
fi

sleep 1

# ========================================# Cross-Language Tests: Go Client + C++ Server (Unix)
# ========================================
echo -e "\n${YELLOW}Running Unix socket tests (Go Client + C++ Server)...${NC}"

# Start C++ Unix socket server
./server_unix_test > server.log 2>&1 &
pid=$!
echo "Started C++ Unix socket server (PID: $pid)"

# Wait for server to start
sleep 1

if ! kill -0 $pid 2>/dev/null; then
	echo -e "${RED}Failed to start C++ Unix socket server${NC}"
	exit 1
fi

# Run Go client tests with external server option
run_with_timeout "Go Client + C++ Server tests" go test -v -count=1 -run TestUnixExternalServer ../test/test_go/service_unix_test.go ../test/test_go/service_test_suite.go ../test/test_go/test_utils.go
status=$?

# Stop server
kill $pid 2>/dev/null || true
wait $pid 2>/dev/null || true
pid=""

if [ $status -eq 0 ]; then
	echo -e "${GREEN}Unix socket tests (Go Client + C++ Server) passed${NC}"
else
	echo -e "${RED}Unix socket tests (Go Client + C++ Server) failed${NC}"
	exit 1
fi

sleep 1

# ========================================
# Cross-Language Tests: C++ Client + Go Server (Unix)
# ========================================
echo -e "\n${YELLOW}Running Unix socket tests (C++ Client + Go Server)...${NC}"

# Build and start Go Unix server
go build -o pingpong_unix ../test/pingpong_server_unix/main.go
./pingpong_unix > server.log 2>&1 &
pid=$!
echo "Started Go Unix socket server (PID: $pid)"

# Wait for server to start
sleep 1

if ! kill -0 $pid 2>/dev/null; then
	echo -e "${RED}Failed to start Go Unix socket server${NC}"
	exit 1
fi

# Run C++ client tests
run_with_timeout "C++ Client + Go Server tests" ./client_unix_tests
status=$?

# Stop server
kill $pid 2>/dev/null || true
wait $pid 2>/dev/null || true
pid=""

if [ $status -eq 0 ]; then
	echo -e "${GREEN}Unix socket tests (C++ Client + Go Server) passed${NC}"
else
	echo -e "${RED}Unix socket tests (C++ Client + Go Server) failed${NC}"
	exit 1
fi

# ========================================
# All Tests Complete
# ========================================
echo -e "\n${GREEN}All Unix socket tests passed!${NC}"
