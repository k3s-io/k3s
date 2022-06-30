#!/bin/bash
set -e -x

. ./scripts/version.sh

GO=${GO-go}

PKG="github.com/k3s-io/k3s"
PKG_CONTAINERD="github.com/containerd/containerd"
PKG_K3S_CONTAINERD="github.com/k3s-io/containerd"
PKG_CRICTL="github.com/kubernetes-sigs/cri-tools/pkg"
PKG_K8S_BASE="k8s.io/component-base"
PKG_K8S_CLIENT="k8s.io/client-go/pkg"
PKG_CNI_PLUGINS="github.com/containernetworking/plugins"
PKG_KUBE_ROUTER="github.com/cloudnativelabs/kube-router"

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

    -X ${PKG_CNI_PLUGINS}/pkg/utils/buildversion.BuildVersion=${VERSION_CNIPLUGINS}
    -X ${PKG_CNI_PLUGINS}/plugins/meta/flannel.Program=flannel
    -X ${PKG_CNI_PLUGINS}/plugins/meta/flannel.Version=${VERSION_FLANNEL}
    -X ${PKG_CNI_PLUGINS}/plugins/meta/flannel.Commit=${COMMIT}
    -X ${PKG_CNI_PLUGINS}/plugins/meta/flannel.buildDate=${buildDate}

    -X ${PKG_KUBE_ROUTER}/pkg/version.Version=${VERSION_KUBE_ROUTER}
    -X ${PKG_KUBE_ROUTER}/pkg/version.BuildDate=${buildDate}
"
if [ -n "${DEBUG}" ]; then
  GCFLAGS="-N -l"
else
  LDFLAGS="-w -s"
fi

STATIC="
    -extldflags '-static -lm -ldl -lz -lpthread'
"
TAGS="apparmor seccomp netcgo osusergo providerless urfave_cli_no_docs"
RUNC_TAGS="apparmor seccomp"
RUNC_STATIC="static"

if [ "$SELINUX" = "true" ]; then
    TAGS="$TAGS selinux"
    RUNC_TAGS="$RUNC_TAGS selinux"
fi

if [ "$STATIC_BUILD" != "true" ]; then
    STATIC="
"
    RUNC_STATIC=""
else
    TAGS="static_build libsqlite3 $TAGS"
fi

mkdir -p bin

if [ ${ARCH} = armv7l ] || [ ${ARCH} = arm ]; then
    export GOARCH="arm"
    export GOARM="7"
fi

if [ ${ARCH} = s390x ]; then
    export GOARCH="s390x"
fi
