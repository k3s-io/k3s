#!/bin/bash
set -e

GO=${GO-go}

mkdir -p $(dirname $0)/../bin
cd $(dirname $0)/../bin

echo Running
ARGS="--disable-agent"
if echo -- "$@" | grep -q rootless; then
    ARGS=""
    PATH=$(pwd):$PATH
fi
"${GO}" run -tags "apparmor" ../main.go server $ARGS "$@"
