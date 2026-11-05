#!/bin/bash

# Code generator checks:
#   1. generator unit, golden-file, corpus-compile and generated-runtime tests
#   2. regenerate the test / benchmark code and `go vet` everything
#   3. compile every generated C++ header on its own
#   4. generate twice more into temporary directories and require identical
#      output (determinism), equal to the checked-in-tree generation

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

cd "$(dirname "$0")"
ROOT_DIR="$(pwd)"
CXX="${CXX:-g++}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo -e "${YELLOW}Running generator tests...${NC}"
go test -count=1 ./internal/...
echo -e "${GREEN}Generator tests passed${NC}"

echo -e "\n${YELLOW}Generating test and benchmark code...${NC}"
./gen-test-code.sh > /dev/null
./gen-benchmark-code.sh > /dev/null
echo -e "${GREEN}Generated${NC}"

# pkg/serialize's Reader.ReadByte / Writer.WriteByte predate this check and
# deliberately differ from io.ByteReader / io.ByteWriter, so the stdmethods
# analyzer is disabled.
echo -e "\n${YELLOW}Running go vet...${NC}"
go vet -stdmethods=false ./...
echo -e "${GREEN}go vet passed${NC}"

echo -e "\n${YELLOW}Compiling each generated C++ header on its own...${NC}"
failed=0
for header in $(find test/scg/generated benchmark/scg/generated -name '*.h' | sort); do
	echo "#include \"$ROOT_DIR/$header\"" > "$TMP_DIR/tu.cpp"
	if ! "$CXX" -std=c++17 -Wall -Wextra -Werror -Wpedantic -Wno-unused-parameter -fsyntax-only \
		-I./include -isystem ./third_party/include "$TMP_DIR/tu.cpp" 2> "$TMP_DIR/err.txt"; then
		echo -e "${RED}  $header failed to compile${NC}"
		head -20 "$TMP_DIR/err.txt"
		failed=1
	else
		echo "  $header"
	fi
done
if [ $failed -ne 0 ]; then
	echo -e "${RED}Generated C++ headers failed to compile${NC}"
	exit 1
fi
echo -e "${GREEN}All generated C++ headers compile${NC}"

echo -e "\n${YELLOW}Checking that generation is deterministic...${NC}"
generate() {
	local out="$1"
	for d in pingpong basic sample wire; do
		go run ./cmd/scg-go --base-package="github.com/kbirk/scg/test/scg/generated/$d" --input="./test/scg/$d" --output="$out/test/$d" > /dev/null
		go run ./cmd/scg-cpp --input="./test/scg/$d" --output="$out/test/$d" > /dev/null
	done
	go run ./cmd/scg-go --base-package="github.com/kbirk/scg/benchmark/scg/generated/benchmark" --input="./benchmark/scg/benchmark" --output="$out/benchmark/benchmark" > /dev/null
	go run ./cmd/scg-cpp --input="./benchmark/scg/benchmark" --output="$out/benchmark/benchmark" > /dev/null
}
generate "$TMP_DIR/a"
generate "$TMP_DIR/b"
if ! diff -r "$TMP_DIR/a" "$TMP_DIR/b" > "$TMP_DIR/diff.txt"; then
	echo -e "${RED}Two generations of the same input differ:${NC}"
	head -40 "$TMP_DIR/diff.txt"
	exit 1
fi
for d in pingpong basic sample wire; do
	if ! diff -r "$TMP_DIR/a/test/$d" "test/scg/generated/$d" > "$TMP_DIR/diff.txt"; then
		echo -e "${RED}test/scg/generated/$d differs from a fresh generation:${NC}"
		head -40 "$TMP_DIR/diff.txt"
		exit 1
	fi
done
echo -e "${GREEN}Generation is deterministic${NC}"

echo -e "\n${GREEN}All code generator checks passed!${NC}"
