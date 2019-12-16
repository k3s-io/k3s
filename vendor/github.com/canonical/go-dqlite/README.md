go-dqlite [![Build Status](https://travis-ci.org/canonical/go-dqlite.png)](https://travis-ci.org/canonical/go-dqlite) [![Coverage Status](https://coveralls.io/repos/github/canonical/go-dqlite/badge.svg?branch=master)](https://coveralls.io/github/canonical/go-dqlite?branch=master) [![Go Report Card](https://goreportcard.com/badge/github.com/canonical/go-dqlite)](https://goreportcard.com/report/github.com/canonical/go-dqlite) [![GoDoc](https://godoc.org/github.com/canonical/go-dqlite?status.svg)](https://godoc.org/github.com/canonical/go-dqlite)
======

This repository provides the `go-dqlite` Go package, containing bindings for the
[dqlite](https://github.com/canonical/dqlite) C library and a pure-Go
client for the dqlite wire [protocol](https://github.com/canonical/dqlite/blob/master/doc/protocol.md).

Usage
-----

The best way to understand how to use the ```go-dqlite``` package is probably by
looking at the source code of the [demo
program](https://github.com/canonical/go-dqlite/tree/master/cmd/dqlite-demo) and
use it as example.

Build
-----

In order to use the go-dqlite package in your application, you'll need to have
the [dqlite](https://github.com/canonical/dqlite) C library installed on your
system, along with its dependencies. You then need to pass the ```-tags```
argument to the Go tools when building or testing your packages, for example:

```bash
go build -tags libsqlite3
go test -tags libsqlite3
```

Documentation
-------------

The documentation for this package can be found on [Godoc](http://godoc.org/github.com/canonical/go-dqlite).

Demo
----

To see dqlite in action, either install the Debian package from the PPA:

```bash
sudo add-apt-repository -y ppa:dqlite/stable
sudo apt install dqlite libdqlite-dev
```

or build the dqlite C library and its dependencies from source, as described
[here](https://github.com/canonical/dqlite#build), and then run:

```
go install -tags libsqlite3 ./cmd/dqlite-demo
```

from the top-level directory of this repository.

Once the ```dqlite-demo``` binary is installed, start three nodes of the demo
application, respectively with IDs ```1```, ```2,``` and ```3```:

```bash
dqlite-demo start 1 &
dqlite-demo start 2 &
dqlite-demo start 3 &
```

The node with ID ```1``` automatically becomes the leader of a single node
cluster, while the nodes with IDs ```2``` and ```3``` are waiting to be notified
what cluster they belong to. Let's make nodes ```2``` and ```3``` join the
cluster:

```bash
dqlite-demo add 2
dqlite-demo add 3
```

Now we can start using the cluster. The demo application is just a simple
key/value store that stores data in a SQLite table. Let's insert a key pair:

```bash
dqlite-demo update my-key my-value
```

and then retrive it from the database:

```bash
dqlite-demo query my-key
```

Currently node ```1``` is the leader. If we stop it and then try to query the
key again we'll notice that the ```query``` command hangs for a bit waiting for
the failover to occur and for another node to step up as leader:

```
kill -TERM %1; sleep 0.1; dqlite-demo query my-key; dqlite-demo cluster
```
