#!/bin/bash
set -e

cd $(dirname $0)/../bin

echo Running
go run -tags "apparmor" ../cmd/server/main.go --debug server --disable-agent
