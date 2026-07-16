#!/bin/bash
set -eu

BASE_VERSION="${1}"
REPO="https://github.com/k3s-io/kubernetes.git"
all_tags=$(git ls-remote --tags "$REPO")

changed=0

latest_suffix_for() {
  echo "$all_tags" | grep -oE "refs/tags/${1}${BASE_VERSION}-k3s[0-9]+\$" \
    | sed -E "s#.*-k3s([0-9]+)\$#\1#" | sort -n | tail -1
}

# Add || true so grep doesn't fail the script if go.mod is empty
pairs=$(grep -oE 'k8s\.io/[A-Za-z0-9_-]+ => github\.com/k3s-io/kubernetes/staging/src/k8s\.io/[A-Za-z0-9_-]+' go.mod || true)

if [ -z "$pairs" ]; then
  echo "No staging dependencies found in go.mod"
  exit 0
fi

while IFS= read -r pair; do
  module_name=$(echo "$pair" | awk '{print $1}')
  fork_path=$(echo "$pair" | awk '{print $3}' | sed 's#github.com/k3s-io/kubernetes/##')

  n=$(latest_suffix_for "${fork_path}/")
  if [ -z "$n" ]; then
    echo "No tag for ${module_name} at ${BASE_VERSION}, leaving untouched" >&2
    continue
  fi

  # More robust regex for spaces/tabs, and execute the replacement
  sed -i -E "s#(${module_name}[[:space:]]+=>[[:space:]]+github\.com/k3s-io/kubernetes/staging/src/[^[:space:]]+)[[:space:]]+v[0-9]+\.[0-9]+\.[0-9]+-k3s[0-9]+#\1 ${BASE_VERSION}-k3s${n}#" go.mod
  
  # CRITICAL: We must echo output so Updatecli knows a change occurred!
  echo "Bumped staging module ${module_name} to ${BASE_VERSION}-k3s${n}"
  changed=1
done <<< "$pairs"

# If nothing changed, ensure Updatecli sees no stdout so it doesn't create an empty commit
if [ "$changed" -eq 0 ]; then
  exit 0
fi
