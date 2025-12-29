#!/bin/bash

# Generate benchmark message and service code
echo "Generating benchmark code..."

# Generate Go code
go run ./cmd/scg-go/main.go \
	--base-package="github.com/kbirk/scg/benchmark/scg/generated/benchmark" \
	--input="./benchmark/scg/benchmark" \
	--output="./benchmark/scg/generated/benchmark"

# Generate C++ code
go run ./cmd/scg-cpp/main.go \
	--input="./benchmark/scg/benchmark" \
	--output="./benchmark/scg/generated/benchmark"

echo "Benchmark code generation complete!"
