#!/bin/bash

openssl genrsa -out ./test/server.key 4096
openssl req -new -x509 -sha256 -key ./test/server.key -out ./test/server.crt -days 3650
