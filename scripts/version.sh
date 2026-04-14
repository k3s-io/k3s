#!/bin/bash

GO=${GO-go}
ARCH=${ARCH:-$("${GO}" env GOARCH)}
OS=${OS:-$("${GO}" env GOOS)}
SUFFIX="-${ARCH}"

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

VERSION_ROOT="v0.15.0"
case ${ARCH} in
    amd64)
      K3S_ROOT_SHA256="20066815d9941185fce3934cc3bae2fa3e2dbb46ca7e63462efb2ea59f1b15c4"
    ;;
    arm)
      K3S_ROOT_SHA256="2e43dac7750da52a756a9d4e8598d6e89937d565582660b26ca124bd9c8dbfaa"
    ;;
    arm64)
      K3S_ROOT_SHA256="4bdfc715dc8b5e2c4956f8686b895a56386f4cc6468215dcd22130a680650577"
    ;;
    riscv64)
      K3S_ROOT_SHA256="0e79998acaa156059ba1ff676a4e5f290069e25f799de43ee016a0e780f9d4d3"
    ;;
    *)
      echo "[ERROR] unsupported architecture: ${ARCH}"
      exit 1
    ;;
esac

VERSION_HELM_JOB="v0.9.16-build20260410"

GO_VERSION_URL="https://raw.githubusercontent.com/kubernetes/kubernetes/${VERSION_K8S}/.go-version"
VERSION_GOLANG="go"$(curl -sL "${GO_VERSION_URL}" | tr -d '[:space:]')

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

BINARY_POSTFIX=
if [ ${OS} = windows ]; then
    BINARY_POSTFIX=.exe
fi
