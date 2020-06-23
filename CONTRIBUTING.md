# Contributing to k3s #

## Getting Help ##

If you have a question about k3s or have encountered problems using it,
start by [asking in slack](https://slack.rancher.io/).

## Submitting a Pull Request ##

1. Submit an [issue][issue] describing your proposed change.
2. We will try to respond to your issue promptly.
3. Fork this repo, develop and test your code changes. See the project's
   [README](README.md) for further information about working in this repository.
4. Submit a pull request against this repo's `master` branch.
    - Include instructions on how to test your changes.
5. Your branch may be merged once all configured checks pass, including:
    - The branch has passed tests in CI.
    - Two reviews from k3s maintainers

## Committing ##

We prefer squash or rebase commits so that all changes from a branch are
committed to master as a single commit. All pull requests are squashed when
merged, but rebasing prior to merge gives you better control over the commit
message.

### Commit messages ###

Finalized commit messages should be in the following format:

```txt
Subject

Problem

Solution

Validation

Fixes #[GitHub issue ID]
```

#### Subject ####

- one line, <= 50 characters
- describe what is done; not the result
- use the active voice
- capitalize first word and proper nouns
- do not end in a period — this is a title/subject
- reference the GitHub issue by number

#### Problem ####

Explain the context and why you're making that change.  What is the problem
you're trying to solve? In some cases there is not a problem and this can be
thought of as being the motivation for your change.

#### Solution ####

Describe the modifications you've made.

If this PR changes a behavior, it is helpful to describe the difference between
the old behavior and the new behavior. Provide example CLI output, or changed
YAML where applicable.

Describe any implementation changes which are particularly complex or
unintuitive.

List any follow-up work that will need to be done in a future PR and link to any
relevant Github issues.

#### Validation ####

Describe the testing you've done to validate your change.  Give instructions for
reviewers to replicate your tests.  Performance-related changes should include
before- and after- benchmark results.

[issue]: https://github.com/rancher/k3s/issues/new
[slack]: http://slack.rancher.io/

_This contributing doc adapated from Linkerd2's contributing doc._
