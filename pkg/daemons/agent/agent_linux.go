//go:build linux
// +build linux

package agent

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/k3s-io/k3s/pkg/cgroups"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/opencontainers/runc/libcontainer/userns"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/kubernetes/pkg/kubeapiserver/authorizer/modes"
)

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

func checkRuntimeEndpoint(cfg *config.Agent, argsMap map[string]string) {
	if strings.HasPrefix(argsMap["container-runtime-endpoint"], unixPrefix) {
		argsMap["container-runtime-endpoint"] = cfg.RuntimeSocket
	} else {
		argsMap["container-runtime-endpoint"] = unixPrefix + cfg.RuntimeSocket
	}
}

func kubeProxyArgs(cfg *config.Agent) map[string]string {
	bindAddress := "127.0.0.1"
	_, IPv6only, _ := util.GetFirstString([]string{cfg.NodeIP})
	if IPv6only {
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
	return argsMap
}

func kubeletArgs(cfg *config.Agent) map[string]string {
	bindAddress := "127.0.0.1"
	_, IPv6only, _ := util.GetFirstString([]string{cfg.NodeIP})
	if IPv6only {
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
		argsMap["containerd"] = cfg.RuntimeSocket
		argsMap["serialize-image-pulls"] = "false"
		checkRuntimeEndpoint(cfg, argsMap)
	}
	if cfg.PauseImage != "" {
		argsMap["pod-infra-container-image"] = cfg.PauseImage
	}
	if cfg.ImageServiceSocket != "" {
		if strings.HasPrefix(cfg.ImageServiceSocket, unixPrefix) {
			argsMap["image-service-endpoint"] = cfg.ImageServiceSocket
		} else {
			argsMap["image-service-endpoint"] = unixPrefix + cfg.ImageServiceSocket
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
	defaultIP, err := net.ChooseHostInterface()
	if err != nil || defaultIP.String() != cfg.NodeIP {
		argsMap["node-ip"] = cfg.NodeIP
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
	if userns.RunningInUserNS() {
		argsMap["feature-gates"] = util.AddFeatureGate(argsMap["feature-gates"], "DevicePlugins=false")
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
		argsMap["feature-gates"] = util.AddFeatureGate(argsMap["feature-gates"], "KubeletCredentialProviders=true")
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

	if !cfg.DisableServiceLB && cfg.EnableIPv6 {
		argsMap["allowed-unsafe-sysctls"] = "net.ipv6.conf.all.forwarding"
	}

	return argsMap
}
