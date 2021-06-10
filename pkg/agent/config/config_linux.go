// +build linux

package config

import (
	"path/filepath"

	"github.com/rancher/k3s/pkg/daemons/config"
)

func applyContainerdStateAndAddress(nodeConfig *config.Node) {
	nodeConfig.Containerd.State = "/run/k3s/containerd"
	nodeConfig.Containerd.Address = filepath.Join(nodeConfig.Containerd.State, "containerd.sock")
}
