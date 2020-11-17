#!/bin/bash

set -e

if [ "${DEBUG}" = 1 ]; then
    set -x
fi

. ./scripts/version.sh

MAX_BINARY_SIZE=61000000
BIN_SUFFIX="-${ARCH}"
if [ ${ARCH} = amd64 ]; then
    BIN_SUFFIX=""
elif [ ${ARCH} = arm ]; then
    BIN_SUFFIX="-armhf"
fi

CMD_NAME="dist/artifacts/k3s${BIN_SUFFIX}"
SIZE=$(stat -c '%s' ${CMD_NAME})

if [ ${SIZE} -gt ${MAX_BINARY_SIZE} ]; then
    echo "k3s binary ${CMD_NAME} size ${SIZE} exceeds max acceptable size of ${MAX_BINARY_SIZE} bytes"
    exit 1
fi

echo "k3s binary ${CMD_NAME} size ${SIZE} is less than max acceptable size of ${MAX_BINARY_SIZE} bytes"
exit 0
