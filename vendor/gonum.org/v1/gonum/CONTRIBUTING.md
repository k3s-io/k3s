# [<img src="https://avatars1.githubusercontent.com/u/3771091?v=3&s=30">](https://github.com/gonum) Contribution Guidelines
#### Introduction

The Gonum project provides general purpose numerical computing and data science libraries for the Go programming language, and we would like you to join us in improving Gonum's quality and scope.
This document is for anyone who is contributing or interested in contributing.
Questions about Gonum or the use of its libraries can be directed to [gonum-dev](https://groups.google.com/forum/#!forum/gonum-dev).

#### Table of Contents

[Project Scope](#project-scope)

[The Gonum Repositories](#the-gonum-repositories)

[Contributing](#Contributing)
  * [Working Together](#working-together)
  * [Reporting Bugs](#reporting-bugs)
  * [Suggesting Enhancements](#suggesting-enhancements)
  * [Your First Code Contribution](#your-first-code-contribution)
  * [Code Contribution](#code-contribution)
  * [Code Review](#code-review)
  * [What Can I Do to Help?](#what-can-i-do-to-help)
  * [Style](#style)

## Project Scope

The purpose of the Gonum project is to provide general purpose numerical computing and data science libraries for the Go programming language.
The libraries should aim to provide building blocks for disciplinary work and advanced algorithms.
Code should be implemented in pure Go.
Calls to C, Fortran, or other languages may be justified with performance considerations, but should be opt-in for users.
Calls to assembly should be opt-out, if included.
Code should favor readability and explicitness over cleverness.
This makes code easy to review and verify, not only at submission, but also for users who want to understand how the algorithms work.
Where possible, the source of algorithms should be referenced in the comments.

## The Gonum Repositories

Here are the current repositories for the gonum project.
If code you want to contribute doesn't quite fit in any of them, then please start a discussion on the [mailing list](https://groups.google.com/forum/#!forum/gonum-dev).
Code can be found at [github.com/gonum/](https://github.com/gonum/)\<repo\>, and documentation at godoc.org/github.com/gonum/\<repo\>.

* [gonum](https://github.com/gonum/gonum) — The gonum repository contains the majority of Gonum packages
* [plot](https://github.com/gonum/plot) — A repository for plotting and visualizing data
* [netlib](https://github.com/gonum/netlib) — The netlib repository contains Cgo wrappers for C implementations of BLAS and LAPACKE
* [hdf5](https://github.com/gonum/hdf5) — The hdf5 repository contains a Cgo wrapper for the HDF5 data storage and access library
* [exp](https://github.com/gonum/exp) — The exp repository contains experimental and incomplete package implementations
* [tools](https://github.com/gonum/tools) — Tools for working with Go code
* [talks](https://github.com/gonum/talks) — Talks given about Go and Gonum delivered by Gonum developers
* [gonum.github.io](https://github.com/gonum/gonum.github.io) — Gonum website

## Contributing

### Working Together

When contributing or otherwise participating please follow the [Gonum community's conduct expectations](CONDUCT.md).

### Reporting Bugs

When you encounter a bug, please open an issue on the corresponding repository.
Start the issue title with the repository/sub-repository name, like `stat/distmv: issue name`.
Be specific about the environment you encountered the bug in.
If you are able to write a test that reproduces the bug, please include it in the issue.
As a rule we keep all tests OK.

### Suggesting Enhancements

If the scope of the enhancement is small, open an issue.
If it is large, such as suggesting a new repository, sub-repository, or interface refactoring, then please start a discussion on [the gonum-dev list](https://groups.google.com/forum/#!forum/gonum-dev).

### Your First Code Contribution

If you are a new contributor, thank you!  Before your first merge, you will need to be added to the [CONTRIBUTORS](https://github.com/gonum/license/blob/master/CONTRIBUTORS) and [AUTHORS](https://github.com/gonum/license/blob/master/AUTHORS) file.
Open a pull request adding yourself to them.
All Gonum code follows the BSD license in the [license document](https://github.com/gonum/license/blob/master/LICENSE).
We prefer that code contributions do not come with additional licensing.
For exceptions, added code must also follow a BSD license.

### Code Contribution

If it is possible to split a large pull request into two or more smaller pull requests, please try to do so.
Pull requests should include tests for any new code before merging.
It is ok to start a pull request on partially implemented code to get feedback, and see if your approach to a problem is sound.
You don't need to have tests, or even have code that compiles to open a pull request, although both will be needed before merge.
When tests use magic numbers, please include a comment explaining the source of the number.
Benchmarks are optional for new features, but if you are submitting a pull request justified by performance improvement, you will need benchmarks to measure the impact of your change, and the pull request should include a report from [benchcmp](https://godoc.org/golang.org/x/tools/cmd/benchcmp) or, preferably, [benchstat](https://github.com/rsc/benchstat).

### Code Review

If you are a contributor, please be welcoming to new contributors.  [Here](http://sarah.thesharps.us/2014/09/01/the-gentle-art-of-patch-review/) is a good guide.

There are several terms code reviews may use that you should become familiar with.

  * ` LGTM ` — looks good to me
  * ` SGTM ` — sounds good to me
  * ` s/foo/bar/ ` — please replace ` foo ` with ` bar `; this is [sed syntax](http://en.wikipedia.org/wiki/Sed#Usage)
  * ` s/foo/bar/g ` — please replace ` foo ` with ` bar ` throughout your entire change

We follow the convention of requiring at least 1 reviewer to say LGTM before a merge.
When code is tricky or controversial, submitters and reviewers can request additional review from others and more LGTMs before merge.
You can ask for more review by saying PTAL in a comment in a pull request.
You can follow a PTAL with one or more @someone to get the attention of particular people.
If you don't know who to ask, and aren't getting enough review after saying PTAL, then PTAL @gonum/developers will get more attention.
Also note that you do not have to be the pull request submitter to request additional review.

### What Can I Do to Help?

If you are looking for some way to help the Gonum project, there are good places to start, depending on what you are comfortable with.
You can [search](https://github.com/issues?utf8=%E2%9C%93&q=is%3Aopen+is%3Aissue+user%3Agonum) for open issues in need of resolution.
You can improve documentation, or improve examples.
You can add and improve tests.
You can improve performance, either by improving accuracy, speed, or both.
You can suggest and implement new features that you think belong in Gonum.

### Style

We use [Go style](https://github.com/golang/go/wiki/CodeReviewComments).
