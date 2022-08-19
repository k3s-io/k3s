#!/bin/bash

set -e

if [ "${DEBUG}" = 1 ]; then
    set -x
fi

. ./scripts/version.sh

# Try to keep the K3s binary under 68 megabytes.
# "68M ought to be enough for anybody"
MAX_BINARY_MB=68
MAX_BINARY_SIZE=$((MAX_BINARY_MB * 1024 * 1024))
BIN_SUFFIX="-${ARCH}"
if [ ${ARCH} = amd64 ]; then
    BIN_SUFFIX=""
elif [ ${ARCH} = arm ]; then
    BIN_SUFFIX="-armhf"
elif [ ${ARCH} = s390x ]; then
    BIN_SUFFIX="-s390x"
fi

CMD_NAME="dist/artifacts/${PROG}${BIN_SUFFIX}"
SIZE=$(stat -c '%s' ${CMD_NAME})

if [ ${SIZE} -gt ${MAX_BINARY_SIZE} ]; then
  echo "${PROG} binary ${CMD_NAME} size ${SIZE} exceeds max acceptable size of ${MAX_BINARY_SIZE} bytes (${MAX_BINARY_MB} MiB)"
  exit 1
fi

echo "${PROG} binary ${CMD_NAME} size ${SIZE} is less than max acceptable size of ${MAX_BINARY_SIZE} bytes (${MAX_BINARY_MB} MiB)"
exit 0
