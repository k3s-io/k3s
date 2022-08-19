#!/bin/bash
set -e -x

. ./scripts/version.sh

cd $(dirname $0)

${PROG} crictl images -o json \
    | jq -r '.images[].repoTags[0] | select(. != null)' \
    | tee image-list.txt
