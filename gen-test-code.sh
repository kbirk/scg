#!/bin/bash

# generate test files
go run ./cmd/scg-go/main.go --base-package="github.com/kbirk/scg/test/pingpong" --input="./test/files/input/pingpong" --output="./test/files/output/pingpong"
go run ./cmd/scg-go/main.go --base-package="github.com/kbirk/scg/test/basic" --input="./test/files/input/basic" --output="./test/files/output/basic"
go run ./cmd/scg-cpp/main.go --input="./test/files/input/pingpong" --output="./test/files/output/pingpong"
go run ./cmd/scg-cpp/main.go --input="./test/files/input/basic" --output="./test/files/output/basic"
