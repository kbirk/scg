#!/bin/bash

set -e

# generate test files
go run ./cmd/scg-go/main.go --base-package="github.com/kbirk/scg/test/scg/generated/pingpong" --input="./test/scg/pingpong" --output="./test/scg/generated/pingpong"
go run ./cmd/scg-go/main.go --base-package="github.com/kbirk/scg/test/scg/generated/basic" --input="./test/scg/basic" --output="./test/scg/generated/basic"
go run ./cmd/scg-go/main.go --base-package="github.com/kbirk/scg/test/scg/generated/sample" --input="./test/scg/sample" --output="./test/scg/generated/sample"
go run ./cmd/scg-go/main.go --base-package="github.com/kbirk/scg/test/scg/generated/wire" --input="./test/scg/wire" --output="./test/scg/generated/wire"
go run ./cmd/scg-cpp/main.go --input="./test/scg/pingpong" --output="./test/scg/generated/pingpong"
go run ./cmd/scg-cpp/main.go --input="./test/scg/basic" --output="./test/scg/generated/basic"
go run ./cmd/scg-cpp/main.go --input="./test/scg/sample" --output="./test/scg/generated/sample"
go run ./cmd/scg-cpp/main.go --input="./test/scg/wire" --output="./test/scg/generated/wire"
