#!/bin/bash
set -e -x

cd $(dirname $0)/..

. ./scripts/version.sh

GO=${GO-go}

PKG="github.com/k3s-io/k3s"
PKG_CONTAINERD="github.com/containerd/containerd"
PKG_K3S_CONTAINERD="github.com/k3s-io/containerd"
PKG_CRICTL="sigs.k8s.io/cri-tools/pkg"
PKG_K8S_BASE="k8s.io/component-base"
PKG_K8S_CLIENT="k8s.io/client-go/pkg"

buildDate=$(date -u '+%Y-%m-%dT%H:%M:%SZ')

VERSIONFLAGS="
    -X ${PKG}/pkg/version.Version=${VERSION}
    -X ${PKG}/pkg/version.GitCommit=${COMMIT:0:8}

    -X ${PKG_K8S_CLIENT}/version.gitVersion=${VERSION}
    -X ${PKG_K8S_CLIENT}/version.gitCommit=${COMMIT}
    -X ${PKG_K8S_CLIENT}/version.gitTreeState=${TREE_STATE}
    -X ${PKG_K8S_CLIENT}/version.buildDate=${buildDate}

    -X ${PKG_K8S_BASE}/version.gitVersion=${VERSION}
    -X ${PKG_K8S_BASE}/version.gitCommit=${COMMIT}
    -X ${PKG_K8S_BASE}/version.gitTreeState=${TREE_STATE}
    -X ${PKG_K8S_BASE}/version.buildDate=${buildDate}

    -X ${PKG_CRICTL}/version.Version=${VERSION_CRICTL}

    -X ${PKG_CONTAINERD}/version.Version=${VERSION_CONTAINERD}
    -X ${PKG_CONTAINERD}/version.Package=${PKG_K3S_CONTAINERD}

    -X ${PKG_CONTAINERD}/version.Version=${VERSION_CONTAINERD}
    -X ${PKG_CONTAINERD}/version.Package=${PKG_K3S_CONTAINERD}
"

LDFLAGS="
    -w -s"

STATIC=""

TAGS="netcgo osusergo providerless"

mkdir -p bin

# Sanity check for downstream dependencies
echo 'Validate K3s cross-compilation on Windows x86_64' 
GOOS=windows CGO_ENABLED=1 CXX=x86_64-w64-mingw32-g++ CC=x86_64-w64-mingw32-gcc \
    "${GO}" build -tags "${TAGS}" -ldflags "${VERSIONFLAGS} ${LDFLAGS} ${STATIC}" -o bin/k3s.exe ./cmd/server/main.go

if [ "${KEEP_WINDOWS_BIN}" != 'true' ]; then
    rm -rf bin/k3s.exe
fi
