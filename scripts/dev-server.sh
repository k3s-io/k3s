#!/bin/bash
set -e

mkdir -p $(dirname $0)/../bin
cd $(dirname $0)/../bin

echo Running
go run -tags "apparmor" ../main.go --debug server --disable-agent
