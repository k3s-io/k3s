#!/bin/bash
set -e

mkdir -p $(dirname $0)/../bin
cd $(dirname $0)/../bin

echo Running
ARGS="--disable-agent"
if echo -- "$@" | grep -q rootless; then
    ARGS=""
    PATH=$(pwd):$PATH
fi
go run -tags "apparmor" ../main.go server $ARGS "$@"
