//go:build linux

package syssetup

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeproxyconfig "k8s.io/kubernetes/pkg/proxy/apis/config"
)

func Test_UnitKernelSysctls(t *testing.T) {
	// A zero-valued conntrack config contributes no conntrack sysctls, keeping the
	// assertions below focused on the bridge netfilter gating behavior.
	conntrackConfig := &kubeproxyconfig.KubeProxyConntrackConfiguration{
		TCPEstablishedTimeout: &metav1.Duration{},
		TCPCloseWaitTimeout:   &metav1.Duration{},
	}

	const (
		ipv4Bridge = "net/bridge/bridge-nf-call-iptables"
		ipv6Bridge = "net/bridge/bridge-nf-call-ip6tables"
	)

	tests := []struct {
		name            string
		enableIPv6      bool
		setBridgeFilter bool
		wantIPv4Bridge  bool
		wantIPv6Bridge  bool
	}{
		{name: "bridge filter enabled, ipv4 only", enableIPv6: false, setBridgeFilter: true, wantIPv4Bridge: true, wantIPv6Bridge: false},
		{name: "bridge filter enabled, dual stack", enableIPv6: true, setBridgeFilter: true, wantIPv4Bridge: true, wantIPv6Bridge: true},
		{name: "bridge filter disabled, ipv4 only", enableIPv6: false, setBridgeFilter: false, wantIPv4Bridge: false, wantIPv6Bridge: false},
		{name: "bridge filter disabled, dual stack", enableIPv6: true, setBridgeFilter: false, wantIPv4Bridge: false, wantIPv6Bridge: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sysctls := kernelSysctls(tt.enableIPv6, tt.setBridgeFilter, conntrackConfig)

			if _, ok := sysctls[ipv4Bridge]; ok != tt.wantIPv4Bridge {
				t.Errorf("%s present = %v, want %v", ipv4Bridge, ok, tt.wantIPv4Bridge)
			}
			if _, ok := sysctls[ipv6Bridge]; ok != tt.wantIPv6Bridge {
				t.Errorf("%s present = %v, want %v", ipv6Bridge, ok, tt.wantIPv6Bridge)
			}
			// IPv4 forwarding is always managed, regardless of the bridge filter flag.
			if _, ok := sysctls["net/ipv4/conf/all/forwarding"]; !ok {
				t.Error("net/ipv4/conf/all/forwarding should always be set")
			}
		})
	}
}
