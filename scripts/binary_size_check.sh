#!/bin/bash

set -e

. ./scripts/version.sh

GO=${GO-go}
ARCH=${ARCH:-$("${GO}" env GOARCH)}

if [ "${DEBUG}" = 1 ]; then
    set -x
fi

# Try to keep the K3s binary under 75 megabytes.
# "64M ought to be enough for anybody"
MAX_BINARY_MB=75
MAX_BINARY_SIZE=$((MAX_BINARY_MB * 1024 * 1024))
BIN_SUFFIX="-${ARCH}"
if [ ${ARCH} = amd64 ]; then
    BIN_SUFFIX=""
elif [ ${ARCH} = arm ]; then
    BIN_SUFFIX="-armhf"
elif [ ${ARCH} = s390x ]; then
    BIN_SUFFIX="-s390x"
fi

CMD_NAME="dist/artifacts/k3s${BIN_SUFFIX}${BINARY_POSTFIX}"
SIZE=$(stat -c '%s' ${CMD_NAME})

if [ -n "${DEBUG}" ]; then
    echo "DEBUG is set, ignoring binary size"
    exit 0
fi

if [ ${SIZE} -gt ${MAX_BINARY_SIZE} ]; then
  echo "k3s binary ${CMD_NAME} size ${SIZE} exceeds max acceptable size of ${MAX_BINARY_SIZE} bytes (${MAX_BINARY_MB} MiB)"
  exit 1
fi

echo "k3s binary ${CMD_NAME} size ${SIZE} is less than max acceptable size of ${MAX_BINARY_SIZE} bytes (${MAX_BINARY_MB} MiB)"
exit 0
