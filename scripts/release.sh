#!/bin/bash
set -e

cd $(dirname $0)/..

if [ -z "$K3S_ARM64_HOST" ]; then
    echo K3S_ARM_HOST must be set
    exit 1
fi

if [ -z "$K3S_ARM64_HOST_USER" ]; then
    echo K3S_ARM_HOST_USER must be set
    exit 1
fi

if [ -z "$K3S_ARM_HOST" ]; then
    K3S_ARM_HOST=${K3S_ARM64_HOST}
fi

if [ -z "$K3S_ARM_HOST_USER" ]; then
    K3S_ARM_HOST_USER=${K3S_ARM64_HOST_USER}
fi


rm -rf dist
mkdir -p build
make ci > build/build-amd64.log 2>&1 &
AMD_PID=$!

DAPPER_HOST_ARCH=arm DOCKER_HOST="ssh://${K3S_ARM_HOST_USER}@${K3S_ARM_HOST}" make release-arm
DAPPER_HOST_ARCH=arm64 DOCKER_HOST="ssh://${K3S_ARM64_HOST_USER}@${K3S_ARM64_HOST}" make release-arm

echo Waiting for amd64 build to finish
wait -n $AMD_PID || {
    cat build/build-amd64.log
    exit 1
}
ls -la dist
echo Done
