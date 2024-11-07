//go:build windows
// +build windows

package containerd

import (
	"github.com/containerd/containerd"
	"github.com/k3s-io/k3s/pkg/agent/templates"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	util3 "github.com/k3s-io/k3s/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/cri-client/pkg/util"
)

func getContainerdArgs(cfg *config.Node) []string {
	args := []string{
		"containerd",
		"-c", cfg.Containerd.Config,
	}
	return args
}

// SetupContainerdConfig generates the containerd.toml, using a template combined with various
// runtime configurations and registry mirror settings provided by the administrator.
func SetupContainerdConfig(cfg *config.Node) error {
	if cfg.SELinux {
		logrus.Warn("SELinux isn't supported on windows")
	}

	containerdConfig := templates.ContainerdConfig{
		NodeConfig:            cfg,
		DisableCgroup:         true,
		SystemdCgroup:         false,
		IsRunningInUserNS:     false,
		PrivateRegistryConfig: cfg.AgentConfig.Registry,
		NoDefaultEndpoint:     cfg.Containerd.NoDefault,
	}

	if err := writeContainerdConfig(cfg, containerdConfig); err != nil {
		return err
	}

	return writeContainerdHosts(cfg, containerdConfig)
}

func Client(address string) (*containerd.Client, error) {
	addr, _, err := util.GetAddressAndDialer(address)
	if err != nil {
		return nil, err
	}

	return containerd.New(addr)
}

func OverlaySupported(root string) error {
	return errors.Wrapf(util3.ErrUnsupportedPlatform, "overlayfs is not supported")
}

func FuseoverlayfsSupported(root string) error {
	return errors.Wrapf(util3.ErrUnsupportedPlatform, "fuse-overlayfs is not supported")
}

func StargzSupported(root string) error {
	return errors.Wrapf(util3.ErrUnsupportedPlatform, "stargz is not supported")
}
