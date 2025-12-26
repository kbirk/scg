#!/bin/bash

# Generate benchmark message and service code
echo "Generating benchmark code..."

# Generate Go code
go run ./cmd/scg-go/main.go \
	--base-package="github.com/kbirk/scg/benchmarks/output/benchmark" \
	--input="./benchmarks/input" \
	--output="./benchmarks/output/benchmark"

echo "Benchmark code generation complete!"
