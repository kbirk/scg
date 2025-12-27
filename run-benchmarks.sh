#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# ========================================
# Go Benchmarks
# ========================================
echo -e "${YELLOW}Running Go benchmarks...${NC}"
go test -bench=. -benchmem ./benchmarks
if [ $? -eq 0 ]; then
	echo -e "${GREEN}Go benchmarks completed successfully${NC}"
else
	echo -e "${RED}Go benchmarks failed${NC}"
	exit 1
fi
