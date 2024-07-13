//go:build windows
// +build windows

package agent

import (
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/sirupsen/logrus"
	"k8s.io/kubernetes/pkg/kubeapiserver/authorizer/modes"
	utilsnet "k8s.io/utils/net"
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
		// cadvisor wants the containerd CRI socket without the prefix, but kubelet wants it with the prefix
		if strings.HasPrefix(cfg.RuntimeSocket, socketPrefix) {
			argsMap["container-runtime-endpoint"] = cfg.RuntimeSocket
		} else {
			argsMap["container-runtime-endpoint"] = socketPrefix + cfg.RuntimeSocket
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

	if cfg.ProtectKernelDefaults {
		argsMap["protect-kernel-defaults"] = "true"
	}

	return argsMap
}
