#!/bin/bash

if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
    DIRTY="-dirty"
fi

COMMIT=$(git rev-parse --short HEAD)
GIT_TAG=${DRONE_TAG:-$(git tag -l --contains HEAD | head -n 1)}

if [[ -z "$DIRTY" && -n "$GIT_TAG" ]]; then
    VERSION=$GIT_TAG
else
    VERSION="${COMMIT}${DIRTY}"
fi

ARCH=$(go env GOARCH)
SUFFIX="-${ARCH}"

VERSION_CONTAINERD=$(grep github.com/containerd/containerd $(dirname $0)/../go.mod | head -n1 | awk '{print $4}')
VERSION_CRICTL=$(grep github.com/kubernetes-sigs/cri-tools $(dirname $0)/../go.mod | head -n1 | awk '{print $4}')
