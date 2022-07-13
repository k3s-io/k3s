# K3S Kubernetes Patch Release Process
This document details the K3S kubernetes patch release process.

For reference, this document makes the following assumptions:

```
Current K3S Version: v1.22.4
New K3S Version [New k8s patch]: v1.22.5

Affected repos are:
k3s-io/kubernetes
k3s-io/k3s
```

# Before You Begin
You’ll be primarily using git and go. Git can be installed via the local package manager. Make sure Go is installed and configured correctly, utilizing a “gopath”. This can be set via an environment variable called GOPATH. eg. export GOPATH=”${HOME}/go”, typically.

Clone and Setup Remotes
Clone from upstream then add k3s-io fork and your personal fork.
```sh
# initial clone
git clone --origin upstream \
  https://github.com/kubernetes/kubernetes.git \
  ${GOPATH}/src/github.com/kubernetes/kubernetes
 
# cd into the repo
cd ${GOPATH}/src/github.com/kubernetes/kubernetes
 
# optionally, set your work email for contributions
git config user.email <dev-name>@suse.com
 
# export your github name if different from user
# add the k3s-io remote
git remote add k3s-io https://github.com/k3s-io/kubernetes.git
 
# fetch all remote branches and tags. If you receive a message saying that
# previous tags will be "clobbered", add --force to the command below.
git fetch --all --tags
```

# Maintaining a Fork
Checkout the Latest, Rebase, and Create tags
Establish a local branch for rebasing:

```sh
# Set up your new/old versions of Kubernetes
export GHUSER=<Github Username>
export NEW_K8S=v1.22.5
export OLD_K8S=v1.22.4
export NEW_K8S_CLIENT=v0.22.5
export OLD_K8S_CLIENT=v0.22.4
export OLD_K3S_VER=k3s1 #check that this is correct, there may have been additional bug fix releases
export NEW_K3S_VER=k3s1
export RELEASE_BRANCH=release-1.22
export GOPATH=$(go env GOPATH)
 
# clean old builds
rm -rf _output
 
# Rebase k3s customizations from the previous release, minus the merge commit (~1),
# onto the new upstream tag. This will leave you on a detached head that will be
# tagged in the following step.
git rebase --onto $NEW_K8S $OLD_K8S $OLD_K8S-k3s1~1
  
 
# Kubernetes is very picky about go versions. We use alpine and docker to build with that go version
GOVERSION=$(yq e '.dependencies[] | select(.name == "golang: upstream version").version' build/dependencies.yaml)
GOIMAGE="golang:${GOVERSION}-alpine3.15"

BUILD_CONTAINER="FROM ${GOIMAGE}\n \
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

echo -e $BUILD_CONTAINER | docker build -t ${GOIMAGE}-dev -

GOWRAPPER="docker run --rm -u $(id -u) \
--mount type=tmpfs,destination=${GOPATH}/pkg \
--mount type=tmpfs,destination=/home/go \
-v ${GOPATH}/src:${GOPATH}/src \
-v ${GOPATH}/.cache:${GOPATH}/.cache \
-v ${HOME}/.gitconfig:/home/go/.gitconfig \
-e HOME=/home/go \
-e GOCACHE=${GOPATH}/.cache \
-w ${PWD} ${GOIMAGE}-dev"

# Rebasing pulls in the tags.sh script. Now build kubernetes and create the tags that will be pushed to your personal fork.
$GOWRAPPER ./tag.sh ${NEW_K8S}-k3s1 2>&1 | tee ~/tags-${NEW_K8S}-k3s1.log
```

After tag.sh runs, you should see list of git push commands at the end of the output.
Save this output to a file called ```push.sh``` and mark it as executable by running the following command:
```sh
chmod +x push.sh
```

