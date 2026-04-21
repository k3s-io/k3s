# Contributing to K3s #

Thanks for taking the time to contribute to K3s!

Please review and follow the [Code of Conduct](CODE_OF_CONDUCT.md).

Contributing is not limited to writing code and submitting a PR. Feel free to submit an [issue](https://github.com/k3s-io/k3s/issues/new/choose) or comment on an existing one to report a bug, provide feedback, or suggest a new feature. You can also join the discussion on [slack](https://rancher-users.slack.com/channels/k3s).

Of course, contributing code is more than welcome! To keep things simple, if you're fixing a small issue, you can simply submit a PR and we will pick it up. However, if you're planning to submit a bigger PR to implement a new feature or fix a relatively complex bug, please open an issue that explains the change and the motivation for it. If you're addressing a bug, please explain how to reproduce it.

If you're interested in contributing documentation, please note the following:
- Doc issues are raised in this repository, and they are tracked under the `kind/documentation` label.
- Pull requests are submitted to the K3s documentation source in the [k3s-io docs repository.](https://github.com/k3s-io/docs).

If you're interested in contributing new tests, please see the [TESTING.md](./tests/TESTING.md).

## Code Convention

See the [code conventions documentation](./docs/contrib/code_conventions.md) for more information on how to write code for K3s.

### Opening PRs and organizing commits
PRs should generally address only 1 issue at a time. If you need to fix two bugs, open two separate PRs. This will keep the scope of your pull requests smaller and allow them to be reviewed and merged more quickly.

When possible, fill out as much detail in the pull request template as is reasonable. Most important is to reference the GitHub issue that you are addressing with the PR.

**NOTE:** GitHub has [a feature](https://docs.github.com/en/github/managing-your-work-on-github/linking-a-pull-request-to-an-issue#linking-a-pull-request-to-an-issue-using-a-keyword) that will automatically close issues referenced with a keyword (such as "Fixes") by a PR or commit once the PR/commit is merged. Don't use these keywords. We don't want issues to be automatically closed. We want our testers to independently verify and close them.

Generally, pull requests should consist of a single logical commit. However, if your PR is for a large feature, you may need a more logical breakdown of commits. This is fine as long as each commit is a single logical unit.

The other exception to this single-commit rule is if your PR includes a change to a vendored dependency or generated code. To make reviewing easier, these changes should be segregated into their own commit. Note that as we migrate from using the vendor directory to a pure go module model for our projects, this will be less of an issue.

As the issue and the PR already include all the required information, commit messages are normally empty. The title of the commit should summarize in a few words what the commit is trying to do.

For each commit, please ensure you sign off as mentioned below in the [Developer Certificate Of Origin section](#developer-certificate-of-origin).

### Reviewing, addressing feedback, and merging
Generally, pull requests need two approvals from maintainers to be merged. One exception to this is when a PR is simply a "pull through" that is just updating a dependency from other Rancher-managed vendor packages or any minor third-party vendor update. In this case, only one approval is needed.

When addressing review feedback, it is helpful to the reviewer if additional changes are made in new commits. This allows the reviewer to easily see the delta between what they previously reviewed and the changes you added to address their feedback.

Once a PR has the necessary approvals, it can be merged. Here’s how the merge should be handled:
- If the PR is a single logical commit, the merger should use the “Rebase and merge” option. This keeps the git commit history very clean and simple and eliminates noise from "merge commits."
- If the PR is more than one logical commit, the merger should use the “Create a merge commit” option.
- If the PR consists of more than one commit because the author added commits to address feedback, the commits should be squashed into a single commit (or more than one logical commit, if it is a big feature that needs more commits). This can be achieved in one of two ways:
  - The merger can use the “Squash and merge” option. If they do this, the merger is responsible for cleaning up the commit message according to the previously stated commit message guidance.
  - The pull request author, after getting the requisite approvals, can reorganize the commits as they see fit (using, for example, git rebase -i) and re-push.

## Development Workflow
To get started with the code, you'll want to follow the standard GitHub fork-and-pull-request workflow.

1. Fork the repository
Start by forking the K3s repository to your own GitHub account. Once forked, clone it locally:

```bash
git clone https://github.com/<your-username>/k3s.git
cd k3s
```
It is also a good idea to add the upstream repository as a remote so you can stay up to date:
```bash
git remote add upstream https://github.com/k3s-io/k3s.git
```

2. Create a branch
Always create a new branch for your work. This keeps your master branch clean and makes it much easier to manage multiple contributions. Pick a descriptive name that relates to the issue you are solving:

```bash
git checkout -b feat/my-new-feature
# OR
git checkout -b fix/issue-number-description
```

3. Make your changes and commit
As you work, remember the golden rule mentioned later: one logical unit per commit. When you are ready to commit, don't forget the -s flag to sign off on the DCO as explained in the ([Developer Certificate Of Origin](#developer-certificate-of-origin)) section:

```bash
git add .
git commit -s -m "Your descriptive title here"
```

4. Push and open a PR
Once your changes are ready and you've verified them (and perhaps run make format), push your branch to your fork:

```bash
git push origin feat/my-new-feature
```

After pushing, head over to the K3s Pull Requests page. GitHub should proactively show a prompt to "Compare & pull request" for your recently pushed branch. Follow the PR template to provide the necessary details.

If you would like to test your changes locally, you should first build. Check our [BUILDING.md](BUILDING.md) file for instructions about how to build K3s. Once built, you can find the K3s binary in dist/artifacts.

To leverage the built binary to set up a cluster, copy the K3s binary to "/usr/local/bin/k3s" and use the K3s script command with INSTALL_K3S_SKIP_DOWNLOAD:
```
curl -sfL https://get.k3s.io | INSTALL_K3S_SKIP_DOWNLOAD=true sh -
```


## Developer Certificate Of Origin ##

To contribute to this project, you must agree to the Developer Certificate of Origin (DCO) for each commit you make. The DCO is a simple statement that you, as a contributor, have the legal right to make the contribution.

See the [DCO](DCO) file for the full text of what you must agree to.

To signify that you agree to the DCO for a commit, you add a line to the git
commit message:

```txt
Signed-off-by: Jane Smith <jane.smith@example.com>
```

In most cases, you can add this signoff to your commit automatically with the
`-s` flag to `git commit`. Please use your real name and a reachable email address.

## Golangci-lint ##

There is a CI check for formatting on our code, you'll need to install `goimports` to be able to attend this check, you can do it by running the command:

```
go install golang.org/x/tools/cmd/goimports@latest
```

then run:

```
make format
```
