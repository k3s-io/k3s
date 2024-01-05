# Cut Release

1. Verify that the merge CI has successfully completed before cutting the RC
1. After the merge CI has completed, cut an RC by creating a release in the GitHub interface
   1. the title is the version of k3s you are releasing with the rc1 subversion eg. "v1.25.0-rc1+k3s1"
   1. In the case of a update to k3s, it should be incremented from `k3s1` to `k3s2`, for example, meaning the k3s version is being incremented.
   1. the target should match the release branch, remember that the latest version is attached to "master"
   1. no description
   1. the tag should match the title
1. After the RC is cut validate that the CI for the RC passes
1. After the RC CI passes notify the release SLACK channel about the new RC

Example Full Command List (this is not a script!):
```
export SSH_MOUNT_PATH="/var/folders/...krzO/agent.452"
export GLOBAL_GIT_CONFIG_PATH="/Users/mtrachier/.gitconfig"
export OLD_K8S="v1.22.14"
export NEW_K8S="v1.22.15"
export OLD_K8S_CLIENT="v0.22.14"
export NEW_K8S_CLIENT="v0.22.15"
export OLD_K3S_VER="v1.22.14-k3s1" 
export NEW_K3S_VER="v1.22.15-k3s1"
export RELEASE_BRANCH="release-1.22"
export GOPATH="/Users/mtrachier/go"
export GOVERSION="1.16.15"
export GOIMAGE="golang:1.16.15-alpine3.15"
export BUILD_CONTAINER="FROM golang:1.16.15-alpine3.15\n RUN apk add --no-cache bash git make tar gzip curl git coreutils rsync alpine-sdk"

install -d /Users/mtrachier/go/src/github.com/kubernetes
rm -rf /Users/mtrachier/go/src/github.com/kubernetes/kubernetes
git clone --origin upstream https://github.com/kubernetes/kubernetes.git /Users/mtrachier/go/src/github.com/kubernetes/kubernetes
cd /Users/mtrachier/go/src/github.com/kubernetes/kubernetes
git remote add k3s-io https://github.com/k3s-io/kubernetes.git
git fetch --all --tags

# this second fetch should return no more tags pulled, this makes it easier to see pull errors
git fetch --all --tags

# rebase
rm -rf _output
git rebase --onto v1.22.15 v1.22.14 v1.22.14-k3s1~1

# validate go version
echo "GOVERSION is $(yq -e '.dependencies[] | select(.name == "golang: upstream version").version' build/dependencies.yaml)"

# generate build container
echo -e "FROM golang:1.16.15-alpine3.15\n RUN apk add --no-cache bash git make tar gzip curl git coreutils rsync alpine-sdk" | docker build -t golang:1.16.15-alpine3.15-dev -

# run tag.sh
# note user id is 502, I am not root user
docker run --rm -u 502 \
--mount type=tmpfs,destination=/Users/mtrachier/go/pkg \
-v /Users/mtrachier/go/src:/go/src \
-v /Users/mtrachier/go/.cache:/go/.cache \
-v /Users/mtrachier/.gitconfig:/go/.gitconfig \
-e HOME=/go \
-e GOCACHE=/go/.cache \
-w /go/src/github.com/kubernetes/kubernetes golang:1.16.15-alpine3.15-dev ./tag.sh v1.22.15-k3s1 2>&1 | tee ~/tags-v1.22.15-k3s1.log

# generate and run push.sh, make sure to paste in the tag.sh output below
vim push.sh
chmod +x push.sh
./push.sh

install -d /Users/mtrachier/go/src/github.com/k3s-io
rm -rf /Users/mtrachier/go/src/github.com/k3s-io/k3s
git clone --origin upstream https://github.com/k3s-io/k3s.git /Users/mtrachier/go/src/github.com/k3s-io/k3s
cd /Users/mtrachier/go/src/github.com/k3s-io/k3s

git checkout -B v1.22.15-k3s1 upstream/release-1.22
git clean -xfd


# note that sed has different parameters on MacOS than Linux
# also note that zsh is the default MacOS shell and is not bash/dash (the default Linux shells)
sed -Ei '' "\|github.com/k3s-io/kubernetes| s|v1.22.14-k3s1|v1.22.15-k3s1|" go.mod
git diff
sed -Ei '' "s/k8s.io\/kubernetes v.*$/k8s.io\/kubernetes v1.22.15/" go.mod
git diff
sed -Ei '' "s/v0.22.14/v0.22.15/g" go.mod
git diff
go mod tidy

# make sure go version is updated in all locations
vim .github/workflows/integration.yaml
vim .github/workflows/unitcoverage.yaml
vim Dockerfile.dapper
vim Dockerfile.manifest
vim Dockerfile.test

git commit --all --signoff -m "Update to v1.22.15"
git remote add origin https://github.com/matttrach/k3s-1.git
git push --set-upstream origin v1.22.15-k3s1

# use link to generate pull request, make sure your target is the proper release branch 'release-1.22'
```