### tag.sh example output:
```sh
# Just use automation on the tags.sh and tags-${NEW_K8S}-k3s1.log files produced
grep -F 'git push' ~/tags-${NEW_K8S}-k3s1.log | awk '{print "refs/tags/" $4}' | xargs -tr git push $GHUSER --force
 
OR
git push $REMOTE staging/src/k8s.io/api/v1.22.5-k3s1
git push $REMOTE staging/src/k8s.io/apiextensions-apiserver/v1.22.5-k3s1
git push $REMOTE staging/src/k8s.io/apimachinery/v1.22.5-k3s1
git push $REMOTE staging/src/k8s.io/apiserver/v1.22.5-k3s1
git push $REMOTE staging/src/k8s.io/client-go/v1.22.5-k3s1
git push $REMOTE staging/src/k8s.io/cli-runtime/v1.22.5-k3s1
git push $REMOTE staging/src/k8s.io/cloud-provider/v1.22.5-k3s1
git push $REMOTE staging/src/k8s.io/cluster-bootstrap/v1.22.5-k3s1
git push $REMOTE staging/src/k8s.io/code-generator/v1.22.5-k3s1
git push $REMOTE staging/src/k8s.io/component-base/v1.22.5-k3s1
git push $REMOTE staging/src/k8s.io/cri-api/v1.22.5-k3s1
git push $REMOTE staging/src/k8s.io/csi-translation-lib/v1.22.5-k3s1
git push $REMOTE staging/src/k8s.io/kube-aggregator/v1.22.5-k3s1
git push $REMOTE staging/src/k8s.io/kube-controller-manager/v1.22.5-k3s1
git push $REMOTE staging/src/k8s.io/kubectl/v1.22.5-k3s1
git push $REMOTE staging/src/k8s.io/kubelet/v1.22.5-k3s1
git push $REMOTE staging/src/k8s.io/kube-proxy/v1.22.5-k3s1
git push $REMOTE staging/src/k8s.io/kube-scheduler/v1.22.5-k3s1
git push $REMOTE staging/src/k8s.io/legacy-cloud-providers/v1.22.5-k3s1
git push $REMOTE staging/src/k8s.io/metrics/v1.22.5-k3s1
git push $REMOTE staging/src/k8s.io/sample-apiserver/v1.22.5-k3s1
git push $REMOTE staging/src/k8s.io/sample-cli-plugin/v1.22.5-k3s1
git push $REMOTE staging/src/k8s.io/sample-controller/v1.22.5-k3s1
git push $REMOTE v1.22.5-k3s1

#Updating and Testing

export REMOTE=$GHUSER # points to your personal fork of Kubernetes

# Manually execute the push commands one by one as seen above or run the push.sh script
git push ...
# OR
./push.sh
```
You now have a collection of tagged k8s modules in your worktree, and need to update go.mod in k3s to point at these in order to put in an initial PR for testing.
```sh
# this should be your fork here even though the pathing wouldn't indicate that
cd $GOPATH/src/github.com/rancher/k3s
git remote add upstream https://github.com/k3s-io/k3s.git
git fetch upstream
git checkout -B $NEW_K8S-$NEW_K3S_VER upstream/$RELEASE_BRANCH
git clean -xfd
 
sed -Ei "\|github.com/k3s-io/kubernetes| s|${OLD_K8S}-${OLD_K3S_VER}|${NEW_K8S}-${NEW_K3S_VER}|" go.mod
sed -Ei "s/github.com\/k3s-io\/kubernetes/github.com\/$GHUSER\/kubernetes/g" go.mod
sed -Ei "s/k8s.io\/kubernetes v\S+/k8s.io\/kubernetes $NEW_K8S/" go.mod
sed -Ei "s/$OLD_K8S_CLIENT/$NEW_K8S_CLIENT/g" go.mod # This should only change ~6 lines in go.mod
 
mkdir -p build/data && DRONE_TAG=$NEW_K8S-$NEW_K3S_VER make download && make generate

# if go generate fails and asks for a "go mod download" do it, then run make generate again.
go mod tidy
git checkout -- */zz_generated*
git add go.mod go.sum

# This may add zz_generated* files that can corrupt the build. --all may be too aggressive.
git commit --all --signoff -m "Update to $NEW_K8S"
git push --set-upstream origin $NEW_K8S-$NEW_K3S_VER # run git remote -v for your origin
```

Create a commit with all the changes, and push this to a new branch in your personal fork.
Create a PR to merge your branch into the corresponding release branch, and wait for CI to run tests on the PR. Make sure to create the PR against the associated release branch for this update.

Note: Drone should fail on "validate-go-mods" at this point... this is expected, it prevents you from merging the PR before the next step

