package main

import (
	"os"
	"path/filepath"

	hostlocal "github.com/containernetworking/plugins/plugins/ipam/host-local"
	"github.com/containernetworking/plugins/plugins/main/bridge"
	"github.com/containernetworking/plugins/plugins/main/loopback"
	"github.com/containernetworking/plugins/plugins/meta/flannel"
	"github.com/containernetworking/plugins/plugins/meta/portmap"
	"github.com/docker/docker/pkg/reexec"
)

func main() {
	os.Args[0] = filepath.Base(os.Args[0])
	reexec.Register("host-local", hostlocal.Main)
	reexec.Register("bridge", bridge.Main)
	reexec.Register("flannel", flannel.Main)
	reexec.Register("loopback", loopback.Main)
	reexec.Register("portmap", portmap.Main)
	reexec.Init()
}
