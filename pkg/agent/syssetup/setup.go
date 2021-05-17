// +build !windows

package syssetup

import (
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/google/cadvisor/machine"
	"github.com/google/cadvisor/utils/sysfs"
	"github.com/sirupsen/logrus"
	kubeproxyconfig "k8s.io/kubernetes/pkg/proxy/apis/config"
	"k8s.io/kubernetes/pkg/util/sysctl"
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
// function properly.
func Configure(enableIPv6 bool, config *kubeproxyconfig.KubeProxyConntrackConfiguration) {
	loadKernelModule("overlay")
	loadKernelModule("nf_conntrack")
	loadKernelModule("br_netfilter")
	loadKernelModule("iptable_nat")
	if enableIPv6 {
		loadKernelModule("ip6table_nat")
	}

	// Kernel is inconsistent about how devconf is configured for
	// new network namespaces between ipv4 and ipv6. Make sure to
	// enable forwarding on all and default for both ipv4 and ipv6.
	sysctls := map[string]int{
		"net/ipv4/conf/all/forwarding":       1,
		"net/ipv4/conf/default/forwarding":   1,
		"net/bridge/bridge-nf-call-iptables": 1,
	}

	if enableIPv6 {
		sysctls["net/ipv6/conf/all/forwarding"] = 1
		sysctls["net/ipv6/conf/default/forwarding"] = 1
		sysctls["net/bridge/bridge-nf-call-ip6tables"] = 1
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

	sys := sysctl.New()
	for entry, value := range sysctls {
		if val, _ := sys.GetSysctl(entry); val != value {
			logrus.Infof("Set sysctl '%v' to %v", entry, value)
			if err := sys.SetSysctl(entry, value); err != nil {
				logrus.Errorf("Failed to set sysctl: %v", err)
			}
		}
	}
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