Update the PR with the k3s-io/kubernetes tags
After drone passes and you get approval, you should now push the tags to k3s-io/kubernetes. If you went through all maintained versions, (currently v1.19, v1.20, and v1.21), you will need to manually update the push script for each tag version you’re needing to push.

```sh
# Go back to the kubernetes repo
cd ${GOPATH}/src/github.com/kubernetes/kubernetes
export REMOTE=k3s-io # this remote should be github.com/k3s-io/kubernetes
git push ...
# OR
./push.sh
# OR
grep -F 'git push' ~/tags-${NEW_K8S}-k3s1.log | awk '{print "refs/tags/" $4}' | xargs -tr git push $REMOTE --force
```
Update the pr with the real tags by changing running the commands below. If there were any dependency changes, ie higher version backport requiring manual update, run go mod vendor before proceeding.
```sh
# Go back to the k3s repo
sed -i "s/github.com\/$GHUSER\/kubernetes/github.com\/k3s-io\/kubernetes/g" go.mod
# Release-1.21 and NEWER
mkdir -p build/data && DRONE_TAG=$NEW_K8S-$NEW_K3S_VER make download && make generate
go mod tidy
git checkout -- */zz_generated*
git commit --all --signoff -m "Update tags to k3s-io for $NEW_K8S"
git push
```

Once CI passes again and you get two approvals, you can squash-merge the PR and then tag an RC.

# Create a Release Candidate 
Releases are kicked off and created by creating a new tag. To create new release in Github UI:

Set title and tag as v1.22.5-rc1+k3s1, target as release-1.22
Leave description blank.
Mark as pre-release.
Publish
Drone CI can be found here.

# Create GA Release Candidate
Once QA has verified after 24-72 hours that the RC is good (or that any fixes have been added in follow up RC candidates), it is time for the general release.

1.Create new release in GH UI
Set title: v1.22.5+k3s1, add description with release notes.
Leave tag blank!
Mark as pre-release.
Save as draft until RC testing is complete.

2.Once QA signs off on RC:
Set tag to be created - should match title.
Ensure as pre-release is still checked.
Publish.

3.Once CI has completed and artifacts are created:
Edit release to remove pre-release, and save.

# Create Release Images
The k3s-upgrade repository bundles a k3s binary + script that allows a user to upgrade to a new k3s release.


Go to the k3s-upgrade repository and manually create a new tag for the release. This will kick off a build of the image. This process is normally automated, however this can fail. If the automation does fail, do the following:

Draft a new release

Enter the tag, e.g.

v1.22.5-rc1+k3s1

This process will take some time but upon completion, the images will be listed here.

Check k3s and k3s-upgrade Images Exist
The k3s images will be published here.

The upgrade images will be published here.

Verifying Component Release Versions
With each release, k3s publishes release notes that include a table of the components k3s uses and their versions.


# Update Rancher KDM
This step is specific to Rancher and serves to update Rancher's [Kontainer Driver Metadata](https://github.com/rancher/kontainer-driver-metadata/).

Create a PR in the latest https://github.com/rancher/kontainer-driver-metadata/ dev branch to update the kubernetes versions in channels.yaml.

The PR should consist of two commits.

Change channels.yaml to update the kubernetes versions.
Run go generate. Commit the changes this caused to data/data.json. Title this second commit "go generate".

NOTE: If this is a ew minor release of kubernetes, then a new entry will need to be created in `channels.yaml`. Ensure to set the min/max versions accordingly. If you are not certain what they should be, reach out to the team for input on this as it will depend on what Rancher will be supporting.

NOTE: As of v1.21.4 and above, every new release minor or patch requires a new entry be created in `channels.yaml`. It is possible to build off the server, agent, and chart arguments defined in other entries.

For example, v1.21.4 has server args defined as follows:
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

If you are unsure of the new minor versions min/max constraints you can ask the Project manager and/or QA.

# Create system-agent-installer-k3s Release Images
The system-agent-installer-k3s repository is used with Rancher v2prov system. Any K3s version set in Rancher KDM must be published here as well (RCs and full releases).
Go to the repo and manually create a new release and tag with the version number. This will kick off a build of the image.

Build progress can be tracked here.

# Update Channel Server
Once the release is verified, the channel server config needs to be updated to reflect the new version for “stable”. channel.yaml can be found at the root of the K3s repo.
