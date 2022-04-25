#!/bin/bash

set -e

if [ "${DEBUG}" = 1 ]; then
    set -x
fi

. ./scripts/version.sh

# Try to keep the K3s binary under 64 megabytes.
# "64M ought to be enough for anybody"
MAX_BINARY_MB=64
MAX_BINARY_SIZE=$((MAX_BINARY_MB * 1024 * 1024))
BIN_SUFFIX="-${ARCH}"
if [ ${ARCH} = amd64 ]; then
    BIN_SUFFIX=""
elif [ ${ARCH} = arm ]; then
    BIN_SUFFIX="-armhf"
elif [ ${ARCH} = s390x ]; then
    BIN_SUFFIX="-s390x"
fi

CMD_NAME="dist/artifacts/k3s${BIN_SUFFIX}"
SIZE=$(stat -c '%s' ${CMD_NAME})

if [ ${SIZE} -gt ${MAX_BINARY_SIZE} ]; then
  echo "k3s binary ${CMD_NAME} size ${SIZE} exceeds max acceptable size of ${MAX_BINARY_SIZE} bytes (${MAX_BINARY_MB} MiB)"
  exit 1
fi

echo "k3s binary ${CMD_NAME} size ${SIZE} is less than max acceptable size of ${MAX_BINARY_SIZE} bytes (${MAX_BINARY_MB} MiB)"
exit 0
