# Contributing to k3s #

Thanks for taking the time to contribute to k3s!

Contributing is not limited to writing code and submitting a PR. Feel free to submit an [issue](https://github.com/k3s-io/k3s/issues/new/choose) or comment on an existing one to report a bug, provide feedback, or suggest a new feature. You can also join the discussion on [slack](https://slack.rancher.io/).

Of course, contributing code is more than welcome! To keep things simple, if you're fixing a small issue, you can simply submit a PR and we will pick it up. However, if you're planning to submit a bigger PR to implement a new feature or fix a relatively complex bug, please open an issue that explains the change and the motivation for it. If you're addressing a bug, please explain how to reproduce it.

If you're interested in contributing documentation, please note the following:
- Doc issues are raised in this repository, and they are tracked under the `kind/documentation` label.
- Pull requests are submitted to the K3s documentation source in the [Rancher docs repository.](https://github.com/rancher/docs/) The K3s docs content is in the `content/k3s/` directory.

## Pull request and git commit guidance

### Opening PRs and organizing commits
PRs should generally address only 1 issue at a time. If you need to fix two bugs, open two separate PRs. This will keep the scope of your pull requests smaller and allow them to be reviewed and merged more quickly.

When possible, fill out as much detail in the pull request template as is reasonable. Most important is to reference the GitHub issue that you are addressing with the PR.

**NOTE:** GitHub has [a feature](https://docs.github.com/en/github/managing-your-work-on-github/linking-a-pull-request-to-an-issue#linking-a-pull-request-to-an-issue-using-a-keyword) that will automatically close issues referenced with a keyword (such as "Fixes") by a PR or commit once the PR/commit is merged. Don't use these keywords. We don't want issues to be automatically closed. We want our testers to independently verify and close them.

Generally, pull requests should consist of a single logical commit. However, if your PR is for a large feature, you may need a more logical breakdown of commits. This is fine as long as each commit is a single logical unit.

The other exception to this single-commit rule is if your PR includes a change to a vendored dependency or generated code. To make reviewing easier, these changes should be segregated into their own commit. Note that as we migrate from using the vendor directory to a pure go module model for our projects, this will be less of an issue.

### Writing good commit messages
Git commit messages should explain the how and why of your change and be separated into a brief subject line followed by a more detailed body. When in doubt, follow this guide for good commit messages and you can’t go wrong: https://chris.beams.io/posts/git-commit/.

One particularly useful point made in the above guide is regarding commit subject lines:

> A properly formed Git commit subject line should always be able to complete the following sentence:
> 
> - If applied, this commit will <ins>your subject line here</ins>

A simple but effective convention to follow for commits is the “problem / solution” pattern. It looks like this:
```
<Subject>

Problem: <Statement of problem>

Solution: <Statement of solution>
```

As an example, here is a commit taken from the rancher/rancher repo:
```
commit b71ce2892eecb7c87a5212e3486f1de899a694aa
Author: Dan Ramich <danold215@gmail.com>
Date:   Tue Jun 19 11:56:52 2018 -0700

    Add Validator for RoleTemplate

    Problem:
    Builtin RoleTemplates can be updated through the API

    Solution:
    Add a Validator to ensure the only field that can be changed on a
    builtin RoleTemplate is 'locked'
```

### Reviewing, addressing feedback, and merging
Generally, pull requests need two approvals from maintainers to be merged. One exception to this is when a PR is simply a "pull through" that is just updating a dependency from other Rancher-managed vendor packages or any minor third-party vendor update. In this case, only one approval is needed.

When addressing review feedback, it is helpful to the reviewer if additional changes are made in new commits. This allows the reviewer to easily see the delta between what they previously reviewed and the changes you added to address their feedback.

Once a PR has the necessary approvals, it can be merged. Here’s how the merge should be handled:
- If the PR is a single logical commit, the merger should use the “Rebase and merge” option. This keeps the git commit history very clean and simple and eliminates noise from "merge commits."
- If the PR is more than one logical commit, the merger should use the “Create a merge commit” option.
- If the PR consists of more than one commit because the author added commits to address feedback, the commits should be squashed into a single commit (or more than one logical commit, if it is a big feature that needs more commits). This can be achieved in one of two ways:
  - The merger can use the “Squash and merge” option. If they do this, the merger is responsible for cleaning up the commit message according to the previously stated commit message guidance.
  - The pull request author, after getting the requisite approvals, can reorganize the commits as they see fit (using, for example, git rebase -i) and re-push.

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
