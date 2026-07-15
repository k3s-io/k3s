#!/bin/bash
set -eu

BASE_VERSION="${1}"
REPO="https://github.com/k3s-io/kubernetes.git"
all_tags=$(git ls-remote --tags "$REPO")

latest_suffix_for() {
  echo "$all_tags" | grep -oE "refs/tags/${1}${BASE_VERSION}-k3s[0-9]+\$" \
    | sed -E "s#.*-k3s([0-9]+)\$#\1#" | sort -n | tail -1
}

pairs=$(grep -oE 'k8s\.io/[A-Za-z0-9_-]+ => github\.com/k3s-io/kubernetes/staging/src/k8s\.io/[A-Za-z0-9_-]+' go.mod)

while IFS= read -r pair; do
  module_name=$(echo "$pair" | awk '{print $1}')
  fork_path=$(echo "$pair" | awk '{print $3}' | sed 's#github.com/k3s-io/kubernetes/##')

  n=$(latest_suffix_for "${fork_path}/")
  [ -n "$n" ] || { echo "no tag for ${module_name}, leaving untouched" >&2; continue; }

  sed -i -E "s#(${module_name} => github\.com/k3s-io/kubernetes/staging/src/[^ ]+) v[0-9]+\.[0-9]+\.[0-9]+-k3s[0-9]+#\1 ${BASE_VERSION}-k3s${n}#" go.mod
done <<< "$pairs"
