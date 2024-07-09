#!/bin/bash

set -e

# go tests
go test -count=1 ./...

# build cpp tests
mkdir -p .build && cd .build && cmake ../test/test_cpp && make

# ~~~~~~~~~~~~~~~~~~

# run cpp tests
./serialize_tests
./uuid_tests
# ~~~~~~~~~~~~~~~~~~

# start pingpong server
go build -o pingpong ../test/pingpong_server/main.go
./pingpong > output.txt 2>&1 &
pid=$!
sleep 1

if ! ./client_no_tls_tests; then
	kill $pid
	exit 1
fi

# kill pingpong server
kill $pid
sleep 1

# ~~~~~~~~~~~~~~~~~~

# start pingpong server
go build -o pingpong_tls ../test/pingpong_server_tls/main.go
./pingpong_tls --cert="../test/server.crt" --key="../test/server.key" > output.txt 2>&1 &
pid=$!
sleep 1

if ! ./client_tls_tests; then
	kill $pid
	exit 1
fi

# kill pingpong server
kill $pid
sleep 1
