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
# Go TCP Tests
# ========================================
echo -e "${YELLOW}Running Go TCP tests...${NC}"
run_with_timeout "Go TCP tests" go test -v -count=1 ./test/test_go/service_tcp_test.go ./test/test_go/test_utils.go
if [ $? -eq 0 ]; then
	echo -e "${GREEN}Go TCP tests passed${NC}"
else
	echo -e "${RED}Go TCP tests failed${NC}"
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
# C++ TCP Tests
# ========================================
echo -e "\n${YELLOW}Running C++ TCP tests...${NC}"

# Start server
./server_tcp_test > server.log 2>&1 &
pid=$!
echo "Started C++ TCP server (PID: $pid)"

# Wait for server to start
sleep 1

# Run client tests
run_with_timeout "C++ TCP client tests" ./client_tcp_tests
status=$?

# Stop server
kill $pid
wait $pid 2>/dev/null || true
pid=""

if [ $status -eq 0 ]; then
	echo -e "${GREEN}C++ TCP tests passed${NC}"
else
	echo -e "${RED}C++ TCP tests failed${NC}"
	exit 1
fi
