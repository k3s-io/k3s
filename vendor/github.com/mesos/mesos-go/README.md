# Go bindings for Apache Mesos

Pure Go language bindings for Apache Mesos, under development.
As with other pure implementations, mesos-go uses the HTTP wire protocol to communicate directly with a running Mesos master and its slave instances.
One of the objectives of this project is to provide an idiomatic Go API that makes it super easy to create Mesos frameworks using Go. 

[![Build Status](https://travis-ci.org/mesos/mesos-go.svg)](https://travis-ci.org/mesos/mesos-go)
[![GoDoc](https://godoc.org/github.com/mesos/mesos-go?status.png)](https://godoc.org/github.com/mesos/mesos-go)
[![Coverage Status](https://coveralls.io/repos/github/mesos/mesos-go/badge.svg?branch=master)](https://coveralls.io/github/mesos/mesos-go?branch=master)

## Status
New projects should use the Mesos v1 API bindings, located in `api/v1`.
Unless otherwise indicated, the remainder of this README describes the Mesos v1 API implementation.

Please **vendor** this library to avoid unpleasant surprises via `go get ...`.

The Mesos v0 API version of the bindings, located in `api/v0`, are more mature but will not see any major development besides critical compatibility and bug fixes.

### Compatibility
`mesos-N` tags mark the start of support for a specific Mesos version while maintaining backwards compatibility with the previous major version.

### Features
- The SchedulerDriver API implemented
- The ExecutorDriver API implemented
- Example programs on how to use the API
- Modular design for easy readability/extensibility

### Pre-Requisites
- Go 1.7 or higher; https://golang.org/dl/
    - A standard and working Go workspace setup (multiple golang versions are tested via CI)
- Apache Mesos 1.0 or newer; http://mesos.apache.org/downloads/
- protoc compiler; https://github.com/google/protobuf/releases
    - v3.3.x is tested by CI and should be used for code generation
- `govendor`; https://github.com/kardianos/govendor

## Installing
Users of this library are encouraged to vendor it. API stability isn't guaranteed at this stage.
```shell
# download the source code
$ go get -d github.com/mesos/mesos-go

# build the example binaries
$ cd $GOPATH/src/github.com/mesos/mesos-go
$ make install
```

## Testing
```shell
$ make test
```

## Contributing
Contributions are welcome. Please refer to [CONTRIBUTING.md](CONTRIBUTING.md) for
guidelines.

## License
This project is [Apache License 2.0](LICENSE).
