# Git workflows

This document is an overview of K3s git workflow. It includes conventions, tips, and how to maintain good repository hygiene.

- [Branching model](#branching-model)
  - [Branch naming conventions](#branch-naming-conventions)
  - [Backport policy](#backport-policy)
- [Git operations](#git-operations)
  - [Setting up](#setting-up)
  - [Branching out](#branching-out)
  - [Keeping local branches in sync](#keeping-local-branches-in-sync)
  - [Pushing changes](#pushing-changes)

## Branching model

K3s project uses the [GitHub flow](https://docs.github.com/en/get-started/quickstart/github-flow) as its branching model, where most of the changes come from repositories forks instead of branches within the same one.

### Branch naming conventions

Every forked repository works independently, meaning that any contributor can create branches with the name they see fit. However, it is worth noting that K3s mirrors [Kubernetes version skew policy](https://kubernetes.io/releases/version-skew-policy/) by maintaining release branches for the most recent three minor releases. The only exception is that the main branch mirrors the latest Kubernetes release (1.23) instead of using a `release-` prefixed one.

```text
master       -------------------------------------------. (Kubernetes 1.23)
release-1.21            \---------------|---------------. (Kubernetes 1.21)
release-1.22                            \---------------. (Kubernetes 1.22)
```

Upon Kubernetes release, each branch will be tagged accordingly to the version they mirror, and the artifacts will be built from there.

### Backport policy

All new work happens on the main branch, which means that for most cases, one should branch out from there and create the pull request against it. If the change involves adding a feature or patching K3s, the maintainers will backport it into the supported release branches.

## Git operations

There are everyday tasks related to git that every contributor needs to perform, and this section elaborates on them.

### Setting up

Creating a K3s fork, cloning it, and setting its upstream remote can be summarized on:

1. Visit <https://github.com/k3s-io/k3s>
2. Click the `Fork` button (top right) to establish a cloud-based fork
3. Clone fork to local storage
4. Add to your fork K3s remote as upstream

Once cloned, in code it would look this way:

```sh
## Clone fork to local storage
export user="your github profile name"
git clone https://github.com/$user/k3s.git
# or: git clone git@github.com:$user/k3s.git

## Add k3s as upstream to your fork
cd k3s 
git remote add upstream https://github.com/k3s-io/k3s.git
# or: git remote add upstream git@github.com:k3s-io/k3s.git

## Ensure to never push to upstream directly
git remote set-url --push upstream no_push

## Confirm that your remotes make sense:
git remote -v
```

### Branching out

Every time one wants to work on a new K3s feature, we do:

1. Get local main branch up to date
2. Create a new branch from the main one (i.e.: myfeature branch )

In code it would look this way:

```sh
## Get local master up to date
# Assuming the k3s clone is the current working directory
git fetch upstream
git checkout master
git rebase upstream/master

## Create a new branch from master
git checkout -b myfeature
```

### Keeping local branches in sync

Either when branching out from master or a release one, keep in mind it is worth checking if any change has been pushed upstream by doing:

```sh
git fetch upstream
git rebase upstream/master
```

It is suggested to `fetch` then `rebase` instead of `pull` since the latter does a merge, which leaves merge commits. For this, one can consider changing the local repository configuration by doing `git config branch.autoSetupRebase always` to change the behavior of `git pull`, or another non-merge option such as `git pull --rebase`.

### Pushing changes

For commit messages and signatures please refer to the [CONTRIBUTING.md](../../CONTRIBUTING.md) document.

Nobody should push directly to upstream, even if one has such contributor access; instead, prefer [Github's pull request](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/about-pull-requests) mechanism to contribute back into K3s. For expectations and guidelines about pull requests, consult the [CONTRIBUTING.md](../../CONTRIBUTING.md) document.
