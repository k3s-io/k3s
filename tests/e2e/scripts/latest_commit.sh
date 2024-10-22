#!/bin/bash

branch=$1
output_file=$2
# Grabs the last 10 commit SHA's from the given branch, then purges any commits that do not have a passing CI build
iterations=0

# The VMs take time on startup to hit aws, wait loop until we can
while ! curl -s --fail https://k3s-ci-builds.s3.amazonaws.com > /dev/null; do
    ((iterations++))
    if [ "$iterations" -ge 30 ]; then
        echo "Unable to hit https://k3s-ci-builds.s3.amazonaws.com"
        exit 1
    fi
    sleep 1
done

if [ -n "$GH_TOKEN" ]; then
    response=$(curl -s -H "Authorization: token $GH_TOKEN" -H 'Accept: application/vnd.github.v3+json' "https://api.github.com/repos/k3s-io/k3s/commits?per_page=10&sha=$branch")
else
    response=$(curl -s -H 'Accept: application/vnd.github.v3+json' "https://api.github.com/repos/k3s-io/k3s/commits?per_page=10&sha=$branch")
fi
type=$(echo "$response" | jq -r type)

# Verify if the response is an array with the k3s commits
if [[ $type == "object" ]]; then
    message=$(echo "$response" | jq -r .message)
    if [[ $message == "API rate limit exceeded for "* ]]; then
        echo "Github API rate limit exceeded"
	exit 1
    fi
    echo "Github API returned a non-expected response ${message}"
    exit 1
elif [[ $type == "array" ]]; then
    commits_str=$(echo "$response" | jq -j -r '.[] | .sha, " "')
fi

read -a commits <<< "$commits_str"

for commit in "${commits[@]}"; do
    if curl -s --fail https://k3s-ci-builds.s3.amazonaws.com/k3s-$commit.sha256sum > /dev/null; then
        echo "$commit" > "$output_file"
        exit 0
    fi
done

echo "Failed to find a valid commit, checked: " "${commits[@]}"
exit 1