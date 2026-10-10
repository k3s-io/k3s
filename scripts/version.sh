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

VERSION_ROOT="v0.15.3"
case ${ARCH} in
    amd64)
      K3S_ROOT_SHA256=0c5e030e0d3ea5a8292660e37daa1b1e66ce0b0fa5c0482e053abe45f04957d9
    ;;
    arm)
      K3S_ROOT_SHA256=2b995f60e8248a9e814a3a7bf071f75bbc9b431f463c7abdc2ddc09a33464649
    ;;
    arm64)
      K3S_ROOT_SHA256=efa73b8716c12f496ad7b1729987b22b6da80d87914ac404d0e59d2d85f76ccd
    ;;
    riscv64)
      K3S_ROOT_SHA256=3aa8afbba73f8e6d935901926def2c1516dc9620640e9007008dbb2897574a58
    ;;
    *)
      echo "[ERROR] unsupported architecture: ${ARCH}"
      exit 1
    ;;
esac

VERSION_HELM_JOB="v0.13.3-build20260727"

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
