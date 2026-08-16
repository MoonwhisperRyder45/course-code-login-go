#!/bin/sh
set -eu

go test ./...
go build -o /tmp/course-code-login ./cmd/course-login
echo "tests passed; single binary built at /tmp/course-code-login"
