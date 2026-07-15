#!/bin/bash
set -eu

VERSION="${1}"
REPO="https://github.com/k3s-io/kubernetes.git"

modules=$(grep -oE 'github\.com/k3s-io/kubernetes/staging/src/[^ ]+' go.mod \
  | sed 's#github.com/k3s-io/kubernetes/##' | sort -u)

all_tags=$(git ls-remote --tags "$REPO")

missing=0
for module in $modules; do
  if ! echo "$all_tags" | grep -q "refs/tags/${module}/${VERSION}\$"; then
    echo "missing tag: refs/tags/${module}/${VERSION}" >&2
    missing=1
  fi
done

if [ "$missing" -eq 1 ]; then
  echo "not all staging module tags are pushed for ${VERSION}" >&2
  exit 1
fi

echo "all staging module tags present for ${VERSION}"
exit 0
