#!/bin/bash
set -eu

BASE_VERSION="${1}"
REPO="https://github.com/k3s-io/kubernetes.git"

resolved=$(go list -m all 2>/dev/null | awk '{print $1}')
pairs=$(grep -oE 'k8s\.io/[A-Za-z0-9_-]+ => github\.com/k3s-io/kubernetes/staging/src/k8s\.io/[A-Za-z0-9_-]+' go.mod)
all_tags=$(git ls-remote --tags "$REPO")

missing=0
while IFS= read -r pair; do
  module_name=$(echo "$pair" | awk '{print $1}')
  fork_path=$(echo "$pair" | awk '{print $3}' | sed 's#github.com/k3s-io/kubernetes/##')

  echo "$resolved" | grep -qx "$module_name" || continue   # replace-only, skip

  if ! echo "$all_tags" | grep -qE "refs/tags/${fork_path}/${BASE_VERSION}-k3s[0-9]+\$"; then
    echo "missing any tag for ${module_name} at ${BASE_VERSION}" >&2
    missing=1
  fi
done <<< "$pairs"

[ "$missing" -eq 0 ] || { echo "not all required modules have a tag for ${BASE_VERSION}" >&2; exit 1; }
echo "all required modules have at least one -k3sN tag for ${BASE_VERSION}"
