# K3S Kubernetes Patch Release Process
This document details the K3S kubernetes patch release process.

# Before You Begin
You’ll be primarily using git and go. Git can be installed via the local package manager. Make sure Go is installed and configured correctly, utilizing a “gopath”. This can be set via an environment variable called GOPATH. eg. export GOPATH=”${HOME}/go”, typically.

Ensure proper GPG Signing, disable GPG signing in git config with:
```sh
git config --local commit.gpgSign false
```

Configure GPG signing for tags as needed.
    
In Docker, prevent user mismatch with:
```sh
git config --global core.safelyUseIncompatibleGitCredentialHelper true
```

## Clone and Setup Remotes
Clone from upstream then add k3s-io fork and your personal fork.
```sh
# initial clone
git clone --origin upstream \
  https://github.com/kubernetes/kubernetes.git \
  ${GOPATH}/src/github.com/kubernetes/kubernetes
 
cd ${GOPATH}/src/github.com/kubernetes/kubernetes
 
# add the k3s-io remote
git remote add k3s-io https://github.com/k3s-io/kubernetes.git
 
# fetch all remote branches and tags. If you receive a message saying that
# previous tags will be "clobbered", add --force to the command below.
git fetch --all --tags
```
# Rebasing and Generating Tags
Establish a local branch for rebasing:

```sh
export GLOBAL_GIT_CONFIG_PATH=$(git config --list --show-origin --show-scope --global | awk 'NR==1{ split($2,path,":"); print path[2] }')
export SSH_MOUNT_PATH=$(echo ${SSH_AUTH_SOCK} || echo "${HOME}/.ssh/id_rsa")

# Set up your new/old versions of Kubernetes
export OLD_K8S=<old-k8s-version>
export NEW_K8S=<new-k8s-version>
export OLD_K8S_CLIENT=<old-k8s-client-version>
export NEW_K8S_CLIENT=<new-k8s-client-version>
export OLD_K3S_VER="${OLD_K8S}-k3s1" 
export NEW_K3S_VER="${NEW_K8S}-k3s1"
export RELEASE_BRANCH=<k8s-release-branch>
export GOPATH=$(go env GOPATH)
 
# clean old builds
rm -rf _output
 
# Rebase k3s customizations from the previous release, minus the merge commit (~1),
# onto the new upstream tag. This will leave you on a detached head that will be
# tagged in the following step.
git rebase --onto ${NEW_K8S} ${OLD_K8S} ${OLD_K3S_VER}~1
 
# Kubernetes is specific with the Go version used per release. We use alpine and docker to specify the Go version with which we build the project.
export GOVERSION=$(grep -Po '^\s*\K\S+' .go-version)

export GOIMAGE="golang:${GOVERSION}-alpine"

export BUILD_CONTAINER="FROM ${GOIMAGE}\n \
RUN apk add --no-cache \
bash \
git \
make \
tar \
gzip \
curl \
git \
coreutils \
rsync \
alpine-sdk" 

echo -e ${BUILD_CONTAINER} | docker build -t ${GOIMAGE}-dev -

# Rebasing pulls in the tags.sh script.
# Now create the tags by executing tag.sh with the given version variables.
docker run --rm -u $(id -u):$(id -g) \
  -v ${GOPATH}/src:/go/src:rw \
  -v ${GOPATH}/pkg:/go/pkg:rw \
  -v ${GOPATH}/.cache:/go/.cache:rw \
  -v ${GLOBAL_GIT_CONFIG_PATH}:/go/.gitconfig:rw \
  -e GIT_TRACE=1 \
  -e HOME=/go \
  -e GOCACHE=/go/.cache \
  -w /go/src/github.com/kubernetes/kubernetes \
  ${GOIMAGE}-dev chown -R $(id -u) .git | ./tag.sh ${NEW_K3S_VER} 2>&1 | tee ~/tags-${NEW_K3S_VER}.log
```
After tag.sh runs, you should see a list of `git push` commands at the end of the output.
Save this output to a file called ```push.sh``` and mark it as executable by running the following command:
```sh
chmod +x push.sh
```
### tag.sh example output (The kubernetes versions will correspond to those of the patch release, 1.28 is shown below):
```sh
git push ${REMOTE} staging/src/k8s.io/api/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/apiextensions-apiserver/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/apimachinery/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/apiserver/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/client-go/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/cli-runtime/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/cloud-provider/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/cluster-bootstrap/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/code-generator/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/component-base/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/component-helpers/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/controller-manager/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/cri-api/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/csi-translation-lib/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/dynamic-resource-allocation/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/endpointslice/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/kms/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/kube-aggregator/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/kube-controller-manager/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/kubectl/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/kubelet/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/kube-proxy/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/kube-scheduler/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/legacy-cloud-providers/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/metrics/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/mount-utils/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/pod-security-admission/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/sample-apiserver/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/sample-cli-plugin/v1.28.2-k3s1
git push ${REMOTE} staging/src/k8s.io/sample-controller/v1.28.2-k3s1
git push ${REMOTE} v1.28.2-k3s1
```
## Push tags to k3s-io remote
```
export REMOTE=k3s-io
./push.sh
```
## Updating k3s with the new tags
You now have a collection of tagged kubernetes modules in your worktree. By updating go.mod in k3s to point at these modules we will then be prepared to open a PR for review.
```sh
cd ${GOPATH}/src/github.com/k3s-io/k3s 
git remote add upstream https://github.com/k3s-io/k3s.git
git fetch upstream
git branch delete ${NEW_K3S_VER}
git checkout -B ${NEW_K3S_VER} upstream/${RELEASE_BRANCH}
git clean -xfd
 
sed -Ei "\|github.com/k3s-io/kubernetes| s|${OLD_K3S_VER}|${NEW_K3S_VER}|" go.mod
sed -Ei "s/k8s.io\/kubernetes v\S+/k8s.io\/kubernetes ${NEW_K8S}/" go.mod
sed -Ei "s/${OLD_K8S_CLIENT}/${NEW_K8S_CLIENT}/g" go.mod 
 
# since drone perform the builds and tests for the updated tags we no longer need to run make locally.
# We now update the go.sum by running go mod tidy:
go mod tidy
```

