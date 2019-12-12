#!/bin/bash

if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
    DIRTY="-dirty"
fi

COMMIT=$(git rev-parse --short HEAD)
GIT_TAG=${DRONE_TAG:-$(git tag -l --contains HEAD | head -n 1)}

ARCH=$(go env GOARCH)
SUFFIX="-${ARCH}"

VERSION_CONTAINERD=$(grep github.com/containerd/containerd go.mod | head -n1 | awk '{print $4}')
if [ -z "$VERSION_CONTAINERD" ]; then
    VERSION_CONTAINERD="v0.0.0"
fi

VERSION_CRICTL=$(grep github.com/kubernetes-sigs/cri-tools go.mod | head -n1 | awk '{print $4}')
if [ -z "$VERSION_CRICTL" ]; then
    VERSION_CRICTL="v0.0.0"
fi

VERSION_K8S=$(grep k8s.io/kubernetes go.mod | head -n1 | awk '{print $4}')
if [ -z "$VERSION_K8S" ]; then
    VERSION_K8S="v0.0.0"
fi

VERSION_CNIPLUGINS="v0.7.6-k3s1"

if [[ -n "$GIT_TAG" ]]; then
    VERSION=$GIT_TAG
else
    VERSION="$(sed -e 's/[-+].*//' <<< "$VERSION_K8S")+$COMMIT"
    VERSION_TAG="$(sed -e 's/+/-/g' <<< "$VERSION")"
fi
