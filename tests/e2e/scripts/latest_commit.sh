#!/bin/bash
# Grabs the last 5 commit SHA's from the given branch, then purges any commits that do not have a passing CI build
iterations=0
curl -s -H 'Accept: application/vnd.github.v3+json' "https://api.github.com/repos/k3s-io/k3s/commits?per_page=5&sha=$1" | jq -r '.[] | .sha'  &> $2
# The VMs take time on startup to hit googleapis.com, wait loop until we can
while ! curl -s --fail https://storage.googleapis.com/k3s-ci-builds > /dev/null; do
    ((iterations++))
    if [ "$iterations" -ge 30 ]; then
        echo "Unable to hit googleapis.com/k3s-ci-builds"
        exit 1
    fi
    sleep 1
done

iterations=0
curl -s --fail https://storage.googleapis.com/k3s-ci-builds/k3s-$(head -n 1 $2).sha256sum
while [ $? -ne 0 ]; do
    ((iterations++))
    if [ "$iterations" -ge 6 ]; then
        echo "No valid commits found"
        exit 1
    fi
    sed -i 1d "$2"
    sleep 1
    curl -s --fail https://storage.googleapis.com/k3s-ci-builds/k3s-$(head -n 1 $2).sha256sum
done