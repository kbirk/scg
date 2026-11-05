#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Run from the repository root, wherever the script is invoked from
cd "$(dirname "$0")"
ROOT_DIR="$(pwd)"

# ========================================
# Generate test code (Go + C++). The generated code is gitignored, so the
# runner regenerates it to stay self-contained.
# ========================================
echo -e "${YELLOW}Generating test code...${NC}"
if ! ./gen-test-code.sh > /dev/null 2>&1; then
	echo -e "${RED}Test code generation failed${NC}"
	exit 1
fi

# ========================================
# Go Package Unit Tests (serialize + rpc), under the race detector
# ========================================
echo -e "${YELLOW}Running Go package unit tests (-race)...${NC}"
go test -race -count=1 ./pkg/...
if [ $? -eq 0 ]; then
	echo -e "${GREEN}Go package unit tests passed${NC}"
else
	echo -e "${RED}Go package unit tests failed${NC}"
	exit 1
fi

# ========================================
# Go Serialization Tests
# ========================================
echo -e "${YELLOW}Running Go serialization tests...${NC}"
go test -v -count=1 ./test/go -run "Serialize"
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
mkdir -p ./test/cpp/build
cd ./test/cpp/build
cmake ../
# Build only serialization-related targets
cmake --build . -j$(nproc) --target serialize_tests --target uuid_tests --target error_tests --target macro_test --target api_fake_tests --target middleware_tests --target downstream_compat_tests --target client_unit_tests
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

echo "  - error_tests"
./error_tests
if [ $? -eq 0 ]; then
	echo -e "${GREEN}C++ error_tests passed${NC}"
else
	echo -e "${RED}C++ error_tests failed${NC}"
	exit 1
fi

echo "  - api_fake_tests"
./api_fake_tests
if [ $? -eq 0 ]; then
	echo -e "${GREEN}C++ api_fake_tests passed${NC}"
else
	echo -e "${RED}C++ api_fake_tests failed${NC}"
	exit 1
fi

for test_bin in macro_test middleware_tests downstream_compat_tests client_unit_tests; do
	echo "  - $test_bin"
	if ./$test_bin; then
		echo -e "${GREEN}C++ $test_bin passed${NC}"
	else
		echo -e "${RED}C++ $test_bin failed${NC}"
		exit 1
	fi
done

# ========================================
# Cross-language wire compatibility (Go <-> C++)
# ========================================
echo -e "\n${YELLOW}Running cross-language wire compatibility check...${NC}"
cd "$ROOT_DIR"
WIRE_DIR="$(mktemp -d)"
trap 'rm -rf "$WIRE_DIR"' EXIT
go run ./internal/wirecheck encode "$WIRE_DIR/go.txt"
${CXX:-g++} -std=c++17 -Wall -Wextra -Werror -Wpedantic -Wno-unused-parameter -O1 \
	-I./include -I./test/scg/generated -isystem ./third_party/include \
	./internal/wirecheck/cpp/wire_check.cpp -o "$WIRE_DIR/wire_check"
if ! "$WIRE_DIR/wire_check" "$WIRE_DIR/go.txt" "$WIRE_DIR/cpp.txt"; then
	echo -e "${RED}C++ decode of Go encodings failed${NC}"
	exit 1
fi
if ! go run ./internal/wirecheck verify "$WIRE_DIR/cpp.txt"; then
	echo -e "${RED}Go decode of C++ encodings failed${NC}"
	exit 1
fi
echo -e "${GREEN}Cross-language wire compatibility check passed${NC}"

# ========================================
# Cross-language JSON compatibility (Go <-> C++)
# ========================================
# Go ToJSON -> C++ fromJSON -> C++ toJSON -> Go FromJSON, and the reverse
# starting from C++'s own toJSON; values and JSON texts are compared.
echo -e "\n${YELLOW}Running cross-language JSON compatibility check...${NC}"
go run ./internal/wirecheck json-encode "$WIRE_DIR/go_json.txt"
if ! "$WIRE_DIR/wire_check" json-from-go "$WIRE_DIR/go_json.txt" "$WIRE_DIR/cpp_json_rt.txt" "$WIRE_DIR/cpp_json_native.txt"; then
	echo -e "${RED}C++ decode of Go JSON failed${NC}"
	exit 1
fi
if ! go run ./internal/wirecheck json-verify "$WIRE_DIR/cpp_json_rt.txt" "$WIRE_DIR/cpp_json_native.txt" "$WIRE_DIR/go_json_rt.txt"; then
	echo -e "${RED}Go decode of C++ JSON failed${NC}"
	exit 1
fi
if ! "$WIRE_DIR/wire_check" json-from-roundtrip "$WIRE_DIR/go_json_rt.txt"; then
	echo -e "${RED}C++ decode of Go's re-encoded JSON failed${NC}"
	exit 1
fi
echo -e "${GREEN}Cross-language JSON compatibility check passed${NC}"

# ========================================
# All Tests Complete
# ========================================
echo -e "\n${GREEN}All serialization tests passed!${NC}"
