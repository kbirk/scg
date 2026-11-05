#!/bin/bash
# Generates the self-signed test server key and certificate (test/server.key,
# test/server.crt; both gitignored). Non-interactive.
set -euo pipefail

cd "$(dirname "$0")"

openssl genrsa -out ./test/server.key 4096
openssl req -new -x509 -sha256 -key ./test/server.key -out ./test/server.crt -days 3650 \
	-subj "/CN=localhost"
