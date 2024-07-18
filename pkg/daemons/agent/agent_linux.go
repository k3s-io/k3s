//go:build linux
// +build linux

package agent

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/k3s-io/k3s/pkg/cgroups"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	"k8s.io/kubernetes/pkg/kubeapiserver/authorizer/modes"
	utilsnet "k8s.io/utils/net"
)

const socketPrefix = "unix://"

func createRootlessConfig(argsMap map[string]string, controllers map[string]bool) {
	argsMap["feature-gates=KubeletInUserNamespace"] = "true"
	// "/sys/fs/cgroup" is namespaced
	cgroupfsWritable := unix.Access("/sys/fs/cgroup", unix.W_OK) == nil
	if controllers["cpu"] && controllers["pids"] && cgroupfsWritable {
		logrus.Info("cgroup v2 controllers are delegated for rootless.")
		return
	}
	logrus.Fatal("delegated cgroup v2 controllers are required for rootless.")
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

func kubeletArgs(cfg *config.Agent) map[string]string {
	bindAddress := "127.0.0.1"
	if utilsnet.IsIPv6(net.ParseIP(cfg.NodeIP)) {
		bindAddress = "::1"
	}
	argsMap := map[string]string{
		"healthz-bind-address":         bindAddress,
		"read-only-port":               "0",
		"cluster-domain":               cfg.ClusterDomain,
		"kubeconfig":                   cfg.KubeConfigKubelet,
		"eviction-hard":                "imagefs.available<5%,nodefs.available<5%",
		"eviction-minimum-reclaim":     "imagefs.available=10%,nodefs.available=10%",
		"fail-swap-on":                 "false",
		"cgroup-driver":                "cgroupfs",
		"authentication-token-webhook": "true",
		"anonymous-auth":               "false",
		"authorization-mode":           modes.ModeWebhook,
	}
	if cfg.PodManifests != "" && argsMap["pod-manifest-path"] == "" {
		argsMap["pod-manifest-path"] = cfg.PodManifests
	}
	if err := os.MkdirAll(argsMap["pod-manifest-path"], 0755); err != nil {
		logrus.Errorf("Failed to mkdir %s: %v", argsMap["pod-manifest-path"], err)
	}
	if cfg.RootDir != "" {
		argsMap["root-dir"] = cfg.RootDir
		argsMap["cert-dir"] = filepath.Join(cfg.RootDir, "pki")
	}
	if len(cfg.ClusterDNS) > 0 {
		argsMap["cluster-dns"] = util.JoinIPs(cfg.ClusterDNSs)
	}
	if cfg.ResolvConf != "" {
		argsMap["resolv-conf"] = cfg.ResolvConf
	}
	if cfg.RuntimeSocket != "" {
		argsMap["serialize-image-pulls"] = "false"
		if strings.Contains(cfg.RuntimeSocket, "containerd") {
			argsMap["containerd"] = cfg.RuntimeSocket
		}
		// cadvisor wants the containerd CRI socket without the prefix, but kubelet wants it with the prefix
		if strings.HasPrefix(cfg.RuntimeSocket, socketPrefix) {
			argsMap["container-runtime-endpoint"] = cfg.RuntimeSocket
		} else {
			argsMap["container-runtime-endpoint"] = socketPrefix + cfg.RuntimeSocket
		}
	}
	if cfg.ImageServiceSocket != "" {
		if strings.HasPrefix(cfg.ImageServiceSocket, socketPrefix) {
			argsMap["image-service-endpoint"] = cfg.ImageServiceSocket
		} else {
			argsMap["image-service-endpoint"] = socketPrefix + cfg.ImageServiceSocket
		}
	}
	if cfg.ListenAddress != "" {
		argsMap["address"] = cfg.ListenAddress
	}
	if cfg.ClientCA != "" {
		argsMap["anonymous-auth"] = "false"
		argsMap["client-ca-file"] = cfg.ClientCA
	}
	if cfg.ServingKubeletCert != "" && cfg.ServingKubeletKey != "" {
		argsMap["tls-cert-file"] = cfg.ServingKubeletCert
		argsMap["tls-private-key-file"] = cfg.ServingKubeletKey
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
		// Cluster is using the embedded CCM, we know that the feature-gate will be enabled there as well.
		argsMap["feature-gates"] = util.AddFeatureGate(argsMap["feature-gates"], "CloudDualStackNodeIPs=true")
		if nodeIPs := util.JoinIPs(cfg.NodeIPs); nodeIPs != "" {
			argsMap["node-ip"] = util.JoinIPs(cfg.NodeIPs)
		}
	}
	kubeletRoot, runtimeRoot, controllers := cgroups.CheckCgroups()
	if !controllers["cpu"] {
		logrus.Warn("Disabling CPU quotas due to missing cpu controller or cpu.cfs_period_us")
		argsMap["cpu-cfs-quota"] = "false"
	}
	if !controllers["pids"] {
		logrus.Fatal("pids cgroup controller not found")
	}
	if kubeletRoot != "" {
		argsMap["kubelet-cgroups"] = kubeletRoot
	}
	if runtimeRoot != "" {
		argsMap["runtime-cgroups"] = runtimeRoot
	}

	argsMap["node-labels"] = strings.Join(cfg.NodeLabels, ",")
	if len(cfg.NodeTaints) > 0 {
		argsMap["register-with-taints"] = strings.Join(cfg.NodeTaints, ",")
	}

	if !cfg.DisableCCM {
		argsMap["cloud-provider"] = "external"
	}

	if ImageCredProvAvailable(cfg) {
		logrus.Infof("Kubelet image credential provider bin dir and configuration file found.")
		argsMap["image-credential-provider-bin-dir"] = cfg.ImageCredProvBinDir
		argsMap["image-credential-provider-config"] = cfg.ImageCredProvConfig
	}

	if cfg.Rootless {
		createRootlessConfig(argsMap, controllers)
	}

	if cfg.Systemd {
		argsMap["cgroup-driver"] = "systemd"
	}

	if cfg.ProtectKernelDefaults {
		argsMap["protect-kernel-defaults"] = "true"
	}

	if !cfg.DisableServiceLB {
		argsMap["allowed-unsafe-sysctls"] = "net.ipv4.ip_forward,net.ipv6.conf.all.forwarding"
	}
	if cfg.VLevel != 0 {
		argsMap["v"] = strconv.Itoa(cfg.VLevel)
	}
	if cfg.VModule != "" {
		argsMap["vmodule"] = cfg.VModule
	}
	return argsMap
}
