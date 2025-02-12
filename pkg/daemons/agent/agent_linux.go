//go:build linux
// +build linux

package agent

import (
	"net"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/k3s-io/k3s/pkg/cgroups"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	kubeletconfig "k8s.io/kubelet/config/v1beta1"
	utilsnet "k8s.io/utils/net"
	utilsptr "k8s.io/utils/ptr"
)

const socketPrefix = "unix://"

func createRootlessConfig(argsMap map[string]string, controllers map[string]bool) error {
	argsMap["feature-gates=KubeletInUserNamespace"] = "true"
	// "/sys/fs/cgroup" is namespaced
	cgroupfsWritable := unix.Access("/sys/fs/cgroup", unix.W_OK) == nil
	if controllers["cpu"] && controllers["pids"] && cgroupfsWritable {
		logrus.Info("cgroup v2 controllers are delegated for rootless.")
		return nil
	}
	return errors.New("delegated cgroup v2 controllers are required for rootless")
}

func kubeProxyArgs(cfg *config.Agent) map[string]string {
	bindAddress := "127.0.0.1"
	if utilsnet.IsIPv6(net.ParseIP(cfg.NodeIP)) {
		bindAddress = "::1"
	}
	argsMap := map[string]string{
		"proxy-mode":                        "iptables",
		"healthz-bind-address":              bindAddress,
		"kubeconfig":                        cfg.KubeConfigKubeProxy,
		"cluster-cidr":                      util.JoinIPNets(cfg.ClusterCIDRs),
		"conntrack-max-per-core":            "0",
		"conntrack-tcp-timeout-established": "0s",
		"conntrack-tcp-timeout-close-wait":  "0s",
	}
	if cfg.NodeName != "" {
		argsMap["hostname-override"] = cfg.NodeName
	}
	if cfg.VLevel != 0 {
		argsMap["v"] = strconv.Itoa(cfg.VLevel)
	}
	if cfg.VModule != "" {
		argsMap["vmodule"] = cfg.VModule
	}
	if cfg.LogFile != "" {
		argsMap["log_file"] = cfg.LogFile
	}
	if cfg.AlsoLogToStderr {
		argsMap["alsologtostderr"] = "true"
	}
	return argsMap
}

// kubeletArgsAndConfig generates default kubelet args and configuration.
// Kubelet config is frustratingly split across deprecated CLI flags that raise warnings if you use them,
// and a structured configuration file that upstream does not provide a convienent way to initailize with default values.
// The defaults and our desired config also vary by OS.
func kubeletArgsAndConfig(cfg *config.Agent) (map[string]string, *kubeletconfig.KubeletConfiguration, error) {
	defaultConfig, err := defaultKubeletConfig(cfg)
	if err != nil {
		return nil, nil, err
	}
	argsMap := map[string]string{
		"config-dir": cfg.KubeletConfigDir,
		"kubeconfig": cfg.KubeConfigKubelet,
	}

	if cfg.RootDir != "" {
		argsMap["root-dir"] = cfg.RootDir
		argsMap["cert-dir"] = filepath.Join(cfg.RootDir, "pki")
	}
	if cfg.RuntimeSocket != "" {
		defaultConfig.SerializeImagePulls = utilsptr.To(false)
		// note: this is a legacy cadvisor flag that the kubelet still exposes, but
		// it must be set in order for cadvisor to pull stats properly.
		if strings.Contains(cfg.RuntimeSocket, "containerd") {
			argsMap["containerd"] = cfg.RuntimeSocket
		}
		// cadvisor wants the containerd CRI socket without the prefix, but kubelet wants it with the prefix
		if strings.HasPrefix(cfg.RuntimeSocket, socketPrefix) {
			defaultConfig.ContainerRuntimeEndpoint = cfg.RuntimeSocket
		} else {
			defaultConfig.ContainerRuntimeEndpoint = socketPrefix + cfg.RuntimeSocket
		}
	}
	if cfg.ImageServiceSocket != "" {
		if strings.HasPrefix(cfg.ImageServiceSocket, socketPrefix) {
			defaultConfig.ImageServiceEndpoint = cfg.ImageServiceSocket
		} else {
			defaultConfig.ImageServiceEndpoint = socketPrefix + cfg.ImageServiceSocket
		}
	}
	if cfg.NodeName != "" {
		argsMap["hostname-override"] = cfg.NodeName
	}

	// If the embedded CCM is disabled, don't assume that dual-stack node IPs are safe.
	// When using an external CCM, the user wants dual-stack node IPs, they will need to set the node-ip kubelet arg directly.
	// This should be fine since most cloud providers have their own way of finding node IPs that doesn't depend on the kubelet
	// setting them.
	if cfg.DisableCCM {
		dualStack, err := utilsnet.IsDualStackIPs(cfg.NodeIPs)
		if err == nil && !dualStack {
			argsMap["node-ip"] = cfg.NodeIP
		}
	} else {
		argsMap["cloud-provider"] = "external"
		if nodeIPs := util.JoinIPs(cfg.NodeIPs); nodeIPs != "" {
			argsMap["node-ip"] = util.JoinIPs(cfg.NodeIPs)
		}
	}

	kubeletRoot, runtimeRoot, controllers := cgroups.CheckCgroups()
	if !controllers["pids"] {
		return nil, nil, errors.New("pids cgroup controller not found")
	}
	if !controllers["cpu"] {
		logrus.Warn("Disabling CPU quotas due to missing cpu controller or cpu.cfs_period_us")
		defaultConfig.CPUCFSQuota = utilsptr.To(false)
	}
	if kubeletRoot != "" {
		defaultConfig.KubeletCgroups = kubeletRoot
	}
	if runtimeRoot != "" {
		argsMap["runtime-cgroups"] = runtimeRoot
	}

	argsMap["node-labels"] = strings.Join(cfg.NodeLabels, ",")

	if ImageCredProvAvailable(cfg) {
		logrus.Infof("Kubelet image credential provider bin dir and configuration file found.")
		argsMap["image-credential-provider-bin-dir"] = cfg.ImageCredProvBinDir
		argsMap["image-credential-provider-config"] = cfg.ImageCredProvConfig
	}

	if cfg.Rootless {
		if err := createRootlessConfig(argsMap, controllers); err != nil {
			return nil, nil, err
		}
	}

	if cfg.Systemd {
		defaultConfig.CgroupDriver = "systemd"
	}

	if !cfg.DisableServiceLB {
		defaultConfig.AllowedUnsafeSysctls = []string{"net.ipv4.ip_forward", "net.ipv6.conf.all.forwarding"}
	}

	return argsMap, defaultConfig, nil
}
