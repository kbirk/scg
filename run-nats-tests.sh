#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
CONTAINER_NAME="scg-nats-test"
NATS_PORT=4222
NATS_IMAGE="nats:latest"

echo -e "${YELLOW}Starting NATS server in Docker...${NC}"

# Check if container already exists and remove it
if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo "Removing existing container..."
    docker rm -f ${CONTAINER_NAME} > /dev/null 2>&1
fi

# Start NATS server
docker run -d \
    --name ${CONTAINER_NAME} \
    -p ${NATS_PORT}:4222 \
    ${NATS_IMAGE} > /dev/null

if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to start NATS server${NC}"
    exit 1
fi

echo -e "${GREEN}NATS server started on port ${NATS_PORT}${NC}"

# Wait for NATS to be ready
echo "Waiting for NATS to be ready..."
sleep 2

# Function to cleanup
cleanup() {
    echo -e "\n${YELLOW}Stopping NATS server...${NC}"
    docker stop ${CONTAINER_NAME} > /dev/null 2>&1
    docker rm ${CONTAINER_NAME} > /dev/null 2>&1
    echo -e "${GREEN}NATS server stopped and removed${NC}"
}

# ========================================
# Go NATS Tests
# ========================================
trap cleanup EXIT INT TERM
echo -e "${YELLOW}Running NATS tests...${NC}"

go test -v -count=1 ./test/test_go/service_nats_test.go ./test/test_go/service_nats_edge_test.go

TEST_RESULT=$?

if [ $TEST_RESULT -eq 0 ]; then
    echo -e "${GREEN}All NATS tests passed!${NC}"
else
    echo -e "${RED}NATS tests failed${NC}"
fi

exit $TEST_RESULT
