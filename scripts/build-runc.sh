#!/bin/bash
set -e -x

. ./scripts/build-common-env-vars.sh

export GOPATH=$(pwd)/build

echo Building runc
pushd ./build/src/github.com/opencontainers/runc
rm -f runc
make EXTRA_FLAGS="-gcflags=\"all=${GCFLAGS}\"" EXTRA_LDFLAGS="$LDFLAGS" BUILDTAGS="$RUNC_TAGS" $RUNC_STATIC
popd
cp -vf ./build/src/github.com/opencontainers/runc/runc ./bin/
