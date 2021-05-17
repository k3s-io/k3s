package agent

import (
	"bufio"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containerd/cgroups"
	cgroupsv2 "github.com/containerd/cgroups/v2"
	"github.com/opencontainers/runc/libcontainer/system"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/daemons/executor"
	"github.com/rancher/k3s/pkg/util"
	"github.com/rancher/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/component-base/logs"
	"k8s.io/kubernetes/pkg/kubeapiserver/authorizer/modes"

	_ "k8s.io/component-base/metrics/prometheus/restclient" // for client metric registration
	_ "k8s.io/component-base/metrics/prometheus/version"    // for version metric registration
)

const unixPrefix = "unix://"

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
		"proxy-mode":                        "iptables",
		"healthz-bind-address":              "127.0.0.1",
		"kubeconfig":                        cfg.KubeConfigKubeProxy,
		"cluster-cidr":                      util.JoinIPNets(cfg.ClusterCIDRs),
		"conntrack-max-per-core":            "0",
		"conntrack-tcp-timeout-established": "0s",
		"conntrack-tcp-timeout-close-wait":  "0s",
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
		argsMap["cluster-dns"] = util.JoinIPs(cfg.ClusterDNSs)
	}
	if cfg.ResolvConf != "" {
		argsMap["resolv-conf"] = cfg.ResolvConf
	}
	if cfg.RuntimeSocket != "" {
		argsMap["container-runtime"] = "remote"
		argsMap["containerd"] = cfg.RuntimeSocket
		argsMap["serialize-image-pulls"] = "false"
		if strings.HasPrefix(argsMap["container-runtime-endpoint"], unixPrefix) {
			argsMap["container-runtime-endpoint"] = cfg.RuntimeSocket
		} else {
			argsMap["container-runtime-endpoint"] = unixPrefix + cfg.RuntimeSocket
		}
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
	kubeletRoot, runtimeRoot, hasCFS, hasPIDs := CheckCgroups()
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
	if kubeletRoot != "" {
		argsMap["kubelet-cgroups"] = kubeletRoot
	}
	if runtimeRoot != "" {
		argsMap["runtime-cgroups"] = runtimeRoot
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

	if ImageCredProvAvailable(cfg) {
		logrus.Infof("Kubelet image credential provider bin dir and configuration file found.")
		argsMap["feature-gates"] = addFeatureGate(argsMap["feature-gates"], "KubeletCredentialProviders=true")
		argsMap["image-credential-provider-bin-dir"] = cfg.ImageCredProvBinDir
		argsMap["image-credential-provider-config"] = cfg.ImageCredProvConfig
	}

	if cfg.Rootless {
		// "/sys/fs/cgroup" is namespaced
		cgroupfsWritable := unix.Access("/sys/fs/cgroup", unix.W_OK) == nil
		if hasCFS && hasPIDs && cgroupfsWritable {
			logrus.Info("cgroup v2 controllers are delegated for rootless.")
			// cgroupfs v2, delegated for rootless by systemd
			argsMap["cgroup-driver"] = "cgroupfs"
		} else {
			logrus.Warn("cgroup v2 controllers are not delegated for rootless. Setting cgroup driver to \"none\".")
			// flags are from https://github.com/rootless-containers/usernetes/blob/v20190826.0/boot/kubelet.sh
			argsMap["cgroup-driver"] = "none"
			argsMap["feature-gates=SupportNoneCgroupDriver"] = "true"
			argsMap["cgroups-per-qos"] = "false"
			argsMap["enforce-node-allocatable"] = ""
		}
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

// ImageCredProvAvailable checks to see if the kubelet image credential provider bin dir and config
// files exist and are of the correct types. This is exported so that it may be used by downstream projects.
func ImageCredProvAvailable(cfg *config.Agent) bool {
	if info, err := os.Stat(cfg.ImageCredProvBinDir); err != nil || !info.IsDir() {
		logrus.Debugf("Kubelet image credential provider bin directory check failed: %v", err)
		return false
	}
	if info, err := os.Stat(cfg.ImageCredProvConfig); err != nil || info.IsDir() {
		logrus.Debugf("Kubelet image credential provider config file check failed: %v", err)
		return false
	}
	return true
}

func CheckCgroups() (kubeletRoot, runtimeRoot string, hasCFS, hasPIDs bool) {
	cgroupsModeV2 := cgroups.Mode() == cgroups.Unified

	// For Unified (v2) cgroups we can directly check to see what controllers are mounted
	// under the unified hierarchy.
	if cgroupsModeV2 {
		m, err := cgroupsv2.LoadManager("/sys/fs/cgroup", "/")
		if err != nil {
			return "", "", false, false
		}
		controllers, err := m.Controllers()
		if err != nil {
			return "", "", false, false
		}
		// Intentionally using an expressionless switch to match the logic below
		for _, controller := range controllers {
			switch {
			case controller == "cpu":
				hasCFS = true
			case controller == "pids":
				hasPIDs = true
			}
		}
	}

	f, err := os.Open("/proc/self/cgroup")
	if err != nil {
		return "", "", false, false
	}
	defer f.Close()

	scan := bufio.NewScanner(f)
	for scan.Scan() {
		parts := strings.Split(scan.Text(), ":")
		if len(parts) < 3 {
			continue
		}
		controllers := strings.Split(parts[1], ",")
		// For v1 or hybrid, controller can be a single value {"blkio"}, or a comounted set {"cpu","cpuacct"}
		// For v2, controllers = {""} (only contains a single empty string)
		for _, controller := range controllers {
			switch {
			case controller == "name=systemd" || cgroupsModeV2:
				// If we detect that we are running under a `.scope` unit with systemd
				// we can assume we are being directly invoked from the command line
				// and thus need to set our kubelet root to something out of the context
				// of `/user.slice` to ensure that `CPUAccounting` and `MemoryAccounting`
				// are enabled, as they are generally disabled by default for `user.slice`
				// Note that we are not setting the `runtimeRoot` as if we are running with
				// `--docker`, we will inadvertently move the cgroup `dockerd` lives in
				//  which is not ideal and causes dockerd to become unmanageable by systemd.
				last := parts[len(parts)-1]
				i := strings.LastIndex(last, ".scope")
				if i > 0 {
					kubeletRoot = "/" + version.Program
				}
			case controller == "cpu":
				// It is common for this to show up multiple times in /sys/fs/cgroup if the controllers are comounted:
				// as "cpu" and "cpuacct", symlinked to the actual hierarchy at "cpu,cpuacct". Unfortunately the order
				// listed in /proc/self/cgroups may not be the same order used in /sys/fs/cgroup, so this check
				// can fail if we use the comma-separated name. Instead, we check for the controller using the symlink.
				p := filepath.Join("/sys/fs/cgroup", controller, parts[2], "cpu.cfs_period_us")
				if _, err := os.Stat(p); err == nil {
					hasCFS = true
				}
			case controller == "pids":
				hasPIDs = true
			}
		}
	}

	// If we're running with v1 and didn't find a scope assigned by systemd, we need to create our own root cgroup to avoid
	// just inheriting from the parent process. The kubelet will take care of moving us into it when we start it up later.
	if kubeletRoot == "" {
		// Examine process ID 1 to see if there is a cgroup assigned to it.
		// When we are not in a container, process 1 is likely to be systemd or some other service manager.
		// It either lives at `/` or `/init.scope` according to https://man7.org/linux/man-pages/man7/systemd.special.7.html
		// When containerized, process 1 will be generally be in a cgroup, otherwise, we may be running in
		// a host PID scenario but we don't support this.
		g, err := os.Open("/proc/1/cgroup")
		if err != nil {
			return "", "", false, false
		}
		defer g.Close()
		scan = bufio.NewScanner(g)
		for scan.Scan() {
			parts := strings.Split(scan.Text(), ":")
			if len(parts) < 3 {
				continue
			}
			controllers := strings.Split(parts[1], ",")
			// For v1 or hybrid, controller can be a single value {"blkio"}, or a comounted set {"cpu","cpuacct"}
			// For v2, controllers = {""} (only contains a single empty string)
			for _, controller := range controllers {
				switch {
				case controller == "name=systemd" || cgroupsModeV2:
					last := parts[len(parts)-1]
					if last != "/" && last != "/init.scope" {
						kubeletRoot = "/" + version.Program
						runtimeRoot = "/" + version.Program
					}
				}
			}
		}
	}
	return kubeletRoot, runtimeRoot, hasCFS, hasPIDs
}
