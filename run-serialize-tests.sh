#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# ========================================
# Go Serialization Tests
# ========================================
echo -e "${YELLOW}Running Go serialization tests...${NC}"
go test -v -count=1 ./test/test_go -run "Serialize"
if [ $? -eq 0 ]; then
	echo -e "${GREEN}Go serialization tests passed${NC}"
else
	echo -e "${RED}Go serialization tests failed${NC}"
	exit 1
fi

# ========================================
# Build C++ Tests
# ========================================
echo -e "\n${YELLOW}Building C++ serialization tests...${NC}"
mkdir -p .build
cd .build
cmake ../test/test_cpp
# Build only serialization-related targets
cmake --build . --target serialize_tests --target uuid_tests --target macro_test
if [ $? -eq 0 ]; then
	echo -e "${GREEN}C++ serialization tests built successfully${NC}"
else
	echo -e "${RED}Failed to build C++ serialization tests${NC}"
	exit 1
fi

# ========================================
# C++ Serialization Tests
# ========================================
echo -e "\n${YELLOW}Running C++ serialization tests...${NC}"

echo "  - serialize_tests"
./serialize_tests
if [ $? -eq 0 ]; then
	echo -e "${GREEN}C++ serialize_tests passed${NC}"
else
	echo -e "${RED}C++ serialize_tests failed${NC}"
	exit 1
fi

echo "  - uuid_tests"
./uuid_tests
if [ $? -eq 0 ]; then
	echo -e "${GREEN}C++ uuid_tests passed${NC}"
else
	echo -e "${RED}C++ uuid_tests failed${NC}"
	exit 1
fi

# ========================================
# All Tests Complete
# ========================================
echo -e "\n${GREEN}All serialization tests passed!${NC}"
