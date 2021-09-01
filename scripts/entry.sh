#!/bin/bash
set -e

mkdir -p bin dist

ARGS=("$@")
if [ -e "./scripts/${ARGS[-1]}" ]; then
    ./scripts/"${ARGS[-1]}"
else
    exec "${ARGS[-1]}"
fi

chown -R "$DAPPER_UID":"$DAPPER_GID" .
