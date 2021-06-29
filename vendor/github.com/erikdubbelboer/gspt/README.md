gspt
====

`setproctitle()` package for Go.

[![Build Status](https://travis-ci.org/erikdubbelboer/gspt.png?branch=master)](https://travis-ci.org/erikdubbelboer/gspt)

--------------------------------

Installation
------------

Simple install the package to your [$GOPATH](https://github.com/golang/go/wiki/GOPATH) with the [go tool](http://golang.org/cmd/go/):
```bash
go get github.com/erikdubbelboer/gspt
```
Make sure [Git is installed](http://git-scm.com/downloads) on your machine and in your system's `PATH`.

Usage
-----

```go
import "github.com/erikdubbelboer/gspt"

gspt.SetProcTitle("some title")
```

Please check the [documentation](http://godoc.org/github.com/erikdubbelboer/gspt) for more details.
