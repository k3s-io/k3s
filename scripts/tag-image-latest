#!/bin/bash
set -e

cd $(dirname $0)/..

. ./scripts/version.sh

TAG=${TAG:-${VERSION_TAG}${SUFFIX}}
REPO=${REPO:-rancher}
IMAGE_NAME=${IMAGE_NAME:-k3s}

IMAGE=${REPO}/${IMAGE_NAME}:${TAG}
LATEST=${REPO}/${IMAGE_NAME}:latest
docker image tag ${IMAGE} ${LATEST}
echo Tagged ${IMAGE} as ${LATEST}
