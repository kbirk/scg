#!/bin/bash

# install third party deps
cd ./third_party && ./install.sh && cd ..

# generate test files
go run ./cmd/scg-go/main.go --base-package="github.com/kbirk/scg/test/pingpong" --input="./test/data/pingpong" --output="./test/generated/pingpong"
go run ./cmd/scg-go/main.go --base-package="github.com/kbirk/scg/test/basic" --input="./test/data/basic" --output="./test/generated/basic"
go run ./cmd/scg-cpp/main.go --input="./test/data/pingpong" --output="./test/generated/pingpong"
go run ./cmd/scg-cpp/main.go --input="./test/data/basic" --output="./test/generated/basic"
