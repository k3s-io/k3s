// +build vendor

package main

import (
	_ "github.com/containerd/containerd/cmd/containerd-shim"
	_ "github.com/coreos/go-systemd/activation"
	_ "github.com/go-bindata/go-bindata"
	_ "github.com/go-bindata/go-bindata/go-bindata"
	_ "github.com/opencontainers/runc"
	_ "github.com/opencontainers/runc/contrib/cmd/recvtty"
	_ "github.com/opencontainers/runc/libcontainer/nsenter"
	_ "github.com/opencontainers/runc/libcontainer/specconv"
	_ "k8s.io/kubernetes/cmd/hyperkube"
)

func main() {}
