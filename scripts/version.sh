#!/bin/bash

GO=${GO-go}
. ./scripts/platform.sh

if [ -z "$NO_DAPPER" ]; then
    . ./scripts/git_version.sh
fi

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

VERSION_ROOT="v0.15.2"
case ${ARCH} in
    amd64)
      K3S_ROOT_SHA256=9e56393cf828583b50b6b0e66cc47cb6a5e1d0489eab1436421bc20c56c0cf65
    ;;
    arm)
      K3S_ROOT_SHA256=af8614e5b9e2f87d30bd4387c512703c6bf2bc53a3764e5181ef2f2eaccab8d2
    ;;
    arm64)
      K3S_ROOT_SHA256=7a754f4aeb1771b2b147ac8ff48fbc0a152f4ab1c6b4f16f94b1121e5eaaba50
    ;;
    riscv64)
      K3S_ROOT_SHA256=3b76a4a5bfc5c8623702a3b99e3015cd36b0336dd73c7ba4a765d018dc5a9685
    ;;
    *)
      echo "[ERROR] unsupported architecture: ${ARCH}"
      exit 1
    ;;
esac

VERSION_HELM_JOB="v0.11.1-build20260615"

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