Ensure to update the Go version in both the Dockerfile and GitHub workflows, if required.

```sh
export OLD_GO_VERSION=<old-go-version>
sed -i'' "s/$OLD_GO_VERSION/$GOVERSION/g" Dockerfile.* .github/workflows/integration.yaml .github/workflows/unitcoverage.yaml
```

Please note that the modsync script, k3s_modsync.sh, is only necessary when making modifications to target a specific upstream Kubernetes commit instead of a tag in cases where k3s is being updated. In a regular patch release process, the modsync script is not used.

To update the dependencies that are overridden and align them with the upstream version, you can use the following command:

```sh 
curl -s https://raw.githubusercontent.com/rancher/ecm-distro-tools/master/bin/k3s_modsync.sh | sh -
```

## Validate that K3s builds
```sh
SKIP_VALIDATE=1 make
```

If you run into build issues, you may need to update other repos with forks of upstream components such as:
- https://github.com/k3s-io/klog
- https://github.com/k3s-io/cri-dockerd
- https://github.com/k3s-io/containerd

For these repos, you will need to build and push new tags and then update the go.mod with the new tags.

## Create a PR
```sh
git commit --all --signoff -m "Update to ${NEW_K8S}"
git push --set-upstream origin ${NEW_K3S_VER}
```

Create a commit with all the changes, and push this upstream.
Create a PR to merge your branch into the corresponding release branch, and wait for CI to run tests on the PR. Make sure to create the PR against the associated release branch for this update.

Once CI passes and you receive two approvals, you may now squash-merge the PR and then tag an RC after the merge to master CI run completes.

# Create a Release Candidate 
Releases are kicked off and created by tagging a new tag.
To create a new release in Github UI perform the following:

### In the case of a update to k3s, it should be incremented from `k3s1` to `k3s2`, for example, meaning the k3s version is being incremented.

1. Set title and tag according to the release version you're working on. E.g. v1.28.2-rc1+k3s1, if release only for k3s e.g v1.28.2-rc1+k3s2
2. Leave description blank.
3. Check the pre-release field.
4. Publish

