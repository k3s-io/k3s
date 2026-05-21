//go:build !windows

package syssetup

import (
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/google/cadvisor/machine"
	"github.com/google/cadvisor/utils/sysfs"
	"github.com/sirupsen/logrus"
	"k8s.io/component-helpers/node/util/sysctl"
	kubeproxyconfig "k8s.io/kubernetes/pkg/proxy/apis/config"
)

func loadKernelModule(moduleName string) {
	if _, err := os.Stat("/sys/module/" + moduleName); err == nil {
		logrus.Info("Module " + moduleName + " was already loaded")
		return
	}

	if err := exec.Command("modprobe", "--", moduleName).Run(); err != nil {
		logrus.Warnf("Failed to load kernel module %v with modprobe", moduleName)
	}
}

// Configure loads required kernel modules and sets sysctls required for other components to
// function properly. The bridge netfilter sysctls are only managed when setBridgeFilter is
// true; see kernelSysctls for details.
func Configure(enableIPv6, setBridgeFilter bool, config *kubeproxyconfig.KubeProxyConntrackConfiguration) {
	loadKernelModule("overlay")
	loadKernelModule("nf_conntrack")
	loadKernelModule("br_netfilter")
	loadKernelModule("iptable_nat")
	loadKernelModule("iptable_filter")
	loadKernelModule("nft-expr-counter")
	loadKernelModule("nfnetlink-subsys-11")
	loadKernelModule("nft-chain-2-nat")
	if enableIPv6 {
		loadKernelModule("ip6table_nat")
		loadKernelModule("ip6table_filter")
	}

	sys := sysctl.New()
	for entry, value := range kernelSysctls(enableIPv6, setBridgeFilter, config) {
		if val, _ := sys.GetSysctl(entry); val != value {
			logrus.Infof("Set sysctl '%v' to %v", entry, value)
			if err := sys.SetSysctl(entry, value); err != nil {
				logrus.Errorf("Failed to set sysctl: %v", err)
			}
		}
	}
}

// kernelSysctls returns the kernel sysctls that Configure should ensure are set.
// The net/bridge/bridge-nf-call-{ip,ip6}tables sysctls are only included when
// setBridgeFilter is true. These are required by kube-proxy and flannel, but on
// nodes that run neither (for example when using an alternative CNI) k3s should
// leave them untouched so that they can be managed by the administrator.
func kernelSysctls(enableIPv6, setBridgeFilter bool, config *kubeproxyconfig.KubeProxyConntrackConfiguration) map[string]int {
	// Kernel is inconsistent about how devconf is configured for
	// new network namespaces between ipv4 and ipv6. Make sure to
	// enable forwarding on all and default for both ipv4 and ipv6.
	sysctls := map[string]int{
		"net/ipv4/conf/all/forwarding":     1,
		"net/ipv4/conf/default/forwarding": 1,
	}

	if setBridgeFilter {
		sysctls["net/bridge/bridge-nf-call-iptables"] = 1
	}

	if enableIPv6 {
		sysctls["net/ipv6/conf/all/forwarding"] = 1
		sysctls["net/ipv6/conf/default/forwarding"] = 1
		sysctls["net/core/devconf_inherit_init_net"] = 1
		if setBridgeFilter {
			sysctls["net/bridge/bridge-nf-call-ip6tables"] = 1
		}
	}

	if conntrackMax := getConntrackMax(config); conntrackMax > 0 {
		sysctls["net/netfilter/nf_conntrack_max"] = conntrackMax
	}
	if config.TCPEstablishedTimeout.Duration > 0 {
		sysctls["net/netfilter/nf_conntrack_tcp_timeout_established"] = int(config.TCPEstablishedTimeout.Duration / time.Second)
	}
	if config.TCPCloseWaitTimeout.Duration > 0 {
		sysctls["net/netfilter/nf_conntrack_tcp_timeout_close_wait"] = int(config.TCPCloseWaitTimeout.Duration / time.Second)
	}

	return sysctls
}

// getConntrackMax is cribbed from kube-proxy, as recent kernels no longer allow non-init namespaces
// to set conntrack-related sysctls.
// ref: https://github.com/kubernetes/kubernetes/blob/v1.21.1/cmd/kube-proxy/app/server.go#L780
// ref: https://github.com/kubernetes-sigs/kind/issues/2240
func getConntrackMax(config *kubeproxyconfig.KubeProxyConntrackConfiguration) int {
	if config.MaxPerCore != nil && *config.MaxPerCore > 0 {
		floor := 0
		if config.Min != nil {
			floor = int(*config.Min)
		}
		scaled := int(*config.MaxPerCore) * detectNumCPU()
		if scaled > floor {
			logrus.Debugf("getConntrackMax: using scaled conntrack-max-per-core")
			return scaled
		}
		logrus.Debugf("getConntrackMax: using conntrack-min")
		return floor
	}
	return 0
}

// detectNumCPU is also cribbed from kube-proxy
func detectNumCPU() int {
	// try get numCPU from /sys firstly due to a known issue (https://github.com/kubernetes/kubernetes/issues/99225)
	_, numCPU, err := machine.GetTopology(sysfs.NewRealSysFs())
	if err != nil || numCPU < 1 {
		return runtime.NumCPU()
	}
	return numCPU
}
