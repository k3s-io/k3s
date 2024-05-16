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

func applyContainerdQoSClassConfigFileIfPresent(envInfo *cmds.Agent, containerdConfig *config.Containerd) {
	containerdConfigDir := filepath.Join(envInfo.DataDir, "agent", "etc", "containerd")

	blockioPath := filepath.Join(containerdConfigDir, "blockio_config.yaml")

	// Set containerd config if file exists
	if fileInfo, err := os.Stat(blockioPath); !errors.Is(err, os.ErrNotExist) {
		if fileInfo.Mode().IsRegular() {
			logrus.Infof("BlockIO configuration file found")
			containerdConfig.BlockIOConfig = blockioPath
		}
	}

	rdtPath := filepath.Join(containerdConfigDir, "rdt_config.yaml")

	// Set containerd config if file exists
	if fileInfo, err := os.Stat(rdtPath); !errors.Is(err, os.ErrNotExist) {
		if fileInfo.Mode().IsRegular() {
			logrus.Infof("RDT configuration file found")
			containerdConfig.RDTConfig = rdtPath
		}
	}
}

// configureACL will configure an Access Control List for the specified file.
// On Linux, this function is a no-op
func configureACL(file string) error {
	return nil
}