The resulting run can be viewed here: 
[k3s-io/k3s Drone Dashboard](https://drone-publish.k3s.io/k3s-io/k3s)

It may be necessary to create a new release candidate to accommodate changes in dependencies such as modifications in https://github.com/k3s-io/k3s-upgrade. To do this, repeat the previous process for tagging and increment the rc version.

# Check system-agent-installer-k3s Release Images
The system-agent-installer-k3s repository is used with Rancher v2prov system. Any K3s version set in Rancher KDM must be published here as well (RCs and full releases).
To ensure this synchronization, visit the [repository](https://github.com/rancher/system-agent-installer-k3s) and verify the creation of new releases and corresponding tags that align with the version numbers.

Build progress can be tracked [here](https://hub.docker.com/r/rancher/system-agent-installer-k3s/tags). 

# Check Release Images
The k3s-upgrade repository bundles a k3s binary and script that allows a user to upgrade to a new k3s release. This process is normally automated, however this can fail. If the automation does fail, do the following:

Go to the [k3s-upgrade repository](https://github.com/k3s-io/k3s-upgrade) and manually create a new tag for the release. This will kick off a build of the image. 

1. Draft a new release
2. Enter the tag (e.g. v1.28.2-rc1+k3s1).
3. Check k3s and k3s-upgrade images Exist

This process will take some time but upon completion, the images will be listed here.

The k3s images will be published [here](https://hub.docker.com/r/rancher/k3s).
The upgrade images will be published [here](https://hub.docker.com/r/rancher/k3s-upgrade).

Verifying Component Release Versions
With each release, k3s publishes release notes that include a table of the components and their versions.

# Update Rancher KDM
This step is specific to Rancher and serves to update Rancher's [Kontainer Driver Metadata](https://github.com/rancher/kontainer-driver-metadata/).

Create a PR in the latest https://github.com/rancher/kontainer-driver-metadata/ dev branch to update the kubernetes versions in channels.yaml.

The PR should consist of two commits.

Change channels.yaml to update the kubernetes versions.
Run go generate. Commit the changes this caused to data/data.json. Title this second commit "go generate".

NOTE: If this is a ew minor release of kubernetes, then a new entry will need to be created in `channels.yaml`. Ensure to set the min/max versions accordingly. If you are not certain what they should be, reach out to the team for input on this as it will depend on what Rancher will be supporting.

NOTE: As of v1.21.4 and above, every new release minor or patch requires a new entry be created in `channels.yaml`. It is possible to build off the server, agent, and chart arguments defined in other entries.

For example, v1.21.4 has server args defined below. The versions pertaining to the release in progress will match the corresponding patch versions established at the beginning of this document:
```yaml
- version: v1.21.4+k3s2
     minChannelServerVersion: v2.6.0-alpha1
     maxChannelServerVersion: v2.6.99
     ...
     serverArgs: &serverArgs-v1
        tls-san:
           type: array
A later version can point to those arguments with no change:
```
```yaml
- version: v1.21.5+k3s1
    minChannelServerVersion: v2.6.0-alpha1
    maxChannelServerVersion: v2.6.99
    serverArgs: *serverArgs-v1
```

##### QA may request change for the specs based on rcs:
```yaml
- version: v1.28.2-rc1+k3s1
    minChannelServerVersion: v2.8.0-alpha1
    maxChannelServerVersion: v2.8.99
    serverArgs: *serverArgs-v7
```
If you are unsure of the new minor versions min/max constraints you can ask the Project manager and/or QA. 

# Create GA Release Candidate
Once QA has verified that the RC is good (or that any fixes have been added in follow up release candidates), it is time for the general release.

1. Create new release in the Github web interface
2. Set title: ${NEW_K8S}, add description with release notes. Leave the tag section blank.
3. Check the pre-release field.
4. Save as draft until RC testing is complete.

Once QA signs off on a RC:
1. Set tag to be created - this tag should match the tag in the drafted title.
2. Ensure prerelease is checked.
3. Publish.
4. Reiterate the previous checking processes and update KDM specifications accordingly with the GA release tags.
5. CI has completed, and artifacts have been created. Announce the GA and inform that k3s is thawed in the Slack release thread.
6. Create a `system-agent-installer-k3s` release with a matching tag.
##### `After 24 hours`:
1. Uncheck prerelease, and save.
2. Update channel server

The resulting CI/CD run can be viewed here: 
[k3s-io/k3s Drone Dashboard](https://drone-publish.k3s.io/k3s-io/k3s)

# Update Channel Server
Once the release is verified, the channel server config needs to be updated to reflect the new version for “stable”. [channel.yaml can be found at the root of the K3s repo.](https://github.com/k3s-io/k3s/blob/master/channel.yaml)

When updating the channel server a single-line change will need to be performed.
Release Captains responsible for this change will need to update the following stanza to reflect the new stable version of kubernetes relative to the release in progress.
```
# Example channels config
channels:
- name: stable
  latest: <new-k8s-version>+k3s1 # Replace this semver with the version corresponding to the release
```
##### After completing all these processes, ensure that you publish in the release Slack thread that patch releases have been completed and the code freeze has ended.
