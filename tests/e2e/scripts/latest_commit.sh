#!/bin/bash

branch=$1
output_file=$2
# Grabs the last 10 commit SHA's from the given branch, then purges any commits that do not have a passing CI build


# Copied and modified from install.sh
get_commit_artifact_url() {
    commit_id=$1
    github_api_url=https://api.github.com/repos/k3s-io/k3s

    if [ -z "${GITHUB_TOKEN}" ]; then
        fatal "Installing commit builds requires GITHUB_TOKEN with k3s-io/k3s repo permissions"
    fi

    # GET request to the GitHub API to retrieve the Build workflows associated with the commit that have succeeded
    run_id=$(curl -s -H "Authorization: Bearer ${GITHUB_TOKEN}" "${github_api_url}/commits/${commit_id}/check-runs?check_name=build%20%2F%20Build&conclusion=success" | jq -r '[.check_runs | sort_by(.id) | .[].details_url | split("/")[7]] | last')
    # Extract the artifact ID for the "k3s-amd64" artifact
    GITHUB_ART_URL=$(curl -s -H "Authorization: Bearer ${GITHUB_TOKEN}" "${github_api_url}/actions/runs/${run_id}/artifacts" | jq -r ".artifacts[] | select(.name == \"k3s-amd64\") | .archive_download_url")
}

if [ -n "$GITHUB_TOKEN" ]; then
    response=$(curl -s -H "Authorization: token $GITHUB_TOKEN" -H 'Accept: application/vnd.github.v3+json' "https://api.github.com/repos/k3s-io/k3s/commits?per_page=10&sha=$branch")
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
    get_commit_artifact_url "$commit"
    if [ -n "$GITHUB_ART_URL" ]; then
        echo "$commit" > "$output_file"
        echo "Found valid commit: $commit"
        exit 0
    fi
done

echo "Failed to find a valid commit, checked: " "${commits[@]}"
exit 1