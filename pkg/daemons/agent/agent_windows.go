//go:build windows
// +build windows

package agent

import (
	"net"
	"path/filepath"
	"strings"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/sirupsen/logrus"
	kubeletconfig "k8s.io/kubelet/config/v1beta1"
	utilsnet "k8s.io/utils/net"
	utilsptr "k8s.io/utils/ptr"
)

const (
	socketPrefix = "npipe://"
)

func kubeProxyArgs(cfg *config.Agent) map[string]string {
	bindAddress := "127.0.0.1"
	if utilsnet.IsIPv6(net.ParseIP(cfg.NodeIP)) {
		bindAddress = "::1"
	}
	argsMap := map[string]string{
		"proxy-mode":           "kernelspace",
		"healthz-bind-address": bindAddress,
		"kubeconfig":           cfg.KubeConfigKubeProxy,
		"cluster-cidr":         util.JoinIPNets(cfg.ClusterCIDRs),
	}
	if cfg.NodeName != "" {
		argsMap["hostname-override"] = cfg.NodeName
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

	argsMap["node-labels"] = strings.Join(cfg.NodeLabels, ",")

	if ImageCredProvAvailable(cfg) {
		logrus.Infof("Kubelet image credential provider bin dir and configuration file found.")
		argsMap["image-credential-provider-bin-dir"] = cfg.ImageCredProvBinDir
		argsMap["image-credential-provider-config"] = cfg.ImageCredProvConfig
	}

	return argsMap, defaultConfig, nil
}
