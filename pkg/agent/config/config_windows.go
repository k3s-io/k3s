// +build windows

package config

import (
	"path/filepath"

	"github.com/rancher/k3s/pkg/daemons/config"
)

func applyContainerdStateAndAddress(nodeConfig *config.Node) {
	nodeConfig.Containerd.State = filepath.Join(nodeConfig.Containerd.Root, "state")
	nodeConfig.Containerd.Address = "npipe:////./pipe/containerd-containerd"
}
