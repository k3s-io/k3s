genetlink [![builds.sr.ht status](https://builds.sr.ht/~mdlayher/genetlink.svg)](https://builds.sr.ht/~mdlayher/genetlink?) [![GoDoc](https://godoc.org/github.com/mdlayher/genetlink?status.svg)](https://godoc.org/github.com/mdlayher/genetlink) [![Go Report Card](https://goreportcard.com/badge/github.com/mdlayher/genetlink)](https://goreportcard.com/report/github.com/mdlayher/genetlink)
=========

Package `genetlink` implements generic netlink interactions and data types.
MIT Licensed.

For more information about how netlink and generic netlink work,
check out my blog series on [Linux, Netlink, and Go](https://mdlayher.com/blog/linux-netlink-and-go-part-1-netlink/).

If you have any questions or you'd like some guidance, please join us on
[Gophers Slack](https://invite.slack.golangbridge.org) in the `#networking`
channel!

## Stability

See the [CHANGELOG](./CHANGELOG.md) file for a description of changes between
releases.

This package has reached v1.0.0 and any future breaking API changes will prompt
the release of a new major version. Features and bug fixes will continue to
occur in the v1.x.x series.

The general policy of this package is to only support the latest, stable version
of Go. Compatibility shims may be added for prior versions of Go on an as-needed
basis. If you would like to raise a concern, please [file an issue](https://github.com/mdlayher/genetlink/issues/new).

**If you depend on this package in your applications, please use Go modules.**
