//go:build linux
// +build linux

package config

import (
	"os"
	"path/filepath"

	"github.com/k3s-io/k3s/pkg/agent/containerd"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// applyContainerdOSSpecificConfig sets linux-specific containerd config
func applyContainerdOSSpecificConfig(nodeConfig *config.Node) error {
	nodeConfig.Containerd.State = "/run/k3s/containerd"
	nodeConfig.Containerd.Address = filepath.Join(nodeConfig.Containerd.State, "containerd.sock")

	// validate that the selected snapshotter supports the filesystem at the root path.
	// for stargz, also overrides the image service endpoint path.
	switch nodeConfig.AgentConfig.Snapshotter {
	case "overlayfs":
		if err := containerd.OverlaySupported(nodeConfig.Containerd.Root); err != nil {
			return errors.Wrapf(err, "\"overlayfs\" snapshotter cannot be enabled for %q, try using \"fuse-overlayfs\" or \"native\"",
				nodeConfig.Containerd.Root)
		}
	case "fuse-overlayfs":
		if err := containerd.FuseoverlayfsSupported(nodeConfig.Containerd.Root); err != nil {
			return errors.Wrapf(err, "\"fuse-overlayfs\" snapshotter cannot be enabled for %q, try using \"native\"",
				nodeConfig.Containerd.Root)
		}
	case "stargz":
		if err := containerd.StargzSupported(nodeConfig.Containerd.Root); err != nil {
			return errors.Wrapf(err, "\"stargz\" snapshotter cannot be enabled for %q, try using \"overlayfs\" or \"native\"",
				nodeConfig.Containerd.Root)
		}
		nodeConfig.AgentConfig.ImageServiceSocket = "/run/containerd-stargz-grpc/containerd-stargz-grpc.sock"
	}

	return nil
}

// applyContainerdQoSClassConfigFileIfPresent sets linux-specific qos config
func applyContainerdQoSClassConfigFileIfPresent(envInfo *cmds.Agent, containerdConfig *config.Containerd) error {
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

	return nil
}

// applyCRIDockerdOSSpecificConfig sets linux-specific cri-dockerd config
func applyCRIDockerdOSSpecificConfig(nodeConfig *config.Node) error {
	nodeConfig.CRIDockerd.Address = "unix:///run/k3s/cri-dockerd/cri-dockerd.sock"
	return nil
}

// configureACL will configure an Access Control List for the specified file.
// On Linux, this function is a no-op
func configureACL(file string) error {
	return nil
}
