#!/bin/bash

set -e
set -o pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Setup logging
TIMESTAMP=$(date +"%Y%m%d-%H%M%S")
RAW_OUTPUT="/tmp/scg-benchmark-${TIMESTAMP}.txt"
REPORT_DIR="./benchmark/reports"
mkdir -p "$REPORT_DIR"

echo "Benchmark run started at ${TIMESTAMP}" > "$RAW_OUTPUT"

# ========================================
# Go Benchmarks
# ========================================
# echo -e "${YELLOW}Running Go benchmarks...${NC}"
# go test -bench=. -benchmem ./benchmark/go | tee -a "$RAW_OUTPUT"
# if [ $? -eq 0 ]; then
# 	echo -e "${GREEN}Go benchmarks completed successfully${NC}"
#     echo "Go benchmarks completed successfully" >> "$RAW_OUTPUT"
# else
# 	echo -e "${RED}Go benchmarks failed${NC}"
# 	exit 1
# fi

# ========================================
# C++ Benchmarks
# ========================================
echo -e "\n${YELLOW}Building C++ benchmarks...${NC}"
echo "Building C++ benchmarks..." >> "$RAW_OUTPUT"
mkdir -p ./benchmark/cpp/build
cd ./benchmark/cpp/build
cmake ..
cmake --build . -j$(nproc)

echo -e "\n${YELLOW}Running C++ Varint benchmarks...${NC}"
echo "Running C++ Varint benchmarks..." >> "$RAW_OUTPUT"
./varint_bench | tee -a "$RAW_OUTPUT"

echo -e "\n${YELLOW}Running C++ Serialize benchmarks...${NC}"
echo "Running C++ Serialize benchmarks..." >> "$RAW_OUTPUT"
./serialize_bench | tee -a "$RAW_OUTPUT"

echo -e "\n${YELLOW}Running C++ Message benchmarks...${NC}"
echo "Running C++ Message benchmarks..." >> "$RAW_OUTPUT"
./message_bench | tee -a "$RAW_OUTPUT"

echo -e "\n${YELLOW}Running C++ RPC benchmarks...${NC}"
echo "Running C++ RPC benchmarks..." >> "$RAW_OUTPUT"
./rpc_bench | tee -a "$RAW_OUTPUT"

# ========================================
# Analysis
# ========================================
cd ../../.. # Return to root
echo -e "\n${YELLOW}Generating Benchmark Report...${NC}"
python3 ./benchmark/scripts/summarize.py --input "$RAW_OUTPUT" --output-dir "$REPORT_DIR"

echo -e "\n${YELLOW}Comparing with previous run...${NC}"
python3 ./benchmark/scripts/compare.py --report-dir "$REPORT_DIR"
