#!/bin/bash
set -e -x

. ./scripts/build-common-env-vars.sh

export GOPATH=$(pwd)/build

echo Building containerd
pushd ./build/src/github.com/containerd/containerd
TAGS="${TAGS/netcgo/netgo}"
CGO_ENABLED=1 "${GO}" build -tags "$TAGS" -gcflags="all=${GCFLAGS}" -ldflags "$VERSIONFLAGS $LDFLAGS $STATIC" -o bin/containerd              ./cmd/containerd
CGO_ENABLED=1 "${GO}" build -tags "$TAGS" -gcflags="all=${GCFLAGS}" -ldflags "$VERSIONFLAGS $LDFLAGS $STATIC" -o bin/containerd-shim-runc-v2 ./cmd/containerd-shim-runc-v2
popd
cp -vf ./build/src/github.com/containerd/containerd/bin/* ./bin/
