# nri - Node Resource Interface

[![PkgGoDev](https://pkg.go.dev/badge/github.com/containerd/nri)](https://pkg.go.dev/github.com/containerd/nri)
[![Build Status](https://github.com/containerd/nri/workflows/CI/badge.svg)](https://github.com/containerd/nri/actions?query=workflow%3ACI)
[![codecov](https://codecov.io/gh/containerd/nri/branch/master/graph/badge.svg)](https://codecov.io/gh/containerd/nri)
[![Go Report Card](https://goreportcard.com/badge/github.com/containerd/nri)](https://goreportcard.com/report/github.com/containerd/nri)

*This project is currently in DRAFT status*

This project is a WIP for a new, CNI like, interface for managing resources on a node for Pods and Containers.

## Documentation

The basic interface, concepts and plugin design of the Container Network Interface (CNI) is an elegant way to handle multiple implementations of the network stack for containers.
This concept can be used for additional interfaces to customize a container's runtime environment.
This proposal covers a new interface for resource management on a node with a structured API and plugin design for containers.

## Lifecycle

The big selling point for CNI is that it has a structured interface for modifying the network namespace for a container.
This is different from generic hooks as they lack a type safe API injected into the lifecycle of a container.
The lifecycle point that CNI and NRI plugins will be injected into is the point between `Create` and `Start` of the container's init process.

`Create->NRI->Start`

## Configuration

Configuration is split into two parts.  One is the payload that is specific to a plugin invocation while the second is the host level configuration and options that specify what plugins to run and provide additional configuration to a plugin.

### Filepath and Binaries

Plugin binary paths can be configured via the consumer but will default to `/opt/nri/bin`.
Binaries are named with their type as the binary name, same as the CNI plugin naming scheme.

### Host Level Config

The config's default location will be `/etc/nri/resource.d/*.conf`.

```json
{
  "version": "0.1",
  "plugins": [
    {
      "type": "konfine",
      "conf": {
        "systemReserved": [0, 1]
      }
    },
    {
      "type": "clearcfs"
    }
  ]
}
```

### Input

Input to a plugin is provided via `STDIN` as a `json` payload.

```json
{
  "version": "0.1",
  "state": "create",
  "id": "redis",
  "pid": 1234,
  "spec": {
    "resources": {},
    "cgroupsPath": "default/redis",
    "namespaces": {
      "pid": "/proc/44/ns/pid",
      "mount": "/proc/44/ns/mnt",
      "net": "/proc/44/ns/net"
    },
    "annotations": {
      "qos.class": "ls"
    }
  }
}
```

### Output

```json
{
  "version": "0.1",
  "state": "create",
  "id": "redis",
  "pid": 1234,
  "cgroupsPath": "qos-ls/default/redis"
}
```

## Commands

*  Invoke - provides invocations into different lifecycle changes of a container
	- states: `setup|pause|resume|update|delete`

## Packages

A Go based API and client package will be created for both producers of plugins and consumers, commonly being the container runtime (containerd).

### Sample Plugin

**clearcfs**

Clear the cfs quotas for `ls` services.


```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/containerd/containerd/pkg/nri/skel"
	"github.com/containerd/containerd/pkg/nri/types"
	"github.com/sirupsen/logrus"
)

var max = []byte("max")

// clearCFS clears any cfs quotas for the containers
type clearCFS struct {
}

func (c *clearCFS) Type() string {
	return "clearcfs"
}

func (c *clearCFS) Invoke(ctx context.Context, r *types.Request) (*types.Result, error) {
	result := r.NewResult()
	if r.State != types.Create {
		return result, nil
	}
	switch r.Spec.Annotations["qos.class"] {
	case "ls":
		logrus.Debugf("clearing cfs for %s", r.ID)
		group, err := cg.Load(r.Spec.CgroupsPath)
		if err != nil {
			return nil, err
		}
		return result, group.Write(cg.CFSMax)
	}
	return result, nil
}

func main() {
	ctx := context.Background()
	if err := skel.Run(ctx, &clearCFS{}); err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
	}
}
```

## Project details

nri is a containerd sub-project, licensed under the [Apache 2.0 license](./LICENSE).
As a containerd sub-project, you will find the:

 * [Project governance](https://github.com/containerd/project/blob/master/GOVERNANCE.md),
 * [Maintainers](https://github.com/containerd/project/blob/master/MAINTAINERS),
 * and [Contributing guidelines](https://github.com/containerd/project/blob/master/CONTRIBUTING.md)

information in our [`containerd/project`](https://github.com/containerd/project) repository.
