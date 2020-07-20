package agent

import (
	"bufio"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencontainers/runc/libcontainer/system"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/daemons/executor"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/component-base/logs"
	"k8s.io/kubernetes/pkg/kubeapiserver/authorizer/modes"

	_ "k8s.io/component-base/metrics/prometheus/restclient" // for client metric registration
	_ "k8s.io/component-base/metrics/prometheus/version"    // for version metric registration
)

func Agent(config *config.Agent) error {
	rand.Seed(time.Now().UTC().UnixNano())

	logs.InitLogs()
	defer logs.FlushLogs()

	if err := startKubelet(config); err != nil {
		return err
	}

	if !config.DisableKubeProxy {
		return startKubeProxy(config)
	}

	return nil
}

func startKubeProxy(cfg *config.Agent) error {
	argsMap := map[string]string{
		"proxy-mode":           "iptables",
		"healthz-bind-address": "127.0.0.1",
		"kubeconfig":           cfg.KubeConfigKubeProxy,
		"cluster-cidr":         cfg.ClusterCIDR.String(),
	}
	if cfg.NodeName != "" {
		argsMap["hostname-override"] = cfg.NodeName
	}

	args := config.GetArgsList(argsMap, cfg.ExtraKubeProxyArgs)
	logrus.Infof("Running kube-proxy %s", config.ArgString(args))
	return executor.KubeProxy(args)
}

func startKubelet(cfg *config.Agent) error {
	argsMap := map[string]string{
		"healthz-bind-address":     "127.0.0.1",
		"read-only-port":           "0",
		"cluster-domain":           cfg.ClusterDomain,
		"kubeconfig":               cfg.KubeConfigKubelet,
		"eviction-hard":            "imagefs.available<5%,nodefs.available<5%",
		"eviction-minimum-reclaim": "imagefs.available=10%,nodefs.available=10%",
		"fail-swap-on":             "false",
		//"cgroup-root": "/k3s",
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
		argsMap["seccomp-profile-root"] = filepath.Join(cfg.RootDir, "seccomp")
	}
	if cfg.CNIConfDir != "" {
		argsMap["cni-conf-dir"] = cfg.CNIConfDir
	}
	if cfg.CNIBinDir != "" {
		argsMap["cni-bin-dir"] = cfg.CNIBinDir
	}
	if cfg.CNIPlugin {
		argsMap["network-plugin"] = "cni"
	}
	if len(cfg.ClusterDNS) > 0 {
		argsMap["cluster-dns"] = cfg.ClusterDNS.String()
	}
	if cfg.ResolvConf != "" {
		argsMap["resolv-conf"] = cfg.ResolvConf
	}
	if cfg.RuntimeSocket != "" {
		argsMap["container-runtime"] = "remote"
		argsMap["container-runtime-endpoint"] = cfg.RuntimeSocket
		argsMap["containerd"] = cfg.RuntimeSocket
		argsMap["serialize-image-pulls"] = "false"
	} else if cfg.PauseImage != "" {
		argsMap["pod-infra-container-image"] = cfg.PauseImage
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
	root, hasCFS, hasPIDs := checkCgroups()
	if !hasCFS {
		logrus.Warn("Disabling CPU quotas due to missing cpu.cfs_period_us")
		argsMap["cpu-cfs-quota"] = "false"
	}
	if !hasPIDs {
		logrus.Warn("Disabling pod PIDs limit feature due to missing cgroup pids support")
		argsMap["cgroups-per-qos"] = "false"
		argsMap["enforce-node-allocatable"] = ""
		argsMap["feature-gates"] = addFeatureGate(argsMap["feature-gates"], "SupportPodPidsLimit=false")
	}
	if root != "" {
		argsMap["runtime-cgroups"] = root
		argsMap["kubelet-cgroups"] = root
	}
	if system.RunningInUserNS() {
		argsMap["feature-gates"] = addFeatureGate(argsMap["feature-gates"], "DevicePlugins=false")
	}

	argsMap["node-labels"] = strings.Join(cfg.NodeLabels, ",")
	if len(cfg.NodeTaints) > 0 {
		argsMap["register-with-taints"] = strings.Join(cfg.NodeTaints, ",")
	}
	if !cfg.DisableCCM {
		argsMap["cloud-provider"] = "external"
	}

	if cfg.Rootless {
		// flags are from https://github.com/rootless-containers/usernetes/blob/v20190826.0/boot/kubelet.sh
		argsMap["cgroup-driver"] = "none"
		argsMap["feature-gates=SupportNoneCgroupDriver"] = "true"
		argsMap["cgroups-per-qos"] = "false"
		argsMap["enforce-node-allocatable"] = ""
	}

	if cfg.ProtectKernelDefaults {
		argsMap["protect-kernel-defaults"] = "true"
	}

	args := config.GetArgsList(argsMap, cfg.ExtraKubeletArgs)
	logrus.Infof("Running kubelet %s", config.ArgString(args))

	return executor.Kubelet(args)
}

func addFeatureGate(current, new string) string {
	if current == "" {
		return new
	}
	return current + "," + new
}

func checkCgroups() (root string, hasCFS bool, hasPIDs bool) {
	f, err := os.Open("/proc/self/cgroup")
	if err != nil {
		return "", false, false
	}
	defer f.Close()

	scan := bufio.NewScanner(f)
	for scan.Scan() {
		parts := strings.Split(scan.Text(), ":")
		if len(parts) < 3 {
			continue
		}
		systems := strings.Split(parts[1], ",")
		for _, system := range systems {
			if system == "pids" {
				hasPIDs = true
			} else if system == "cpu" {
				p := filepath.Join("/sys/fs/cgroup", parts[1], parts[2], "cpu.cfs_period_us")
				if _, err := os.Stat(p); err == nil {
					hasCFS = true
				}
			} else if system == "name=systemd" {
				last := parts[len(parts)-1]
				i := strings.LastIndex(last, ".slice")
				if i > 0 {
					root = "/systemd" + last[:i+len(".slice")]
				} else {
					root = "/systemd"
				}
			}
		}
	}

	return root, hasCFS, hasPIDs
}
