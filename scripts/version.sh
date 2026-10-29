#!/bin/bash

GO=${GO-go}
. ./scripts/platform.sh

. ./scripts/git_version.sh

get-module-version(){
  go list -mod=readonly -m -f '{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}' $1
}

get-module-path(){
  go list -mod=readonly -m -f '{{if .Replace}}{{.Replace.Path}}{{else}}{{.Path}}{{end}}' $1
}

RUNC_DIR=build/src/github.com/opencontainers/runc
CONTAINERD_DIR=build/src/github.com/containerd/containerd
HCSSHIM_DIR=build/src/github.com/microsoft/hcsshim
PLUGINS_DIR=build/src/github.com/containernetworking/plugins

PKG_CONTAINERD_K3S=$(get-module-path github.com/containerd/containerd/v2)
VERSION_CONTAINERD=$(get-module-version github.com/containerd/containerd/v2)
if [ -z "$VERSION_CONTAINERD" ]; then
    VERSION_CONTAINERD="v0.0.0"
fi

VERSION_CRICTL=$(get-module-version sigs.k8s.io/cri-tools)
if [ -z "$VERSION_CRICTL" ]; then
    VERSION_CRICTL="v0.0.0"
fi

PKG_KUBERNETES_K3S=$(get-module-path k8s.io/kubernetes)
VERSION_K8S_K3S=$(get-module-version k8s.io/kubernetes)
VERSION_K8S=${VERSION_K8S_K3S%-k3s*}
if [ -z "$VERSION_K8S" ]; then
    VERSION_K8S="v0.0.0"
fi

VERSION_RUNC=$(get-module-version github.com/opencontainers/runc)
if [ -z "$VERSION_RUNC" ]; then
    VERSION_RUNC="v0.0.0"
fi

VERSION_HCSSHIM=$(get-module-version github.com/Microsoft/hcsshim)
if [ -z "$VERSION_HCSSHIM" ]; then
    VERSION_HCSSHIM="v0.0.0"
fi

VERSION_FLANNEL=$(get-module-version github.com/flannel-io/flannel)
if [ -z "$VERSION_FLANNEL" ]; then
  VERSION_FLANNEL="v0.0.0"
fi

VERSION_CRI_DOCKERD=$(get-module-version github.com/Mirantis/cri-dockerd)
if [ -z "$VERSION_CRI_DOCKERD" ]; then
  VERSION_CRI_DOCKERD="v0.0.0"
fi

VERSION_CNIPLUGINS="v1.9.1-k3s1"
VERSION_FLANNEL_PLUGIN="v1.9.0-flannel1"

VERSION_KUBE_ROUTER=$(get-module-version github.com/cloudnativelabs/kube-router/v2)
if [ -z "$VERSION_KUBE_ROUTER" ]; then
    VERSION_KUBE_ROUTER="v0.0.0"
fi

VERSION_ROOT="v0.15.4"
case ${ARCH} in
    amd64)
      K3S_ROOT_SHA256=658581f9dba924b24dcca14fcbb7237d69c9d3cd56995558d56efcd522263ea9
    ;;
    arm)
      K3S_ROOT_SHA256=2a09ec8b4ab7a7fd41f9caa0b04a15cc631c2682f27f6f3605a77684717f3ac4
    ;;
    arm64)
      K3S_ROOT_SHA256=80b65fd619c8534a5cfe09310217a588e7cbb43f2f95b5f3dedcf2b39495dafc
    ;;
    riscv64)
      K3S_ROOT_SHA256=fd5e0b38c0e13fba14ff45df2c0cb89fc8ae8b1c6f5a7480fd24f1d2b644b29e
    ;;
    *)
      echo "[ERROR] unsupported architecture: ${ARCH}"
      exit 1
    ;;
esac

VERSION_HELM_JOB="v0.13.3-build20261021"

if [[ -n "$GIT_TAG" ]]; then
    if [[ ! "$GIT_TAG" =~ ^"$VERSION_K8S"[+-] ]]; then
        echo "Tagged version '$GIT_TAG' does not match expected version '$VERSION_K8S[+-]*'" >&2
        exit 1
    fi
    VERSION=$GIT_TAG
else
    VERSION="$VERSION_K8S+k3s-${COMMIT:0:8}$DIRTY"
fi
VERSION_TAG="$(sed -e 's/+/-/g' <<< "$VERSION")"
