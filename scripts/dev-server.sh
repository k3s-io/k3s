#!/bin/bash
set -e

cd $(dirname $0)/../bin

echo Running
go run -tags k3s ../cli/main.go --debug server --disable-agent
