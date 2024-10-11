#!/bin/bash

branch=$1
output_file=$2
# Grabs the last 10 commit SHA's from the given branch, then purges any commits that do not have a passing CI build
iterations=0
commits_str=$(curl -s -H 'Accept: application/vnd.github.v3+json' "https://api.github.com/repos/k3s-io/k3s/commits?per_page=10&sha=$branch" | jq -j -r '.[] | .sha, " "')
read -a commits <<< "$commits_str"

# The VMs take time on startup to hit aws, wait loop until we can
while ! curl -s --fail https://k3s-ci-builds.s3.amazonaws.com > /dev/null; do
    ((iterations++))
    if [ "$iterations" -ge 30 ]; then
        echo "Unable to hit https://k3s-ci-builds.s3.amazonaws.com"
        exit 1
    fi
    sleep 1
done

for commit in "${commits[@]}"; do
    if curl -s --fail https://k3s-ci-builds.s3.amazonaws.com/k3s-$commit.sha256sum > /dev/null; then
        echo "$commit" > "$output_file"
        exit 0
    fi
done

echo "Failed to find a valid commit, checked: " "${commits[@]}"
exit 1