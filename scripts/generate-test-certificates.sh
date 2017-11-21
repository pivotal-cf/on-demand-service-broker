#!/usr/bin/env bash

set -eu

go_root=$(go env GOROOT)
go run "${go_root}/src/crypto/tls/generate_cert.go"  --rsa-bits 1024 --host 127.0.0.1,::1,example.com --ca --start-date "Jan 1 00:00:00 1970" --duration=1000000h

mv cert.pem key.pem ./integration_tests/fixtures/ssl/
