//go:build linux
// +build linux

package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/sirupsen/logrus"
)

func applyContainerdStateAndAddress(nodeConfig *config.Node) {
	nodeConfig.Containerd.State = "/run/k3s/containerd"
	nodeConfig.Containerd.Address = filepath.Join(nodeConfig.Containerd.State, "containerd.sock")
}

func applyCRIDockerdAddress(nodeConfig *config.Node) {
	nodeConfig.CRIDockerd.Address = "unix:///run/k3s/cri-dockerd/cri-dockerd.sock"
}

func applyContainerdQoSClassConfigFileIfPresent(envInfo *cmds.Agent, nodeConfig *config.Node) {
	blockioPath := filepath.Join(envInfo.DataDir, "agent", "etc", "containerd", "blockio_config.yaml")

	// Set containerd config if file exists
	if _, err := os.Stat(blockioPath); !errors.Is(err, os.ErrNotExist) {
		logrus.Infof("BlockIO configuration file found")
		nodeConfig.Containerd.BlockIOConfig = blockioPath
	}

	rdtPath := filepath.Join(envInfo.DataDir, "agent", "etc", "containerd", "rdt_config.yaml")

	// Set containerd config if file exists
	if _, err := os.Stat(rdtPath); !errors.Is(err, os.ErrNotExist) {
		logrus.Infof("RDT configuration file found")
		nodeConfig.Containerd.RDTConfig = rdtPath
	}
}
