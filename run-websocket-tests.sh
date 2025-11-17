#!/bin/bash

set -e

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
# Go WebSocket Tests
# ========================================
echo -e "${YELLOW}Running Go WebSocket tests...${NC}"
go test -v -count=1 ./test/test_go/service_websocket_test.go ./test/test_go/service_websocket_edge_test.go
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
mkdir -p .build && cd .build && cmake ../test/test_cpp && make
if [ $? -eq 0 ]; then
	echo -e "${GREEN}C++ WebSocket tests built successfully${NC}"
else
	echo -e "${RED}Failed to build C++ WebSocket tests${NC}"
	exit 1
fi

# ========================================
# WebSocket No TLS Tests
# ========================================
echo -e "\n${YELLOW}Running WebSocket (no TLS) tests...${NC}"

# Build and start pingpong server
go build -o pingpong ../test/pingpong_server/main.go
./pingpong > output.txt 2>&1 &
pid=$!
sleep 1

if ! kill -0 $pid 2>/dev/null; then
	echo -e "${RED}Failed to start pingpong server${NC}"
	exit 1
fi

echo -e "${GREEN}PingPong server started (pid: $pid)${NC}"

# Run client tests
if ./client_no_tls_tests; then
	echo -e "${GREEN}WebSocket (no TLS) tests passed${NC}"
else
	echo -e "${RED}WebSocket (no TLS) tests failed${NC}"
	kill $pid 2>/dev/null || true
	exit 1
fi

# Stop server
kill $pid 2>/dev/null || true
wait $pid 2>/dev/null || true
pid=""
sleep 1

# ========================================
# WebSocket TLS Tests
# ========================================
echo -e "\n${YELLOW}Running WebSocket (TLS) tests...${NC}"

# Build and start pingpong server with TLS
go build -o pingpong_tls ../test/pingpong_server_tls/main.go
./pingpong_tls --cert="../test/server.crt" --key="../test/server.key" > output.txt 2>&1 &
pid=$!
sleep 1

if ! kill -0 $pid 2>/dev/null; then
	echo -e "${RED}Failed to start pingpong TLS server${NC}"
	exit 1
fi

echo -e "${GREEN}PingPong TLS server started (pid: $pid)${NC}"

# Run client TLS tests
if ./client_tls_tests; then
	echo -e "${GREEN}WebSocket (TLS) tests passed${NC}"
else
	echo -e "${RED}WebSocket (TLS) tests failed${NC}"
	kill $pid 2>/dev/null || true
	exit 1
fi

# Stop server
kill $pid 2>/dev/null || true
wait $pid 2>/dev/null || true
pid=""
sleep 1

# ========================================
# All Tests Complete
# ========================================
echo -e "\n${GREEN}All WebSocket tests passed!${NC}"
