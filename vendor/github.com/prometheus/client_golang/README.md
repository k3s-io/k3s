# Prometheus Go client library

[![Build Status](https://travis-ci.org/prometheus/client_golang.svg?branch=master)](https://travis-ci.org/prometheus/client_golang)
[![Go Report Card](https://goreportcard.com/badge/github.com/prometheus/client_golang)](https://goreportcard.com/report/github.com/prometheus/client_golang)
[![go-doc](https://godoc.org/github.com/prometheus/client_golang?status.svg)](https://godoc.org/github.com/prometheus/client_golang)

This is the [Go](http://golang.org) client library for
[Prometheus](http://prometheus.io). It has two separate parts, one for
instrumenting application code, and one for creating clients that talk to the
Prometheus HTTP API.

__This library requires Go1.7 or later.__

## Important note about releases, versioning, tagging, and stability

While our goal is to follow [Semantic Versioning](https://semver.org/), this
repository is still pre-1.0.0. To quote the
[Semantic Versioning spec](https://semver.org/#spec-item-4): “Anything may
change at any time. The public API should not be considered stable.” We know
that this is at odds with the widespread use of this library. However, just
declaring something 1.0.0 doesn't make it 1.0.0. Instead, we are working
towards a 1.0.0 release that actually deserves its major version number.

Having said that, we aim for always keeping the tip of master in a workable
state. We occasionally tag versions and track their changes in CHANGELOG.md,
but this happens mostly to keep dependency management tools happy and to give
people a handle they can talk about easily. In particular, all commits in the
master branch have passed the same testing and reviewing. There is no QA
process in place that would render tagged commits more stable or better tested
than others.

There is a plan behind the current (pre-1.0.0) versioning, though:

- v0.9 is the “production release”, currently tracked in the master
  branch. “Patch” releases will usually be just bug fixes, indeed, but
  important new features that do not require invasive code changes might also
  be included in those. We do not plan any breaking changes from one v0.9.x
  release to any later v0.9.y release, but nothing is guaranteed. Since the
  master branch will eventually be switched over to track the upcoming v0.10
  (see below), we recommend to tell your dependency management tool of choice
  to use the latest v0.9.x release, at least for your production software. In
  that way, you should get bug fixes and non-invasive, low-risk new features
  without the need to change anything on your part.
- v0.10 is a planned release that will have a _lot_ of breaking changes
  (despite being only a “minor” release in the Semantic Versioning terminology,
  but as said, pre-1.0.0 means nothing is guaranteed). Essentially, we have
  been piling up feature requests that require breaking changes for a while,
  and they are all collected in the
  [v0.10 milestone](https://github.com/prometheus/client_golang/milestone/2).
  Since there will be so many breaking changes, the development for v0.10 is
  currently not happening in the master branch, but in the
  [dev-0.10 branch](https://github.com/prometheus/client_golang/tree/dev-0.10).
  It will violently change for a while, and it will definitely be in a
  non-working state now and then. It should only be used for sneak-peaks and
  discussions of the new features and designs.
- Once v0.10 is ready for real-life use, it will be merged into the master
  branch (which is the reason why you should lock your dependency management
  tool to v0.9.x and only migrate to v0.10 when both you and v0.10 are ready
  for it). In the ideal case, v0.10 will be the basis for the future v1.0
  release, but we cannot provide an ETA at this time.

## Instrumenting applications

[![code-coverage](http://gocover.io/_badge/github.com/prometheus/client_golang/prometheus)](http://gocover.io/github.com/prometheus/client_golang/prometheus) [![go-doc](https://godoc.org/github.com/prometheus/client_golang/prometheus?status.svg)](https://godoc.org/github.com/prometheus/client_golang/prometheus)

The
[`prometheus` directory](https://github.com/prometheus/client_golang/tree/master/prometheus)
contains the instrumentation library. See the
[guide](https://prometheus.io/docs/guides/go-application/) on the Prometheus
website to learn more about instrumenting applications.

The
[`examples` directory](https://github.com/prometheus/client_golang/tree/master/examples)
contains simple examples of instrumented code.

## Client for the Prometheus HTTP API

[![code-coverage](http://gocover.io/_badge/github.com/prometheus/client_golang/api/prometheus/v1)](http://gocover.io/github.com/prometheus/client_golang/api/prometheus/v1) [![go-doc](https://godoc.org/github.com/prometheus/client_golang/api/prometheus?status.svg)](https://godoc.org/github.com/prometheus/client_golang/api)

The
[`api/prometheus` directory](https://github.com/prometheus/client_golang/tree/master/api/prometheus)
contains the client for the
[Prometheus HTTP API](http://prometheus.io/docs/querying/api/). It allows you
to write Go applications that query time series data from a Prometheus
server. It is still in alpha stage.

## Where is `model`, `extraction`, and `text`?

The `model` packages has been moved to
[`prometheus/common/model`](https://github.com/prometheus/common/tree/master/model).

The `extraction` and `text` packages are now contained in
[`prometheus/common/expfmt`](https://github.com/prometheus/common/tree/master/expfmt).

## Contributing and community

See the [contributing guidelines](CONTRIBUTING.md) and the
[Community section](http://prometheus.io/community/) of the homepage.
