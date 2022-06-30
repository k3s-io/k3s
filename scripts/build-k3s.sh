#!/bin/bash
set -e -x

. ./scripts/build-common-env-vars.sh

echo Building k3s
CGO_ENABLED=1 "${GO}" build -tags "$TAGS" -gcflags="all=${GCFLAGS}" -ldflags "$VERSIONFLAGS $LDFLAGS $STATIC" -o bin/k3s ./cmd/server/main.go
ln -s k3s ./bin/k3s-agent
ln -s k3s ./bin/k3s-server
ln -s k3s ./bin/k3s-etcd-snapshot
ln -s k3s ./bin/k3s-secrets-encrypt
ln -s k3s ./bin/k3s-certificate
ln -s k3s ./bin/k3s-completion
ln -s k3s ./bin/kubectl
ln -s k3s ./bin/crictl
ln -s k3s ./bin/ctr
