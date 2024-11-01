//go:build linux
// +build linux

package containerd

import (
	"os"

	"github.com/containerd/containerd"
	overlayutils "github.com/containerd/containerd/snapshots/overlay/overlayutils"
	fuseoverlayfs "github.com/containerd/fuse-overlayfs-snapshotter"
	stargz "github.com/containerd/stargz-snapshotter/service"
	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/k3s-io/k3s/pkg/agent/templates"
	"github.com/k3s-io/k3s/pkg/cgroups"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/opencontainers/runc/libcontainer/userns"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	"k8s.io/cri-client/pkg/util"
)

const (
	socketPrefix = "unix://"
	runtimesPath = "/usr/local/nvidia/toolkit:/opt/kwasm/bin"
)

func getContainerdArgs(cfg *config.Node) []string {
	args := []string{
		"containerd",
		"-c", cfg.Containerd.Config,
		"-a", cfg.Containerd.Address,
		"--state", cfg.Containerd.State,
		"--root", cfg.Containerd.Root,
	}
	return args
}

// SetupContainerdConfig generates the containerd.toml, using a template combined with various
// runtime configurations and registry mirror settings provided by the administrator.
func SetupContainerdConfig(cfg *config.Node) error {
	isRunningInUserNS := userns.RunningInUserNS()
	_, _, controllers := cgroups.CheckCgroups()
	// "/sys/fs/cgroup" is namespaced
	cgroupfsWritable := unix.Access("/sys/fs/cgroup", unix.W_OK) == nil
	disableCgroup := isRunningInUserNS && (!controllers["cpu"] || !controllers["pids"] || !cgroupfsWritable)
	if disableCgroup {
		logrus.Warn("cgroup v2 controllers are not delegated for rootless. Disabling cgroup.")
	} else {
		// note: this mutatation of the passed agent.Config is later used to set the
		// kubelet's cgroup-driver flag. This may merit moving to somewhere else in order
		// to avoid mutating the configuration while setting up containerd.
		cfg.AgentConfig.Systemd = !isRunningInUserNS && controllers["cpuset"] && os.Getenv("INVOCATION_ID") != ""
	}

	// set the path to include the default runtimes and remove the aditional path entries
	// that we added after finding the runtimes
	originalPath := os.Getenv("PATH")
	os.Setenv("PATH", runtimesPath+string(os.PathListSeparator)+originalPath)
	extraRuntimes := findContainerRuntimes()
	os.Setenv("PATH", originalPath)

	// Verifies if the DefaultRuntime can be found
	if _, ok := extraRuntimes[cfg.DefaultRuntime]; !ok && cfg.DefaultRuntime != "" {
		return errors.Errorf("default runtime %s was not found", cfg.DefaultRuntime)
	}

	containerdConfig := templates.ContainerdConfig{
		NodeConfig:            cfg,
		DisableCgroup:         disableCgroup,
		SystemdCgroup:         cfg.AgentConfig.Systemd,
		IsRunningInUserNS:     isRunningInUserNS,
		EnableUnprivileged:    kernel.CheckKernelVersion(4, 11, 0),
		PrivateRegistryConfig: cfg.AgentConfig.Registry,
		ExtraRuntimes:         extraRuntimes,
		Program:               version.Program,
		NoDefaultEndpoint:     cfg.Containerd.NoDefault,
	}

	selEnabled, selConfigured, err := selinuxStatus()
	if err != nil {
		return errors.Wrap(err, "failed to detect selinux")
	}
	switch {
	case !cfg.SELinux && selEnabled:
		logrus.Warn("SELinux is enabled on this host, but " + version.Program + " has not been started with --selinux - containerd SELinux support is disabled")
	case cfg.SELinux && !selConfigured:
		logrus.Warnf("SELinux is enabled for "+version.Program+" but process is not running in context '%s', "+version.Program+"-selinux policy may need to be applied", SELinuxContextType)
	}

	if err := writeContainerdConfig(cfg, containerdConfig); err != nil {
		return err
	}

	return writeContainerdHosts(cfg, containerdConfig)
}

func Client(address string) (*containerd.Client, error) {
	addr, _, err := util.GetAddressAndDialer(socketPrefix + address)
	if err != nil {
		return nil, err
	}

	return containerd.New(addr)
}

func OverlaySupported(root string) error {
	return overlayutils.Supported(root)
}

func FuseoverlayfsSupported(root string) error {
	return fuseoverlayfs.Supported(root)
}

func StargzSupported(root string) error {
	return stargz.Supported(root)
}
