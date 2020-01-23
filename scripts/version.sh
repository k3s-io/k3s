#!/bin/bash

TREE_STATE=clean
if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
    DIRTY="-dirty"
    TREE_STATE=dirty
fi

COMMIT=$(git log -n3 --pretty=format:"%H %ae" | grep -v ' drone@localhost$' | cut -f1 -d\  | head -1)
if [ -z "${COMMIT}" ]; then
  COMMIT=$(git rev-parse HEAD)
fi

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

VERSION_K8S=$(grep k8s.io/kubernetes go.mod | head -n1 | awk '{print $4}' | sed -e 's/[-+].*//')
if [ -z "$VERSION_K8S" ]; then
    VERSION_K8S="v0.0.0"
fi

VERSION_CNIPLUGINS="v0.7.6-k3s1"

if [[ -n "$GIT_TAG" ]]; then
    if [[ ! "$GIT_TAG" =~ ^"$VERSION_K8S"[+-] ]]; then
        echo "Tagged version '$GIT_TAG' does not match expected version '$VERSION_K8S[+-]*'" >&2
        exit 1
    fi
    VERSION=$GIT_TAG
else
    VERSION="$VERSION_K8S+${COMMIT:0:8}$DIRTY"
fi
VERSION_TAG="$(sed -e 's/+/-/g' <<< "$VERSION")"
