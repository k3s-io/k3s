#!/bin/bash
set -e -x

. ./scripts/env-common.sh
. ./scripts/build-common-env-vars.sh

INSTALLBIN=$(pwd)/bin
if [ ! -x ${INSTALLBIN}/cni ]; then
(
    echo Building cni
    GOLANG_GOPATH=$(pwd)/build
    cd $CNI_PLUGINS_DIR
    GO111MODULE=off GOPATH=$GOLANG_GOPATH CGO_ENABLED=0 "${GO}" build -tags "$TAGS" -gcflags="all=${GCFLAGS}" -ldflags "$VERSIONFLAGS $LDFLAGS $STATIC" -o $INSTALLBIN/cni
)
fi
