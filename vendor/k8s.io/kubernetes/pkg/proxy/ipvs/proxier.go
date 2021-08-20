/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ipvs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"k8s.io/klog/v2"
	utilexec "k8s.io/utils/exec"
	utilnet "k8s.io/utils/net"

	v1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apimachinery/pkg/util/wait"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/tools/events"
	"k8s.io/kubernetes/pkg/features"
	"k8s.io/kubernetes/pkg/proxy"
	"k8s.io/kubernetes/pkg/proxy/healthcheck"
	"k8s.io/kubernetes/pkg/proxy/metaproxier"
	"k8s.io/kubernetes/pkg/proxy/metrics"
	utilproxy "k8s.io/kubernetes/pkg/proxy/util"
	proxyutiliptables "k8s.io/kubernetes/pkg/proxy/util/iptables"
	"k8s.io/kubernetes/pkg/util/async"
	"k8s.io/kubernetes/pkg/util/conntrack"
	utilipset "k8s.io/kubernetes/pkg/util/ipset"
	utiliptables "k8s.io/kubernetes/pkg/util/iptables"
	utilipvs "k8s.io/kubernetes/pkg/util/ipvs"
	utilsysctl "k8s.io/kubernetes/pkg/util/sysctl"
)

const (
	// kubeServicesChain is the services portal chain
	kubeServicesChain utiliptables.Chain = "KUBE-SERVICES"

	// KubeFireWallChain is the kubernetes firewall chain.
	KubeFireWallChain utiliptables.Chain = "KUBE-FIREWALL"

	// kubePostroutingChain is the kubernetes postrouting chain
	kubePostroutingChain utiliptables.Chain = "KUBE-POSTROUTING"

	// KubeMarkMasqChain is the mark-for-masquerade chain
	KubeMarkMasqChain utiliptables.Chain = "KUBE-MARK-MASQ"

	// KubeNodePortChain is the kubernetes node port chain
	KubeNodePortChain utiliptables.Chain = "KUBE-NODE-PORT"

	// KubeMarkDropChain is the mark-for-drop chain
	KubeMarkDropChain utiliptables.Chain = "KUBE-MARK-DROP"

	// KubeForwardChain is the kubernetes forward chain
	KubeForwardChain utiliptables.Chain = "KUBE-FORWARD"

	// KubeLoadBalancerChain is the kubernetes chain for loadbalancer type service
	KubeLoadBalancerChain utiliptables.Chain = "KUBE-LOAD-BALANCER"

	// DefaultScheduler is the default ipvs scheduler algorithm - round robin.
	DefaultScheduler = "rr"

	// DefaultDummyDevice is the default dummy interface which ipvs service address will bind to it.
	DefaultDummyDevice = "kube-ipvs0"

	connReuseMinSupportedKernelVersion = "4.1"

	// https://github.com/torvalds/linux/commit/35dfb013149f74c2be1ff9c78f14e6a3cd1539d1
	connReuseFixedKernelVersion = "5.9"
)

// iptablesJumpChain is tables of iptables chains that ipvs proxier used to install iptables or cleanup iptables.
// `to` is the iptables chain we want to operate.
// `from` is the source iptables chain
var iptablesJumpChain = []struct {
	table   utiliptables.Table
	from    utiliptables.Chain
	to      utiliptables.Chain
	comment string
}{
	{utiliptables.TableNAT, utiliptables.ChainOutput, kubeServicesChain, "kubernetes service portals"},
	{utiliptables.TableNAT, utiliptables.ChainPrerouting, kubeServicesChain, "kubernetes service portals"},
	{utiliptables.TableNAT, utiliptables.ChainPostrouting, kubePostroutingChain, "kubernetes postrouting rules"},
	{utiliptables.TableFilter, utiliptables.ChainForward, KubeForwardChain, "kubernetes forwarding rules"},
	{utiliptables.TableFilter, utiliptables.ChainInput, KubeNodePortChain, "kubernetes health check rules"},
}

var iptablesChains = []struct {
	table utiliptables.Table
	chain utiliptables.Chain
}{
	{utiliptables.TableNAT, kubeServicesChain},
	{utiliptables.TableNAT, kubePostroutingChain},
	{utiliptables.TableNAT, KubeFireWallChain},
	{utiliptables.TableNAT, KubeNodePortChain},
	{utiliptables.TableNAT, KubeLoadBalancerChain},
	{utiliptables.TableNAT, KubeMarkMasqChain},
	{utiliptables.TableFilter, KubeForwardChain},
	{utiliptables.TableFilter, KubeNodePortChain},
}

var iptablesEnsureChains = []struct {
	table utiliptables.Table
	chain utiliptables.Chain
}{
	{utiliptables.TableNAT, KubeMarkDropChain},
}

var iptablesCleanupChains = []struct {
	table utiliptables.Table
	chain utiliptables.Chain
}{
	{utiliptables.TableNAT, kubeServicesChain},
	{utiliptables.TableNAT, kubePostroutingChain},
	{utiliptables.TableNAT, KubeFireWallChain},
	{utiliptables.TableNAT, KubeNodePortChain},
	{utiliptables.TableNAT, KubeLoadBalancerChain},
	{utiliptables.TableFilter, KubeForwardChain},
}

// ipsetInfo is all ipset we needed in ipvs proxier
var ipsetInfo = []struct {
	name    string
	setType utilipset.Type
	comment string
}{
	{kubeLoopBackIPSet, utilipset.HashIPPortIP, kubeLoopBackIPSetComment},
	{kubeClusterIPSet, utilipset.HashIPPort, kubeClusterIPSetComment},
	{kubeExternalIPSet, utilipset.HashIPPort, kubeExternalIPSetComment},
	{kubeExternalIPLocalSet, utilipset.HashIPPort, kubeExternalIPLocalSetComment},
	{kubeLoadBalancerSet, utilipset.HashIPPort, kubeLoadBalancerSetComment},
	{kubeLoadbalancerFWSet, utilipset.HashIPPort, kubeLoadbalancerFWSetComment},
	{kubeLoadBalancerLocalSet, utilipset.HashIPPort, kubeLoadBalancerLocalSetComment},
	{kubeLoadBalancerSourceIPSet, utilipset.HashIPPortIP, kubeLoadBalancerSourceIPSetComment},
	{kubeLoadBalancerSourceCIDRSet, utilipset.HashIPPortNet, kubeLoadBalancerSourceCIDRSetComment},
	{kubeNodePortSetTCP, utilipset.BitmapPort, kubeNodePortSetTCPComment},
	{kubeNodePortLocalSetTCP, utilipset.BitmapPort, kubeNodePortLocalSetTCPComment},
	{kubeNodePortSetUDP, utilipset.BitmapPort, kubeNodePortSetUDPComment},
	{kubeNodePortLocalSetUDP, utilipset.BitmapPort, kubeNodePortLocalSetUDPComment},
	{kubeNodePortSetSCTP, utilipset.HashIPPort, kubeNodePortSetSCTPComment},
	{kubeNodePortLocalSetSCTP, utilipset.HashIPPort, kubeNodePortLocalSetSCTPComment},
	{kubeHealthCheckNodePortSet, utilipset.BitmapPort, kubeHealthCheckNodePortSetComment},
}

// ipsetWithIptablesChain is the ipsets list with iptables source chain and the chain jump to
// `iptables -t nat -A <from> -m set --match-set <name> <matchType> -j <to>`
// example: iptables -t nat -A KUBE-SERVICES -m set --match-set KUBE-NODE-PORT-TCP dst -j KUBE-NODE-PORT
// ipsets with other match rules will be created Individually.
// Note: kubeNodePortLocalSetTCP must be prior to kubeNodePortSetTCP, the same for UDP.
var ipsetWithIptablesChain = []struct {
	name          string
	from          string
	to            string
	matchType     string
	protocolMatch string
}{
	{kubeLoopBackIPSet, string(kubePostroutingChain), "MASQUERADE", "dst,dst,src", ""},
	{kubeLoadBalancerSet, string(kubeServicesChain), string(KubeLoadBalancerChain), "dst,dst", ""},
	{kubeLoadbalancerFWSet, string(KubeLoadBalancerChain), string(KubeFireWallChain), "dst,dst", ""},
	{kubeLoadBalancerSourceCIDRSet, string(KubeFireWallChain), "RETURN", "dst,dst,src", ""},
	{kubeLoadBalancerSourceIPSet, string(KubeFireWallChain), "RETURN", "dst,dst,src", ""},
	{kubeLoadBalancerLocalSet, string(KubeLoadBalancerChain), "RETURN", "dst,dst", ""},
	{kubeNodePortLocalSetTCP, string(KubeNodePortChain), "RETURN", "dst", utilipset.ProtocolTCP},
	{kubeNodePortSetTCP, string(KubeNodePortChain), string(KubeMarkMasqChain), "dst", utilipset.ProtocolTCP},
	{kubeNodePortLocalSetUDP, string(KubeNodePortChain), "RETURN", "dst", utilipset.ProtocolUDP},
	{kubeNodePortSetUDP, string(KubeNodePortChain), string(KubeMarkMasqChain), "dst", utilipset.ProtocolUDP},
	{kubeNodePortSetSCTP, string(KubeNodePortChain), string(KubeMarkMasqChain), "dst,dst", utilipset.ProtocolSCTP},
	{kubeNodePortLocalSetSCTP, string(KubeNodePortChain), "RETURN", "dst,dst", utilipset.ProtocolSCTP},
}

// In IPVS proxy mode, the following flags need to be set
const (
	sysctlBridgeCallIPTables      = "net/bridge/bridge-nf-call-iptables"
	sysctlVSConnTrack             = "net/ipv4/vs/conntrack"
	sysctlConnReuse               = "net/ipv4/vs/conn_reuse_mode"
	sysctlExpireNoDestConn        = "net/ipv4/vs/expire_nodest_conn"
	sysctlExpireQuiescentTemplate = "net/ipv4/vs/expire_quiescent_template"
	sysctlForward                 = "net/ipv4/ip_forward"
	sysctlArpIgnore               = "net/ipv4/conf/all/arp_ignore"
	sysctlArpAnnounce             = "net/ipv4/conf/all/arp_announce"
)

// Proxier is an ipvs based proxy for connections between a localhost:lport
// and services that provide the actual backends.
type Proxier struct {
	// the ipfamily on which this proxy is operating on.
	ipFamily v1.IPFamily
	// endpointsChanges and serviceChanges contains all changes to endpoints and
	// services that happened since last syncProxyRules call. For a single object,
	// changes are accumulated, i.e. previous is state from before all of them,
	// current is state after applying all of those.
	endpointsChanges *proxy.EndpointChangeTracker
	serviceChanges   *proxy.ServiceChangeTracker

	mu           sync.Mutex // protects the following fields
	serviceMap   proxy.ServiceMap
	endpointsMap proxy.EndpointsMap
	portsMap     map[utilnet.LocalPort]utilnet.Closeable
	nodeLabels   map[string]string
	// endpointSlicesSynced, and servicesSynced are set to true when
	// corresponding objects are synced after startup. This is used to avoid updating
	// ipvs rules with some partial data after kube-proxy restart.
	endpointSlicesSynced bool
	servicesSynced       bool
	initialized          int32
	syncRunner           *async.BoundedFrequencyRunner // governs calls to syncProxyRules

	// These are effectively const and do not need the mutex to be held.
	syncPeriod    time.Duration
	minSyncPeriod time.Duration
	// Values are CIDR's to exclude when cleaning up IPVS rules.
	excludeCIDRs []*net.IPNet
	// Set to true to set sysctls arp_ignore and arp_announce
	strictARP      bool
	iptables       utiliptables.Interface
	ipvs           utilipvs.Interface
	ipset          utilipset.Interface
	exec           utilexec.Interface
	masqueradeAll  bool
	masqueradeMark string
	localDetector  proxyutiliptables.LocalTrafficDetector
	hostname       string
	nodeIP         net.IP
	portMapper     utilnet.PortOpener
	recorder       events.EventRecorder

	serviceHealthServer healthcheck.ServiceHealthServer
	healthzServer       healthcheck.ProxierHealthUpdater

	ipvsScheduler string
	// Added as a member to the struct to allow injection for testing.
	ipGetter IPGetter
	// The following buffers are used to reuse memory and avoid allocations
	// that are significantly impacting performance.
	iptablesData     *bytes.Buffer
	filterChainsData *bytes.Buffer
	natChains        *bytes.Buffer
	filterChains     *bytes.Buffer
	natRules         *bytes.Buffer
	filterRules      *bytes.Buffer
	// Added as a member to the struct to allow injection for testing.
	netlinkHandle NetLinkHandle
	// ipsetList is the list of ipsets that ipvs proxier used.
	ipsetList map[string]*IPSet
	// Values are as a parameter to select the interfaces which nodeport works.
	nodePortAddresses []string
	// networkInterfacer defines an interface for several net library functions.
	// Inject for test purpose.
	networkInterfacer     utilproxy.NetworkInterfacer
	gracefuldeleteManager *GracefulTerminationManager
}

// IPGetter helps get node network interface IP and IPs binded to the IPVS dummy interface
type IPGetter interface {
	NodeIPs() ([]net.IP, error)
	BindedIPs() (sets.String, error)
}

// realIPGetter is a real NodeIP handler, it implements IPGetter.
type realIPGetter struct {
	// nl is a handle for revoking netlink interface
	nl NetLinkHandle
}

// NodeIPs returns all LOCAL type IP addresses from host which are taken as the Node IPs of NodePort service.
// It will list source IP exists in local route table with `kernel` protocol type, and filter out IPVS proxier
// created dummy device `kube-ipvs0` For example,
// $ ip route show table local type local proto kernel
// 10.0.0.1 dev kube-ipvs0  scope host  src 10.0.0.1
// 10.0.0.10 dev kube-ipvs0  scope host  src 10.0.0.10
// 10.0.0.252 dev kube-ipvs0  scope host  src 10.0.0.252
// 100.106.89.164 dev eth0  scope host  src 100.106.89.164
// 127.0.0.0/8 dev lo  scope host  src 127.0.0.1
// 127.0.0.1 dev lo  scope host  src 127.0.0.1
// 172.17.0.1 dev docker0  scope host  src 172.17.0.1
// 192.168.122.1 dev virbr0  scope host  src 192.168.122.1
// Then filter out dev==kube-ipvs0, and cut the unique src IP fields,
// Node IP set: [100.106.89.164, 172.17.0.1, 192.168.122.1]
// Note that loopback addresses are excluded.
func (r *realIPGetter) NodeIPs() (ips []net.IP, err error) {
	// Pass in empty filter device name for list all LOCAL type addresses.
	nodeAddress, err := r.nl.GetLocalAddresses("", DefaultDummyDevice)
	if err != nil {
		return nil, fmt.Errorf("error listing LOCAL type addresses from host, error: %v", err)
	}
	// translate ip string to IP
	for _, ipStr := range nodeAddress.UnsortedList() {
		a := net.ParseIP(ipStr)
		if a.IsLoopback() {
			continue
		}
		ips = append(ips, a)
	}
	return ips, nil
}

// BindedIPs returns all addresses that are binded to the IPVS dummy interface kube-ipvs0
func (r *realIPGetter) BindedIPs() (sets.String, error) {
	return r.nl.GetLocalAddresses(DefaultDummyDevice, "")
}

// Proxier implements proxy.Provider
var _ proxy.Provider = &Proxier{}

// NewProxier returns a new Proxier given an iptables and ipvs Interface instance.
// Because of the iptables and ipvs logic, it is assumed that there is only a single Proxier active on a machine.
// An error will be returned if it fails to update or acquire the initial lock.
// Once a proxier is created, it will keep iptables and ipvs rules up to date in the background and
// will not terminate if a particular iptables or ipvs call fails.
func NewProxier(ipt utiliptables.Interface,
	ipvs utilipvs.Interface,
	ipset utilipset.Interface,
	sysctl utilsysctl.Interface,
	exec utilexec.Interface,
	syncPeriod time.Duration,
	minSyncPeriod time.Duration,
	excludeCIDRs []string,
	strictARP bool,
	tcpTimeout time.Duration,
	tcpFinTimeout time.Duration,
	udpTimeout time.Duration,
	masqueradeAll bool,
	masqueradeBit int,
	localDetector proxyutiliptables.LocalTrafficDetector,
	hostname string,
	nodeIP net.IP,
	recorder events.EventRecorder,
	healthzServer healthcheck.ProxierHealthUpdater,
	scheduler string,
	nodePortAddresses []string,
	kernelHandler KernelHandler,
) (*Proxier, error) {
	// Proxy needs br_netfilter and bridge-nf-call-iptables=1 when containers
	// are connected to a Linux bridge (but not SDN bridges).  Until most
	// plugins handle this, log when config is missing
	if val, err := sysctl.GetSysctl(sysctlBridgeCallIPTables); err == nil && val != 1 {
		klog.InfoS("Missing br-netfilter module or unset sysctl br-nf-call-iptables; proxy may not work as intended")
	}

	// Set the conntrack sysctl we need for
	if err := utilproxy.EnsureSysctl(sysctl, sysctlVSConnTrack, 1); err != nil {
		return nil, err
	}

	kernelVersionStr, err := kernelHandler.GetKernelVersion()
	if err != nil {
		return nil, fmt.Errorf("error determining kernel version to find required kernel modules for ipvs support: %v", err)
	}
	kernelVersion, err := version.ParseGeneric(kernelVersionStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing kernel version %q: %v", kernelVersionStr, err)
	}
	if kernelVersion.LessThan(version.MustParseGeneric(connReuseMinSupportedKernelVersion)) {
		klog.ErrorS(nil, fmt.Sprintf("can't set sysctl %s, kernel version must be at least %s", sysctlConnReuse, connReuseMinSupportedKernelVersion))
	} else if kernelVersion.AtLeast(version.MustParseGeneric(connReuseFixedKernelVersion)) {
		// https://github.com/kubernetes/kubernetes/issues/93297
		klog.V(2).InfoS("Left as-is", "sysctl", sysctlConnReuse)
	} else {
		// Set the connection reuse mode
		if err := utilproxy.EnsureSysctl(sysctl, sysctlConnReuse, 0); err != nil {
			return nil, err
		}
	}

	// Set the expire_nodest_conn sysctl we need for
	if err := utilproxy.EnsureSysctl(sysctl, sysctlExpireNoDestConn, 1); err != nil {
		return nil, err
	}

	// Set the expire_quiescent_template sysctl we need for
	if err := utilproxy.EnsureSysctl(sysctl, sysctlExpireQuiescentTemplate, 1); err != nil {
		return nil, err
	}

	// Set the ip_forward sysctl we need for
	if err := utilproxy.EnsureSysctl(sysctl, sysctlForward, 1); err != nil {
		return nil, err
	}

	if strictARP {
		// Set the arp_ignore sysctl we need for
		if err := utilproxy.EnsureSysctl(sysctl, sysctlArpIgnore, 1); err != nil {
			return nil, err
		}

		// Set the arp_announce sysctl we need for
		if err := utilproxy.EnsureSysctl(sysctl, sysctlArpAnnounce, 2); err != nil {
			return nil, err
		}
	}

	// Configure IPVS timeouts if any one of the timeout parameters have been set.
	// This is the equivalent to running ipvsadm --set, a value of 0 indicates the
	// current system timeout should be preserved
	if tcpTimeout > 0 || tcpFinTimeout > 0 || udpTimeout > 0 {
		if err := ipvs.ConfigureTimeouts(tcpTimeout, tcpFinTimeout, udpTimeout); err != nil {
			klog.ErrorS(err, "failed to configure IPVS timeouts")
		}
	}

	// Generate the masquerade mark to use for SNAT rules.
	masqueradeValue := 1 << uint(masqueradeBit)
	masqueradeMark := fmt.Sprintf("%#08x", masqueradeValue)

	ipFamily := v1.IPv4Protocol
	if ipt.IsIPv6() {
		ipFamily = v1.IPv6Protocol
	}

	klog.V(2).InfoS("record nodeIP and family", "nodeIP", nodeIP, "family", ipFamily)

	if len(scheduler) == 0 {
		klog.InfoS("IPVS scheduler not specified, use rr by default")
		scheduler = DefaultScheduler
	}

	serviceHealthServer := healthcheck.NewServiceHealthServer(hostname, recorder)

	ipFamilyMap := utilproxy.MapCIDRsByIPFamily(nodePortAddresses)
	nodePortAddresses = ipFamilyMap[ipFamily]
	// Log the IPs not matching the ipFamily
	if ips, ok := ipFamilyMap[utilproxy.OtherIPFamily(ipFamily)]; ok && len(ips) > 0 {
		klog.InfoS("found node IPs of the wrong family", "ipFamily", ipFamily, "ips", strings.Join(ips, ","))
	}

	// excludeCIDRs has been validated before, here we just parse it to IPNet list
	parsedExcludeCIDRs, _ := utilnet.ParseCIDRs(excludeCIDRs)

	proxier := &Proxier{
		ipFamily:              ipFamily,
		portsMap:              make(map[utilnet.LocalPort]utilnet.Closeable),
		serviceMap:            make(proxy.ServiceMap),
		serviceChanges:        proxy.NewServiceChangeTracker(newServiceInfo, ipFamily, recorder, nil),
		endpointsMap:          make(proxy.EndpointsMap),
		endpointsChanges:      proxy.NewEndpointChangeTracker(hostname, nil, ipFamily, recorder, nil),
		syncPeriod:            syncPeriod,
		minSyncPeriod:         minSyncPeriod,
		excludeCIDRs:          parsedExcludeCIDRs,
		iptables:              ipt,
		masqueradeAll:         masqueradeAll,
		masqueradeMark:        masqueradeMark,
		exec:                  exec,
		localDetector:         localDetector,
		hostname:              hostname,
		nodeIP:                nodeIP,
		portMapper:            &utilnet.ListenPortOpener,
		recorder:              recorder,
		serviceHealthServer:   serviceHealthServer,
		healthzServer:         healthzServer,
		ipvs:                  ipvs,
		ipvsScheduler:         scheduler,
		ipGetter:              &realIPGetter{nl: NewNetLinkHandle(ipFamily == v1.IPv6Protocol)},
		iptablesData:          bytes.NewBuffer(nil),
		filterChainsData:      bytes.NewBuffer(nil),
		natChains:             bytes.NewBuffer(nil),
		natRules:              bytes.NewBuffer(nil),
		filterChains:          bytes.NewBuffer(nil),
		filterRules:           bytes.NewBuffer(nil),
		netlinkHandle:         NewNetLinkHandle(ipFamily == v1.IPv6Protocol),
		ipset:                 ipset,
		nodePortAddresses:     nodePortAddresses,
		networkInterfacer:     utilproxy.RealNetwork{},
		gracefuldeleteManager: NewGracefulTerminationManager(ipvs),
	}
	// initialize ipsetList with all sets we needed
	proxier.ipsetList = make(map[string]*IPSet)
	for _, is := range ipsetInfo {
		proxier.ipsetList[is.name] = NewIPSet(ipset, is.name, is.setType, (ipFamily == v1.IPv6Protocol), is.comment)
	}
	burstSyncs := 2
	klog.V(2).InfoS("ipvs sync params", "ipFamily", ipt.Protocol(), "minSyncPeriod", minSyncPeriod, "syncPeriod", syncPeriod, "burstSyncs", burstSyncs)
	proxier.syncRunner = async.NewBoundedFrequencyRunner("sync-runner", proxier.syncProxyRules, minSyncPeriod, syncPeriod, burstSyncs)
	proxier.gracefuldeleteManager.Run()
	return proxier, nil
}

// NewDualStackProxier returns a new Proxier for dual-stack operation
func NewDualStackProxier(
	ipt [2]utiliptables.Interface,
	ipvs utilipvs.Interface,
	ipset utilipset.Interface,
	sysctl utilsysctl.Interface,
	exec utilexec.Interface,
	syncPeriod time.Duration,
	minSyncPeriod time.Duration,
	excludeCIDRs []string,
	strictARP bool,
	tcpTimeout time.Duration,
	tcpFinTimeout time.Duration,
	udpTimeout time.Duration,
	masqueradeAll bool,
	masqueradeBit int,
	localDetectors [2]proxyutiliptables.LocalTrafficDetector,
	hostname string,
	nodeIP [2]net.IP,
	recorder events.EventRecorder,
	healthzServer healthcheck.ProxierHealthUpdater,
	scheduler string,
	nodePortAddresses []string,
	kernelHandler KernelHandler,
) (proxy.Provider, error) {

	safeIpset := newSafeIpset(ipset)

	ipFamilyMap := utilproxy.MapCIDRsByIPFamily(nodePortAddresses)

	// Create an ipv4 instance of the single-stack proxier
	ipv4Proxier, err := NewProxier(ipt[0], ipvs, safeIpset, sysctl,
		exec, syncPeriod, minSyncPeriod, filterCIDRs(false, excludeCIDRs), strictARP,
		tcpTimeout, tcpFinTimeout, udpTimeout, masqueradeAll, masqueradeBit,
		localDetectors[0], hostname, nodeIP[0],
		recorder, healthzServer, scheduler, ipFamilyMap[v1.IPv4Protocol], kernelHandler)
	if err != nil {
		return nil, fmt.Errorf("unable to create ipv4 proxier: %v", err)
	}

	ipv6Proxier, err := NewProxier(ipt[1], ipvs, safeIpset, sysctl,
		exec, syncPeriod, minSyncPeriod, filterCIDRs(true, excludeCIDRs), strictARP,
		tcpTimeout, tcpFinTimeout, udpTimeout, masqueradeAll, masqueradeBit,
		localDetectors[1], hostname, nodeIP[1],
		nil, nil, scheduler, ipFamilyMap[v1.IPv6Protocol], kernelHandler)
	if err != nil {
		return nil, fmt.Errorf("unable to create ipv6 proxier: %v", err)
	}

	// Return a meta-proxier that dispatch calls between the two
	// single-stack proxier instances
	return metaproxier.NewMetaProxier(ipv4Proxier, ipv6Proxier), nil
}

func filterCIDRs(wantIPv6 bool, cidrs []string) []string {
	var filteredCIDRs []string
	for _, cidr := range cidrs {
		if utilnet.IsIPv6CIDRString(cidr) == wantIPv6 {
			filteredCIDRs = append(filteredCIDRs, cidr)
		}
	}
	return filteredCIDRs
}

// internal struct for string service information
type serviceInfo struct {
	*proxy.BaseServiceInfo
	// The following fields are computed and stored for performance reasons.
	serviceNameString string
}

// returns a new proxy.ServicePort which abstracts a serviceInfo
func newServiceInfo(port *v1.ServicePort, service *v1.Service, baseInfo *proxy.BaseServiceInfo) proxy.ServicePort {
	info := &serviceInfo{BaseServiceInfo: baseInfo}

	// Store the following for performance reasons.
	svcName := types.NamespacedName{Namespace: service.Namespace, Name: service.Name}
	svcPortName := proxy.ServicePortName{NamespacedName: svcName, Port: port.Name}
	info.serviceNameString = svcPortName.String()

	return info
}

// KernelHandler can handle the current installed kernel modules.
type KernelHandler interface {
	GetModules() ([]string, error)
	GetKernelVersion() (string, error)
}

// LinuxKernelHandler implements KernelHandler interface.
type LinuxKernelHandler struct {
	executor utilexec.Interface
}

// NewLinuxKernelHandler initializes LinuxKernelHandler with exec.
func NewLinuxKernelHandler() *LinuxKernelHandler {
	return &LinuxKernelHandler{
		executor: utilexec.New(),
	}
}

// GetModules returns all installed kernel modules.
func (handle *LinuxKernelHandler) GetModules() ([]string, error) {
	// Check whether IPVS required kernel modules are built-in
	kernelVersionStr, err := handle.GetKernelVersion()
	if err != nil {
		return nil, err
	}
	kernelVersion, err := version.ParseGeneric(kernelVersionStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing kernel version %q: %v", kernelVersionStr, err)
	}
	ipvsModules := utilipvs.GetRequiredIPVSModules(kernelVersion)

	var bmods, lmods []string

	// Find out loaded kernel modules. If this is a full static kernel it will try to verify if the module is compiled using /boot/config-KERNELVERSION
	modulesFile, err := os.Open("/proc/modules")
	if err == os.ErrNotExist {
		klog.ErrorS(err, "Failed to read file /proc/modules. Assuming this is a kernel without loadable modules support enabled")
		kernelConfigFile := fmt.Sprintf("/boot/config-%s", kernelVersionStr)
		kConfig, err := ioutil.ReadFile(kernelConfigFile)
		if err != nil {
			return nil, fmt.Errorf("Failed to read Kernel Config file %s with error %v", kernelConfigFile, err)
		}
		for _, module := range ipvsModules {
			if match, _ := regexp.Match("CONFIG_"+strings.ToUpper(module)+"=y", kConfig); match {
				bmods = append(bmods, module)
			}
		}
		return bmods, nil
	}
	if err != nil {
		return nil, fmt.Errorf("Failed to read file /proc/modules with error %v", err)
	}
	defer modulesFile.Close()

	mods, err := getFirstColumn(modulesFile)
	if err != nil {
		return nil, fmt.Errorf("failed to find loaded kernel modules: %v", err)
	}

	builtinModsFilePath := fmt.Sprintf("/lib/modules/%s/modules.builtin", kernelVersionStr)
	b, err := ioutil.ReadFile(builtinModsFilePath)
	if err != nil {
		klog.ErrorS(err, "Failed to read builtin modules file. You can ignore this message when kube-proxy is running inside container without mounting /lib/modules", "filePath", builtinModsFilePath)
	}

	for _, module := range ipvsModules {
		if match, _ := regexp.Match(module+".ko", b); match {
			bmods = append(bmods, module)
		} else {
			// Try to load the required IPVS kernel modules if not built in
			err := handle.executor.Command("modprobe", "--", module).Run()
			if err != nil {
				klog.InfoS("Failed to load kernel module with modprobe. "+
					"You can ignore this message when kube-proxy is running inside container without mounting /lib/modules", "moduleName", module)
			} else {
				lmods = append(lmods, module)
			}
		}
	}

	mods = append(mods, bmods...)
	mods = append(mods, lmods...)
	return mods, nil
}

// getFirstColumn reads all the content from r into memory and return a
// slice which consists of the first word from each line.
func getFirstColumn(r io.Reader) ([]string, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(b), "\n")
	words := make([]string, 0, len(lines))
	for i := range lines {
		fields := strings.Fields(lines[i])
		if len(fields) > 0 {
			words = append(words, fields[0])
		}
	}
	return words, nil
}

// GetKernelVersion returns currently running kernel version.
func (handle *LinuxKernelHandler) GetKernelVersion() (string, error) {
	kernelVersionFile := "/proc/sys/kernel/osrelease"
	fileContent, err := ioutil.ReadFile(kernelVersionFile)
	if err != nil {
		return "", fmt.Errorf("error reading osrelease file %q: %v", kernelVersionFile, err)
	}

	return strings.TrimSpace(string(fileContent)), nil
}

// CanUseIPVSProxier returns true if we can use the ipvs Proxier.
// This is determined by checking if all the required kernel modules can be loaded. It may
// return an error if it fails to get the kernel modules information without error, in which
// case it will also return false.
func CanUseIPVSProxier(handle KernelHandler, ipsetver IPSetVersioner, scheduler string) (bool, error) {
	mods, err := handle.GetModules()
	if err != nil {
		return false, fmt.Errorf("error getting installed ipvs required kernel modules: %v", err)
	}
	loadModules := sets.NewString()
	loadModules.Insert(mods...)

	kernelVersionStr, err := handle.GetKernelVersion()
	if err != nil {
		return false, fmt.Errorf("error determining kernel version to find required kernel modules for ipvs support: %v", err)
	}
	kernelVersion, err := version.ParseGeneric(kernelVersionStr)
	if err != nil {
		return false, fmt.Errorf("error parsing kernel version %q: %v", kernelVersionStr, err)
	}
	mods = utilipvs.GetRequiredIPVSModules(kernelVersion)
	wantModules := sets.NewString()
	// We check for the existence of the scheduler mod and will trigger a missingMods error if not found
	if scheduler == "" {
		scheduler = DefaultScheduler
	}
	schedulerMod := "ip_vs_" + scheduler
	mods = append(mods, schedulerMod)
	wantModules.Insert(mods...)

	modules := wantModules.Difference(loadModules).UnsortedList()
	var missingMods []string
	ConntrackiMissingCounter := 0
	for _, mod := range modules {
		if strings.Contains(mod, "nf_conntrack") {
			ConntrackiMissingCounter++
		} else {
			missingMods = append(missingMods, mod)
		}
	}
	if ConntrackiMissingCounter == 2 {
		missingMods = append(missingMods, "nf_conntrack_ipv4(or nf_conntrack for Linux kernel 4.19 and later)")
	}

	if len(missingMods) != 0 {
		return false, fmt.Errorf("IPVS proxier will not be used because the following required kernel modules are not loaded: %v", missingMods)
	}

	// Check ipset version
	versionString, err := ipsetver.GetVersion()
	if err != nil {
		return false, fmt.Errorf("error getting ipset version, error: %v", err)
	}
	if !checkMinVersion(versionString) {
		return false, fmt.Errorf("ipset version: %s is less than min required version: %s", versionString, MinIPSetCheckVersion)
	}
	return true, nil
}

// CleanupIptablesLeftovers removes all iptables rules and chains created by the Proxier
// It returns true if an error was encountered. Errors are logged.
func cleanupIptablesLeftovers(ipt utiliptables.Interface) (encounteredError bool) {
	// Unlink the iptables chains created by ipvs Proxier
	for _, jc := range iptablesJumpChain {
		args := []string{
			"-m", "comment", "--comment", jc.comment,
			"-j", string(jc.to),
		}
		if err := ipt.DeleteRule(jc.table, jc.from, args...); err != nil {
			if !utiliptables.IsNotFoundError(err) {
				klog.ErrorS(err, "Error removing iptables rules in ipvs proxier")
				encounteredError = true
			}
		}
	}

	// Flush and remove all of our chains. Flushing all chains before removing them also removes all links between chains first.
	for _, ch := range iptablesCleanupChains {
		if err := ipt.FlushChain(ch.table, ch.chain); err != nil {
			if !utiliptables.IsNotFoundError(err) {
				klog.ErrorS(err, "Error removing iptables rules in ipvs proxier")
				encounteredError = true
			}
		}
	}

	// Remove all of our chains.
	for _, ch := range iptablesCleanupChains {
		if err := ipt.DeleteChain(ch.table, ch.chain); err != nil {
			if !utiliptables.IsNotFoundError(err) {
				klog.ErrorS(err, "Error removing iptables rules in ipvs proxier")
				encounteredError = true
			}
		}
	}

	return encounteredError
}

// CleanupLeftovers clean up all ipvs and iptables rules created by ipvs Proxier.
func CleanupLeftovers(ipvs utilipvs.Interface, ipt utiliptables.Interface, ipset utilipset.Interface) (encounteredError bool) {
	// Clear all ipvs rules
	if ipvs != nil {
		err := ipvs.Flush()
		if err != nil {
			klog.ErrorS(err, "Error flushing IPVS rules")
			encounteredError = true
		}
	}
	// Delete dummy interface created by ipvs Proxier.
	nl := NewNetLinkHandle(false)
	err := nl.DeleteDummyDevice(DefaultDummyDevice)
	if err != nil {
		klog.ErrorS(err, "Error deleting dummy device created by IPVS proxier", "device", DefaultDummyDevice)
		encounteredError = true
	}
	// Clear iptables created by ipvs Proxier.
	encounteredError = cleanupIptablesLeftovers(ipt) || encounteredError
	// Destroy ip sets created by ipvs Proxier.  We should call it after cleaning up
	// iptables since we can NOT delete ip set which is still referenced by iptables.
	for _, set := range ipsetInfo {
		err = ipset.DestroySet(set.name)
		if err != nil {
			if !utilipset.IsNotFoundError(err) {
				klog.ErrorS(err, "Error removing ipset", "ipset", set.name)
				encounteredError = true
			}
		}
	}
	return encounteredError
}

// Sync is called to synchronize the proxier state to iptables and ipvs as soon as possible.
func (proxier *Proxier) Sync() {
	if proxier.healthzServer != nil {
		proxier.healthzServer.QueuedUpdate()
	}
	metrics.SyncProxyRulesLastQueuedTimestamp.SetToCurrentTime()
	proxier.syncRunner.Run()
}

// SyncLoop runs periodic work.  This is expected to run as a goroutine or as the main loop of the app.  It does not return.
func (proxier *Proxier) SyncLoop() {
	// Update healthz timestamp at beginning in case Sync() never succeeds.
	if proxier.healthzServer != nil {
		proxier.healthzServer.Updated()
	}
	// synthesize "last change queued" time as the informers are syncing.
	metrics.SyncProxyRulesLastQueuedTimestamp.SetToCurrentTime()
	proxier.syncRunner.Loop(wait.NeverStop)
}

func (proxier *Proxier) setInitialized(value bool) {
	var initialized int32
	if value {
		initialized = 1
	}
	atomic.StoreInt32(&proxier.initialized, initialized)
}

func (proxier *Proxier) isInitialized() bool {
	return atomic.LoadInt32(&proxier.initialized) > 0
}

// OnServiceAdd is called whenever creation of new service object is observed.
func (proxier *Proxier) OnServiceAdd(service *v1.Service) {
	proxier.OnServiceUpdate(nil, service)
}

// OnServiceUpdate is called whenever modification of an existing service object is observed.
func (proxier *Proxier) OnServiceUpdate(oldService, service *v1.Service) {
	if proxier.serviceChanges.Update(oldService, service) && proxier.isInitialized() {
		proxier.Sync()
	}
}

// OnServiceDelete is called whenever deletion of an existing service object is observed.
func (proxier *Proxier) OnServiceDelete(service *v1.Service) {
	proxier.OnServiceUpdate(service, nil)
}

// OnServiceSynced is called once all the initial event handlers were called and the state is fully propagated to local cache.
func (proxier *Proxier) OnServiceSynced() {
	proxier.mu.Lock()
	proxier.servicesSynced = true
	proxier.setInitialized(proxier.endpointSlicesSynced)
	proxier.mu.Unlock()

	// Sync unconditionally - this is called once per lifetime.
	proxier.syncProxyRules()
}

// The following methods exist to implement the Proxier interface however
// ipvs proxier only uses EndpointSlices so the following are noops

// OnEndpointsAdd is called whenever creation of new endpoints object is observed.
func (proxier *Proxier) OnEndpointsAdd(endpoints *v1.Endpoints) {}

// OnEndpointsUpdate is called whenever modification of an existing endpoints object is observed.
func (proxier *Proxier) OnEndpointsUpdate(oldEndpoints, endpoints *v1.Endpoints) {}

// OnEndpointsDelete is called whenever deletion of an existing endpoints object is observed.
func (proxier *Proxier) OnEndpointsDelete(endpoints *v1.Endpoints) {}

// OnEndpointsSynced is called once all the initial event handlers were called and the state is fully propagated to local cache.
func (proxier *Proxier) OnEndpointsSynced() {}

// OnEndpointSliceAdd is called whenever creation of a new endpoint slice object
// is observed.
func (proxier *Proxier) OnEndpointSliceAdd(endpointSlice *discovery.EndpointSlice) {
	if proxier.endpointsChanges.EndpointSliceUpdate(endpointSlice, false) && proxier.isInitialized() {
		proxier.Sync()
	}
}

// OnEndpointSliceUpdate is called whenever modification of an existing endpoint
// slice object is observed.
func (proxier *Proxier) OnEndpointSliceUpdate(_, endpointSlice *discovery.EndpointSlice) {
	if proxier.endpointsChanges.EndpointSliceUpdate(endpointSlice, false) && proxier.isInitialized() {
		proxier.Sync()
	}
}

// OnEndpointSliceDelete is called whenever deletion of an existing endpoint slice
// object is observed.
func (proxier *Proxier) OnEndpointSliceDelete(endpointSlice *discovery.EndpointSlice) {
	if proxier.endpointsChanges.EndpointSliceUpdate(endpointSlice, true) && proxier.isInitialized() {
		proxier.Sync()
	}
}

// OnEndpointSlicesSynced is called once all the initial event handlers were
// called and the state is fully propagated to local cache.
func (proxier *Proxier) OnEndpointSlicesSynced() {
	proxier.mu.Lock()
	proxier.endpointSlicesSynced = true
	proxier.setInitialized(proxier.servicesSynced)
	proxier.mu.Unlock()

	// Sync unconditionally - this is called once per lifetime.
	proxier.syncProxyRules()
}

// OnNodeAdd is called whenever creation of new node object
// is observed.
func (proxier *Proxier) OnNodeAdd(node *v1.Node) {
	if node.Name != proxier.hostname {
		klog.ErrorS(nil, "Received a watch event for a node that doesn't match the current node", "eventNode", node.Name, "currentNode", proxier.hostname)
		return
	}

	if reflect.DeepEqual(proxier.nodeLabels, node.Labels) {
		return
	}

	proxier.mu.Lock()
	proxier.nodeLabels = map[string]string{}
	for k, v := range node.Labels {
		proxier.nodeLabels[k] = v
	}
	proxier.mu.Unlock()
	klog.V(4).InfoS("Updated proxier node labels", "labels", node.Labels)

	proxier.syncProxyRules()
}

// OnNodeUpdate is called whenever modification of an existing
// node object is observed.
func (proxier *Proxier) OnNodeUpdate(oldNode, node *v1.Node) {
	if node.Name != proxier.hostname {
		klog.ErrorS(nil, "Received a watch event for a node that doesn't match the current node", "eventNode", node.Name, "currentNode", proxier.hostname)
		return
	}

	if reflect.DeepEqual(proxier.nodeLabels, node.Labels) {
		return
	}

	proxier.mu.Lock()
	proxier.nodeLabels = map[string]string{}
	for k, v := range node.Labels {
		proxier.nodeLabels[k] = v
	}
	proxier.mu.Unlock()
	klog.V(4).InfoS("Updated proxier node labels", "labels", node.Labels)

	proxier.syncProxyRules()
}

// OnNodeDelete is called whenever deletion of an existing node
// object is observed.
func (proxier *Proxier) OnNodeDelete(node *v1.Node) {
	if node.Name != proxier.hostname {
		klog.ErrorS(nil, "Received a watch event for a node that doesn't match the current node", "eventNode", node.Name, "currentNode", proxier.hostname)
		return
	}
	proxier.mu.Lock()
	proxier.nodeLabels = nil
	proxier.mu.Unlock()

	proxier.syncProxyRules()
}

// OnNodeSynced is called once all the initial event handlers were
// called and the state is fully propagated to local cache.
func (proxier *Proxier) OnNodeSynced() {
}

// This is where all of the ipvs calls happen.
// assumes proxier.mu is held
func (proxier *Proxier) syncProxyRules() {
	proxier.mu.Lock()
	defer proxier.mu.Unlock()

	// don't sync rules till we've received services and endpoints
	if !proxier.isInitialized() {
		klog.V(2).InfoS("Not syncing ipvs rules until Services and Endpoints have been received from master")
		return
	}

	// Keep track of how long syncs take.
	start := time.Now()
	defer func() {
		metrics.SyncProxyRulesLatency.Observe(metrics.SinceInSeconds(start))
		klog.V(4).InfoS("syncProxyRules complete", "elapsed", time.Since(start))
	}()

	// We assume that if this was called, we really want to sync them,
	// even if nothing changed in the meantime. In other words, callers are
	// responsible for detecting no-op changes and not calling this function.
	serviceUpdateResult := proxier.serviceMap.Update(proxier.serviceChanges)
	endpointUpdateResult := proxier.endpointsMap.Update(proxier.endpointsChanges)

	staleServices := serviceUpdateResult.UDPStaleClusterIP
	// merge stale services gathered from updateEndpointsMap
	for _, svcPortName := range endpointUpdateResult.StaleServiceNames {
		if svcInfo, ok := proxier.serviceMap[svcPortName]; ok && svcInfo != nil && conntrack.IsClearConntrackNeeded(svcInfo.Protocol()) {
			klog.V(2).InfoS("Stale service", "protocol", strings.ToLower(string(svcInfo.Protocol())), "svcPortName", svcPortName.String(), "clusterIP", svcInfo.ClusterIP().String())
			staleServices.Insert(svcInfo.ClusterIP().String())
			for _, extIP := range svcInfo.ExternalIPStrings() {
				staleServices.Insert(extIP)
			}
		}
	}

	klog.V(3).InfoS("Syncing ipvs Proxier rules")

	// Begin install iptables

	// Reset all buffers used later.
	// This is to avoid memory reallocations and thus improve performance.
	proxier.natChains.Reset()
	proxier.natRules.Reset()
	proxier.filterChains.Reset()
	proxier.filterRules.Reset()

	// Write table headers.
	utilproxy.WriteLine(proxier.filterChains, "*filter")
	utilproxy.WriteLine(proxier.natChains, "*nat")

	proxier.createAndLinkKubeChain()

	// make sure dummy interface exists in the system where ipvs Proxier will bind service address on it
	_, err := proxier.netlinkHandle.EnsureDummyDevice(DefaultDummyDevice)
	if err != nil {
		klog.ErrorS(err, "Failed to create dummy interface", "interface", DefaultDummyDevice)
		return
	}

	// make sure ip sets exists in the system.
	for _, set := range proxier.ipsetList {
		if err := ensureIPSet(set); err != nil {
			return
		}
		set.resetEntries()
	}

	// Accumulate the set of local ports that we will be holding open once this update is complete
	replacementPortsMap := map[utilnet.LocalPort]utilnet.Closeable{}
	// activeIPVSServices represents IPVS service successfully created in this round of sync
	activeIPVSServices := map[string]bool{}
	// currentIPVSServices represent IPVS services listed from the system
	currentIPVSServices := make(map[string]*utilipvs.VirtualServer)
	// activeBindAddrs represents ip address successfully bind to DefaultDummyDevice in this round of sync
	activeBindAddrs := map[string]bool{}

	bindedAddresses, err := proxier.ipGetter.BindedIPs()
	if err != nil {
		klog.ErrorS(err, "error listing addresses binded to dummy interface")
	}

	hasNodePort := false
	for _, svc := range proxier.serviceMap {
		svcInfo, ok := svc.(*serviceInfo)
		if ok && svcInfo.NodePort() != 0 {
			hasNodePort = true
			break
		}
	}

	// Both nodeAddresses and nodeIPs can be reused for all nodePort services
	// and only need to be computed if we have at least one nodePort service.
	var (
		// List of node addresses to listen on if a nodePort is set.
		nodeAddresses []string
		// List of node IP addresses to be used as IPVS services if nodePort is set.
		nodeIPs []net.IP
	)

	if hasNodePort {
		nodeAddrSet, err := utilproxy.GetNodeAddresses(proxier.nodePortAddresses, proxier.networkInterfacer)
		if err != nil {
			klog.ErrorS(err, "Failed to get node ip address matching nodeport cidr")
		} else {
			nodeAddresses = nodeAddrSet.List()
			for _, address := range nodeAddresses {
				a := net.ParseIP(address)
				if a.IsLoopback() {
					continue
				}
				if utilproxy.IsZeroCIDR(address) {
					nodeIPs, err = proxier.ipGetter.NodeIPs()
					if err != nil {
						klog.ErrorS(err, "Failed to list all node IPs from host")
					}
					break
				}
				nodeIPs = append(nodeIPs, a)
			}
		}
	}

	// filter node IPs by proxier ipfamily
	idx := 0
	for _, nodeIP := range nodeIPs {
		if (proxier.ipFamily == v1.IPv6Protocol) == utilnet.IsIPv6(nodeIP) {
			nodeIPs[idx] = nodeIP
			idx++
		}
	}
	// reset slice to filtered entries
	nodeIPs = nodeIPs[:idx]

	localAddrSet := utilproxy.GetLocalAddrSet()

	// Build IPVS rules for each service.
	for svcName, svc := range proxier.serviceMap {
		svcInfo, ok := svc.(*serviceInfo)
		if !ok {
			klog.ErrorS(nil, "Failed to cast serviceInfo", "svcName", svcName.String())
			continue
		}
		isIPv6 := utilnet.IsIPv6(svcInfo.ClusterIP())
		localPortIPFamily := utilnet.IPv4
		if isIPv6 {
			localPortIPFamily = utilnet.IPv6
		}
		protocol := strings.ToLower(string(svcInfo.Protocol()))
		// Precompute svcNameString; with many services the many calls
		// to ServicePortName.String() show up in CPU profiles.
		svcNameString := svcName.String()

		// Handle traffic that loops back to the originator with SNAT.
		for _, e := range proxier.endpointsMap[svcName] {
			ep, ok := e.(*proxy.BaseEndpointInfo)
			if !ok {
				klog.ErrorS(nil, "Failed to cast BaseEndpointInfo", "endpoint", e.String())
				continue
			}
			if !ep.IsLocal {
				continue
			}
			epIP := ep.IP()
			epPort, err := ep.Port()
			// Error parsing this endpoint has been logged. Skip to next endpoint.
			if epIP == "" || err != nil {
				continue
			}
			entry := &utilipset.Entry{
				IP:       epIP,
				Port:     epPort,
				Protocol: protocol,
				IP2:      epIP,
				SetType:  utilipset.HashIPPortIP,
			}
			if valid := proxier.ipsetList[kubeLoopBackIPSet].validateEntry(entry); !valid {
				klog.ErrorS(nil, "error adding entry to ipset", "entry", entry.String(), "ipset", proxier.ipsetList[kubeLoopBackIPSet].Name)
				continue
			}
			proxier.ipsetList[kubeLoopBackIPSet].activeEntries.Insert(entry.String())
		}

		// Capture the clusterIP.
		// ipset call
		entry := &utilipset.Entry{
			IP:       svcInfo.ClusterIP().String(),
			Port:     svcInfo.Port(),
			Protocol: protocol,
			SetType:  utilipset.HashIPPort,
		}
		// add service Cluster IP:Port to kubeServiceAccess ip set for the purpose of solving hairpin.
		// proxier.kubeServiceAccessSet.activeEntries.Insert(entry.String())
		if valid := proxier.ipsetList[kubeClusterIPSet].validateEntry(entry); !valid {
			klog.ErrorS(nil, "error adding entry to ipset", "entry", entry.String(), "ipset", proxier.ipsetList[kubeClusterIPSet].Name)
			continue
		}
		proxier.ipsetList[kubeClusterIPSet].activeEntries.Insert(entry.String())
		// ipvs call
		serv := &utilipvs.VirtualServer{
			Address:   svcInfo.ClusterIP(),
			Port:      uint16(svcInfo.Port()),
			Protocol:  string(svcInfo.Protocol()),
			Scheduler: proxier.ipvsScheduler,
		}
		// Set session affinity flag and timeout for IPVS service
		if svcInfo.SessionAffinityType() == v1.ServiceAffinityClientIP {
			serv.Flags |= utilipvs.FlagPersistent
			serv.Timeout = uint32(svcInfo.StickyMaxAgeSeconds())
		}
		// We need to bind ClusterIP to dummy interface, so set `bindAddr` parameter to `true` in syncService()
		if err := proxier.syncService(svcNameString, serv, true, bindedAddresses); err == nil {
			activeIPVSServices[serv.String()] = true
			activeBindAddrs[serv.Address.String()] = true
			// ExternalTrafficPolicy only works for NodePort and external LB traffic, does not affect ClusterIP
			// So we still need clusterIP rules in onlyNodeLocalEndpoints mode.
			internalNodeLocal := false
			if utilfeature.DefaultFeatureGate.Enabled(features.ServiceInternalTrafficPolicy) && svcInfo.NodeLocalInternal() {
				internalNodeLocal = true
			}
			if err := proxier.syncEndpoint(svcName, internalNodeLocal, serv); err != nil {
				klog.ErrorS(err, "Failed to sync endpoint for service", "service", serv.String())
			}
		} else {
			klog.ErrorS(err, "Failed to sync service", "service", serv.String())
		}

		// Capture externalIPs.
		for _, externalIP := range svcInfo.ExternalIPStrings() {
			// If the "external" IP happens to be an IP that is local to this
			// machine, hold the local port open so no other process can open it
			// (because the socket might open but it would never work).
			if (svcInfo.Protocol() != v1.ProtocolSCTP) && localAddrSet.Has(net.ParseIP(externalIP)) {
				// We do not start listening on SCTP ports, according to our agreement in the SCTP support KEP
				lp := utilnet.LocalPort{
					Description: "externalIP for " + svcNameString,
					IP:          externalIP,
					IPFamily:    localPortIPFamily,
					Port:        svcInfo.Port(),
					Protocol:    utilnet.Protocol(svcInfo.Protocol()),
				}
				if proxier.portsMap[lp] != nil {
					klog.V(4).InfoS("Port was open before and is still needed", "port", lp.String())
					replacementPortsMap[lp] = proxier.portsMap[lp]
				} else {
					socket, err := proxier.portMapper.OpenLocalPort(&lp)
					if err != nil {
						msg := fmt.Sprintf("can't open port %s, skipping it", lp.String())

						proxier.recorder.Eventf(
							&v1.ObjectReference{
								Kind:      "Node",
								Name:      proxier.hostname,
								UID:       types.UID(proxier.hostname),
								Namespace: "",
							}, nil, v1.EventTypeWarning, err.Error(), "SyncProxyRules", msg)
						klog.ErrorS(err, "can't open port, skipping it", "port", lp.String())
						continue
					}
					klog.V(2).InfoS("Opened local port", "port", lp.String())
					replacementPortsMap[lp] = socket
				}
			} // We're holding the port, so it's OK to install IPVS rules.

			// ipset call
			entry := &utilipset.Entry{
				IP:       externalIP,
				Port:     svcInfo.Port(),
				Protocol: protocol,
				SetType:  utilipset.HashIPPort,
			}

			if svcInfo.NodeLocalExternal() {
				if valid := proxier.ipsetList[kubeExternalIPLocalSet].validateEntry(entry); !valid {
					klog.ErrorS(nil, "error adding entry to ipset", "entry", entry.String(), "ipset", proxier.ipsetList[kubeExternalIPLocalSet].Name)
					continue
				}
				proxier.ipsetList[kubeExternalIPLocalSet].activeEntries.Insert(entry.String())
			} else {
				// We have to SNAT packets to external IPs.
				if valid := proxier.ipsetList[kubeExternalIPSet].validateEntry(entry); !valid {
					klog.ErrorS(nil, "error adding entry to ipset", "entry", entry.String(), "ipset", proxier.ipsetList[kubeExternalIPSet].Name)
					continue
				}
				proxier.ipsetList[kubeExternalIPSet].activeEntries.Insert(entry.String())
			}

			// ipvs call
			serv := &utilipvs.VirtualServer{
				Address:   net.ParseIP(externalIP),
				Port:      uint16(svcInfo.Port()),
				Protocol:  string(svcInfo.Protocol()),
				Scheduler: proxier.ipvsScheduler,
			}
			if svcInfo.SessionAffinityType() == v1.ServiceAffinityClientIP {
				serv.Flags |= utilipvs.FlagPersistent
				serv.Timeout = uint32(svcInfo.StickyMaxAgeSeconds())
			}
			if err := proxier.syncService(svcNameString, serv, true, bindedAddresses); err == nil {
				activeIPVSServices[serv.String()] = true
				activeBindAddrs[serv.Address.String()] = true

				if err := proxier.syncEndpoint(svcName, svcInfo.NodeLocalExternal(), serv); err != nil {
					klog.ErrorS(err, "Failed to sync endpoint for service", "service", serv.String())
				}
			} else {
				klog.ErrorS(err, "Failed to sync service", "service", serv.String())
			}
		}

		// Capture load-balancer ingress.
		for _, ingress := range svcInfo.LoadBalancerIPStrings() {
			if ingress != "" {
				// ipset call
				entry = &utilipset.Entry{
					IP:       ingress,
					Port:     svcInfo.Port(),
					Protocol: protocol,
					SetType:  utilipset.HashIPPort,
				}
				// add service load balancer ingressIP:Port to kubeServiceAccess ip set for the purpose of solving hairpin.
				// proxier.kubeServiceAccessSet.activeEntries.Insert(entry.String())
				// If we are proxying globally, we need to masquerade in case we cross nodes.
				// If we are proxying only locally, we can retain the source IP.
				if valid := proxier.ipsetList[kubeLoadBalancerSet].validateEntry(entry); !valid {
					klog.ErrorS(nil, "error adding entry to ipset", "entry", entry.String(), "ipset", proxier.ipsetList[kubeLoadBalancerSet].Name)
					continue
				}
				proxier.ipsetList[kubeLoadBalancerSet].activeEntries.Insert(entry.String())
				// insert loadbalancer entry to lbIngressLocalSet if service externaltrafficpolicy=local
				if svcInfo.NodeLocalExternal() {
					if valid := proxier.ipsetList[kubeLoadBalancerLocalSet].validateEntry(entry); !valid {
						klog.ErrorS(nil, "error adding entry to ipset", "entry", entry.String(), "ipset", proxier.ipsetList[kubeLoadBalancerLocalSet].Name)
						continue
					}
					proxier.ipsetList[kubeLoadBalancerLocalSet].activeEntries.Insert(entry.String())
				}
				if len(svcInfo.LoadBalancerSourceRanges()) != 0 {
					// The service firewall rules are created based on ServiceSpec.loadBalancerSourceRanges field.
					// This currently works for loadbalancers that preserves source ips.
					// For loadbalancers which direct traffic to service NodePort, the firewall rules will not apply.
					if valid := proxier.ipsetList[kubeLoadbalancerFWSet].validateEntry(entry); !valid {
						klog.ErrorS(nil, "error adding entry to ipset", "entry", entry.String(), "ipset", proxier.ipsetList[kubeLoadbalancerFWSet].Name)
						continue
					}
					proxier.ipsetList[kubeLoadbalancerFWSet].activeEntries.Insert(entry.String())
					allowFromNode := false
					for _, src := range svcInfo.LoadBalancerSourceRanges() {
						// ipset call
						entry = &utilipset.Entry{
							IP:       ingress,
							Port:     svcInfo.Port(),
							Protocol: protocol,
							Net:      src,
							SetType:  utilipset.HashIPPortNet,
						}
						// enumerate all white list source cidr
						if valid := proxier.ipsetList[kubeLoadBalancerSourceCIDRSet].validateEntry(entry); !valid {
							klog.ErrorS(nil, "error adding entry to ipset", "entry", entry.String(), "ipset", proxier.ipsetList[kubeLoadBalancerSourceCIDRSet].Name)
							continue
						}
						proxier.ipsetList[kubeLoadBalancerSourceCIDRSet].activeEntries.Insert(entry.String())

						// ignore error because it has been validated
						_, cidr, _ := net.ParseCIDR(src)
						if cidr.Contains(proxier.nodeIP) {
							allowFromNode = true
						}
					}
					// generally, ip route rule was added to intercept request to loadbalancer vip from the
					// loadbalancer's backend hosts. In this case, request will not hit the loadbalancer but loop back directly.
					// Need to add the following rule to allow request on host.
					if allowFromNode {
						entry = &utilipset.Entry{
							IP:       ingress,
							Port:     svcInfo.Port(),
							Protocol: protocol,
							IP2:      ingress,
							SetType:  utilipset.HashIPPortIP,
						}
						// enumerate all white list source ip
						if valid := proxier.ipsetList[kubeLoadBalancerSourceIPSet].validateEntry(entry); !valid {
							klog.ErrorS(nil, "error adding entry to ipset", "entry", entry.String(), "ipset", proxier.ipsetList[kubeLoadBalancerSourceIPSet].Name)
							continue
						}
						proxier.ipsetList[kubeLoadBalancerSourceIPSet].activeEntries.Insert(entry.String())
					}
				}

				// ipvs call
				serv := &utilipvs.VirtualServer{
					Address:   net.ParseIP(ingress),
					Port:      uint16(svcInfo.Port()),
					Protocol:  string(svcInfo.Protocol()),
					Scheduler: proxier.ipvsScheduler,
				}
				if svcInfo.SessionAffinityType() == v1.ServiceAffinityClientIP {
					serv.Flags |= utilipvs.FlagPersistent
					serv.Timeout = uint32(svcInfo.StickyMaxAgeSeconds())
				}
				if err := proxier.syncService(svcNameString, serv, true, bindedAddresses); err == nil {
					activeIPVSServices[serv.String()] = true
					activeBindAddrs[serv.Address.String()] = true
					if err := proxier.syncEndpoint(svcName, svcInfo.NodeLocalExternal(), serv); err != nil {
						klog.ErrorS(err, "Failed to sync endpoint for service", "service", serv)
					}
				} else {
					klog.ErrorS(err, "Failed to sync service", "service", serv)
				}
			}
		}

		if svcInfo.NodePort() != 0 {
			if len(nodeAddresses) == 0 || len(nodeIPs) == 0 {
				// Skip nodePort configuration since an error occurred when
				// computing nodeAddresses or nodeIPs.
				continue
			}

			var lps []utilnet.LocalPort
			for _, address := range nodeAddresses {
				lp := utilnet.LocalPort{
					Description: "nodePort for " + svcNameString,
					IP:          address,
					IPFamily:    localPortIPFamily,
					Port:        svcInfo.NodePort(),
					Protocol:    utilnet.Protocol(svcInfo.Protocol()),
				}
				if utilproxy.IsZeroCIDR(address) {
					// Empty IP address means all
					lp.IP = ""
					lps = append(lps, lp)
					// If we encounter a zero CIDR, then there is no point in processing the rest of the addresses.
					break
				}
				lps = append(lps, lp)
			}

			// For ports on node IPs, open the actual port and hold it.
			for _, lp := range lps {
				if proxier.portsMap[lp] != nil {
					klog.V(4).InfoS("Port was open before and is still needed", "port", lp.String())
					replacementPortsMap[lp] = proxier.portsMap[lp]
					// We do not start listening on SCTP ports, according to our agreement in the
					// SCTP support KEP
				} else if svcInfo.Protocol() != v1.ProtocolSCTP {
					socket, err := proxier.portMapper.OpenLocalPort(&lp)
					if err != nil {
						msg := fmt.Sprintf("can't open port %s, skipping it", lp.String())

						proxier.recorder.Eventf(
							&v1.ObjectReference{
								Kind:      "Node",
								Name:      proxier.hostname,
								UID:       types.UID(proxier.hostname),
								Namespace: "",
							}, nil, v1.EventTypeWarning, err.Error(), "SyncProxyRules", msg)
						klog.ErrorS(err, "can't open port, skipping it", "port", lp.String())
						continue
					}
					klog.V(2).InfoS("Opened local port", "port", lp.String())

					if lp.Protocol == utilnet.UDP {
						conntrack.ClearEntriesForPort(proxier.exec, lp.Port, isIPv6, v1.ProtocolUDP)
					}
					replacementPortsMap[lp] = socket
				} // We're holding the port, so it's OK to install ipvs rules.
			}

			// Nodeports need SNAT, unless they're local.
			// ipset call

			var (
				nodePortSet *IPSet
				entries     []*utilipset.Entry
			)

			switch protocol {
			case utilipset.ProtocolTCP:
				nodePortSet = proxier.ipsetList[kubeNodePortSetTCP]
				entries = []*utilipset.Entry{{
					// No need to provide ip info
					Port:     svcInfo.NodePort(),
					Protocol: protocol,
					SetType:  utilipset.BitmapPort,
				}}
			case utilipset.ProtocolUDP:
				nodePortSet = proxier.ipsetList[kubeNodePortSetUDP]
				entries = []*utilipset.Entry{{
					// No need to provide ip info
					Port:     svcInfo.NodePort(),
					Protocol: protocol,
					SetType:  utilipset.BitmapPort,
				}}
			case utilipset.ProtocolSCTP:
				nodePortSet = proxier.ipsetList[kubeNodePortSetSCTP]
				// Since hash ip:port is used for SCTP, all the nodeIPs to be used in the SCTP ipset entries.
				entries = []*utilipset.Entry{}
				for _, nodeIP := range nodeIPs {
					entries = append(entries, &utilipset.Entry{
						IP:       nodeIP.String(),
						Port:     svcInfo.NodePort(),
						Protocol: protocol,
						SetType:  utilipset.HashIPPort,
					})
				}
			default:
				// It should never hit
				klog.ErrorS(nil, "Unsupported protocol type", "protocol", protocol)
			}
			if nodePortSet != nil {
				entryInvalidErr := false
				for _, entry := range entries {
					if valid := nodePortSet.validateEntry(entry); !valid {
						klog.ErrorS(nil, "error adding entry to ipset", "entry", entry.String(), "ipset", nodePortSet.Name)
						entryInvalidErr = true
						break
					}
					nodePortSet.activeEntries.Insert(entry.String())
				}
				if entryInvalidErr {
					continue
				}
			}

			// Add externaltrafficpolicy=local type nodeport entry
			if svcInfo.NodeLocalExternal() {
				var nodePortLocalSet *IPSet
				switch protocol {
				case utilipset.ProtocolTCP:
					nodePortLocalSet = proxier.ipsetList[kubeNodePortLocalSetTCP]
				case utilipset.ProtocolUDP:
					nodePortLocalSet = proxier.ipsetList[kubeNodePortLocalSetUDP]
				case utilipset.ProtocolSCTP:
					nodePortLocalSet = proxier.ipsetList[kubeNodePortLocalSetSCTP]
				default:
					// It should never hit
					klog.ErrorS(nil, "Unsupported protocol type", "protocol", protocol)
				}
				if nodePortLocalSet != nil {
					entryInvalidErr := false
					for _, entry := range entries {
						if valid := nodePortLocalSet.validateEntry(entry); !valid {
							klog.ErrorS(nil, "error adding entry to ipset", "entry", entry.String(), "ipset", nodePortLocalSet.Name)
							entryInvalidErr = true
							break
						}
						nodePortLocalSet.activeEntries.Insert(entry.String())
					}
					if entryInvalidErr {
						continue
					}
				}
			}

			// Build ipvs kernel routes for each node ip address
			for _, nodeIP := range nodeIPs {
				// ipvs call
				serv := &utilipvs.VirtualServer{
					Address:   nodeIP,
					Port:      uint16(svcInfo.NodePort()),
					Protocol:  string(svcInfo.Protocol()),
					Scheduler: proxier.ipvsScheduler,
				}
				if svcInfo.SessionAffinityType() == v1.ServiceAffinityClientIP {
					serv.Flags |= utilipvs.FlagPersistent
					serv.Timeout = uint32(svcInfo.StickyMaxAgeSeconds())
				}
				// There is no need to bind Node IP to dummy interface, so set parameter `bindAddr` to `false`.
				if err := proxier.syncService(svcNameString, serv, false, bindedAddresses); err == nil {
					activeIPVSServices[serv.String()] = true
					if err := proxier.syncEndpoint(svcName, svcInfo.NodeLocalExternal(), serv); err != nil {
						klog.ErrorS(err, "Failed to sync endpoint for service", "service", serv)
					}
				} else {
					klog.ErrorS(err, "Failed to sync service", "service", serv)
				}
			}
		}

		if svcInfo.HealthCheckNodePort() != 0 {
			nodePortSet := proxier.ipsetList[kubeHealthCheckNodePortSet]
			entry := &utilipset.Entry{
				// No need to provide ip info
				Port:     svcInfo.HealthCheckNodePort(),
				Protocol: "tcp",
				SetType:  utilipset.BitmapPort,
			}

			if valid := nodePortSet.validateEntry(entry); !valid {
				klog.ErrorS(nil, "error adding entry to ipset", "entry", entry.String(), "ipset", nodePortSet.Name)
				continue
			}
			nodePortSet.activeEntries.Insert(entry.String())
		}
	}

	// sync ipset entries
	for _, set := range proxier.ipsetList {
		set.syncIPSetEntries()
	}

	// Tail call iptables rules for ipset, make sure only call iptables once
	// in a single loop per ip set.
	proxier.writeIptablesRules()

	// Sync iptables rules.
	// NOTE: NoFlushTables is used so we don't flush non-kubernetes chains in the table.
	proxier.iptablesData.Reset()
	proxier.iptablesData.Write(proxier.natChains.Bytes())
	proxier.iptablesData.Write(proxier.natRules.Bytes())
	proxier.iptablesData.Write(proxier.filterChains.Bytes())
	proxier.iptablesData.Write(proxier.filterRules.Bytes())

	klog.V(5).InfoS("Restoring iptables", "rules", string(proxier.iptablesData.Bytes()))
	err = proxier.iptables.RestoreAll(proxier.iptablesData.Bytes(), utiliptables.NoFlushTables, utiliptables.RestoreCounters)
	if err != nil {
		klog.ErrorS(err, "Failed to execute iptables-restore", "rules", string(proxier.iptablesData.Bytes()))
		metrics.IptablesRestoreFailuresTotal.Inc()
		// Revert new local ports.
		utilproxy.RevertPorts(replacementPortsMap, proxier.portsMap)
		return
	}
	for name, lastChangeTriggerTimes := range endpointUpdateResult.LastChangeTriggerTimes {
		for _, lastChangeTriggerTime := range lastChangeTriggerTimes {
			latency := metrics.SinceInSeconds(lastChangeTriggerTime)
			metrics.NetworkProgrammingLatency.Observe(latency)
			klog.V(4).InfoS("Network programming", "endpoint", klog.KRef(name.Namespace, name.Name), "elapsed", latency)
		}
	}

	// Close old local ports and save new ones.
	for k, v := range proxier.portsMap {
		if replacementPortsMap[k] == nil {
			v.Close()
		}
	}
	proxier.portsMap = replacementPortsMap

	// Get legacy bind address
	// currentBindAddrs represents ip addresses bind to DefaultDummyDevice from the system
	currentBindAddrs, err := proxier.netlinkHandle.ListBindAddress(DefaultDummyDevice)
	if err != nil {
		klog.ErrorS(err, "Failed to get bind address")
	}
	legacyBindAddrs := proxier.getLegacyBindAddr(activeBindAddrs, currentBindAddrs)

	// Clean up legacy IPVS services and unbind addresses
	appliedSvcs, err := proxier.ipvs.GetVirtualServers()
	if err == nil {
		for _, appliedSvc := range appliedSvcs {
			currentIPVSServices[appliedSvc.String()] = appliedSvc
		}
	} else {
		klog.ErrorS(err, "Failed to get ipvs service")
	}
	proxier.cleanLegacyService(activeIPVSServices, currentIPVSServices, legacyBindAddrs)

	if proxier.healthzServer != nil {
		proxier.healthzServer.Updated()
	}
	metrics.SyncProxyRulesLastTimestamp.SetToCurrentTime()

	// Update service healthchecks.  The endpoints list might include services that are
	// not "OnlyLocal", but the services list will not, and the serviceHealthServer
	// will just drop those endpoints.
	if err := proxier.serviceHealthServer.SyncServices(serviceUpdateResult.HCServiceNodePorts); err != nil {
		klog.ErrorS(err, "Error syncing healthcheck services")
	}
	if err := proxier.serviceHealthServer.SyncEndpoints(endpointUpdateResult.HCEndpointsLocalIPSize); err != nil {
		klog.ErrorS(err, "Error syncing healthcheck endpoints")
	}

	// Finish housekeeping.
	// TODO: these could be made more consistent.
	for _, svcIP := range staleServices.UnsortedList() {
		if err := conntrack.ClearEntriesForIP(proxier.exec, svcIP, v1.ProtocolUDP); err != nil {
			klog.ErrorS(err, "Failed to delete stale service IP connections", "ip", svcIP)
		}
	}
	proxier.deleteEndpointConnections(endpointUpdateResult.StaleEndpoints)
}

// writeIptablesRules write all iptables rules to proxier.natRules or proxier.FilterRules that ipvs proxier needed
// according to proxier.ipsetList information and the ipset match relationship that `ipsetWithIptablesChain` specified.
// some ipset(kubeClusterIPSet for example) have particular match rules and iptables jump relation should be sync separately.
func (proxier *Proxier) writeIptablesRules() {
	// We are creating those slices ones here to avoid memory reallocations
	// in every loop. Note that reuse the memory, instead of doing:
	//   slice = <some new slice>
	// you should always do one of the below:
	//   slice = slice[:0] // and then append to it
	//   slice = append(slice[:0], ...)
	// To avoid growing this slice, we arbitrarily set its size to 64,
	// there is never more than that many arguments for a single line.
	// Note that even if we go over 64, it will still be correct - it
	// is just for efficiency, not correctness.
	args := make([]string, 64)

	for _, set := range ipsetWithIptablesChain {
		if _, find := proxier.ipsetList[set.name]; find && !proxier.ipsetList[set.name].isEmpty() {
			args = append(args[:0], "-A", set.from)
			if set.protocolMatch != "" {
				args = append(args, "-p", set.protocolMatch)
			}
			args = append(args,
				"-m", "comment", "--comment", proxier.ipsetList[set.name].getComment(),
				"-m", "set", "--match-set", proxier.ipsetList[set.name].Name,
				set.matchType,
			)
			utilproxy.WriteLine(proxier.natRules, append(args, "-j", set.to)...)
		}
	}

	if !proxier.ipsetList[kubeClusterIPSet].isEmpty() {
		args = append(args[:0],
			"-A", string(kubeServicesChain),
			"-m", "comment", "--comment", proxier.ipsetList[kubeClusterIPSet].getComment(),
			"-m", "set", "--match-set", proxier.ipsetList[kubeClusterIPSet].Name,
		)
		if proxier.masqueradeAll {
			utilproxy.WriteLine(proxier.natRules, append(args, "dst,dst", "-j", string(KubeMarkMasqChain))...)
		} else if proxier.localDetector.IsImplemented() {
			// This masquerades off-cluster traffic to a service VIP.  The idea
			// is that you can establish a static route for your Service range,
			// routing to any node, and that node will bridge into the Service
			// for you.  Since that might bounce off-node, we masquerade here.
			// If/when we support "Local" policy for VIPs, we should update this.
			utilproxy.WriteLine(proxier.natRules, proxier.localDetector.JumpIfNotLocal(append(args, "dst,dst"), string(KubeMarkMasqChain))...)
		} else {
			// Masquerade all OUTPUT traffic coming from a service ip.
			// The kube dummy interface has all service VIPs assigned which
			// results in the service VIP being picked as the source IP to reach
			// a VIP. This leads to a connection from VIP:<random port> to
			// VIP:<service port>.
			// Always masquerading OUTPUT (node-originating) traffic with a VIP
			// source ip and service port destination fixes the outgoing connections.
			utilproxy.WriteLine(proxier.natRules, append(args, "src,dst", "-j", string(KubeMarkMasqChain))...)
		}
	}

	// externalIPRules adds iptables rules applies to Service ExternalIPs
	externalIPRules := func(args []string) {
		// Allow traffic for external IPs that does not come from a bridge (i.e. not from a container)
		// nor from a local process to be forwarded to the service.
		// This rule roughly translates to "all traffic from off-machine".
		// This is imperfect in the face of network plugins that might not use a bridge, but we can revisit that later.
		externalTrafficOnlyArgs := append(args,
			"-m", "physdev", "!", "--physdev-is-in",
			"-m", "addrtype", "!", "--src-type", "LOCAL")
		utilproxy.WriteLine(proxier.natRules, append(externalTrafficOnlyArgs, "-j", "ACCEPT")...)
		dstLocalOnlyArgs := append(args, "-m", "addrtype", "--dst-type", "LOCAL")
		// Allow traffic bound for external IPs that happen to be recognized as local IPs to stay local.
		// This covers cases like GCE load-balancers which get added to the local routing table.
		utilproxy.WriteLine(proxier.natRules, append(dstLocalOnlyArgs, "-j", "ACCEPT")...)
	}

	if !proxier.ipsetList[kubeExternalIPSet].isEmpty() {
		// Build masquerade rules for packets to external IPs.
		args = append(args[:0],
			"-A", string(kubeServicesChain),
			"-m", "comment", "--comment", proxier.ipsetList[kubeExternalIPSet].getComment(),
			"-m", "set", "--match-set", proxier.ipsetList[kubeExternalIPSet].Name,
			"dst,dst",
		)
		utilproxy.WriteLine(proxier.natRules, append(args, "-j", string(KubeMarkMasqChain))...)
		externalIPRules(args)
	}

	if !proxier.ipsetList[kubeExternalIPLocalSet].isEmpty() {
		args = append(args[:0],
			"-A", string(kubeServicesChain),
			"-m", "comment", "--comment", proxier.ipsetList[kubeExternalIPLocalSet].getComment(),
			"-m", "set", "--match-set", proxier.ipsetList[kubeExternalIPLocalSet].Name,
			"dst,dst",
		)
		externalIPRules(args)
	}

	// -A KUBE-SERVICES  -m addrtype  --dst-type LOCAL -j KUBE-NODE-PORT
	args = append(args[:0],
		"-A", string(kubeServicesChain),
		"-m", "addrtype", "--dst-type", "LOCAL",
	)
	utilproxy.WriteLine(proxier.natRules, append(args, "-j", string(KubeNodePortChain))...)

	// mark drop for KUBE-LOAD-BALANCER
	utilproxy.WriteLine(proxier.natRules, []string{
		"-A", string(KubeLoadBalancerChain),
		"-j", string(KubeMarkMasqChain),
	}...)

	// mark drop for KUBE-FIRE-WALL
	utilproxy.WriteLine(proxier.natRules, []string{
		"-A", string(KubeFireWallChain),
		"-j", string(KubeMarkDropChain),
	}...)

	// Accept all traffic with destination of ipvs virtual service, in case other iptables rules
	// block the traffic, that may result in ipvs rules invalid.
	// Those rules must be in the end of KUBE-SERVICE chain
	proxier.acceptIPVSTraffic()

	// If the masqueradeMark has been added then we want to forward that same
	// traffic, this allows NodePort traffic to be forwarded even if the default
	// FORWARD policy is not accept.
	utilproxy.WriteLine(proxier.filterRules,
		"-A", string(KubeForwardChain),
		"-m", "comment", "--comment", `"kubernetes forwarding rules"`,
		"-m", "mark", "--mark", fmt.Sprintf("%s/%s", proxier.masqueradeMark, proxier.masqueradeMark),
		"-j", "ACCEPT",
	)

	// The following two rules ensure the traffic after the initial packet
	// accepted by the "kubernetes forwarding rules" rule above will be
	// accepted.
	utilproxy.WriteLine(proxier.filterRules,
		"-A", string(KubeForwardChain),
		"-m", "comment", "--comment", `"kubernetes forwarding conntrack pod source rule"`,
		"-m", "conntrack",
		"--ctstate", "RELATED,ESTABLISHED",
		"-j", "ACCEPT",
	)
	utilproxy.WriteLine(proxier.filterRules,
		"-A", string(KubeForwardChain),
		"-m", "comment", "--comment", `"kubernetes forwarding conntrack pod destination rule"`,
		"-m", "conntrack",
		"--ctstate", "RELATED,ESTABLISHED",
		"-j", "ACCEPT",
	)

	// Add rule to accept traffic towards health check node port
	utilproxy.WriteLine(proxier.filterRules,
		"-A", string(KubeNodePortChain),
		"-m", "comment", "--comment", proxier.ipsetList[kubeHealthCheckNodePortSet].getComment(),
		"-m", "set", "--match-set", proxier.ipsetList[kubeHealthCheckNodePortSet].Name, "dst",
		"-j", "ACCEPT",
	)

	// Install the kubernetes-specific postrouting rules. We use a whole chain for
	// this so that it is easier to flush and change, for example if the mark
	// value should ever change.
	// NB: THIS MUST MATCH the corresponding code in the kubelet
	utilproxy.WriteLine(proxier.natRules, []string{
		"-A", string(kubePostroutingChain),
		"-m", "mark", "!", "--mark", fmt.Sprintf("%s/%s", proxier.masqueradeMark, proxier.masqueradeMark),
		"-j", "RETURN",
	}...)
	// Clear the mark to avoid re-masquerading if the packet re-traverses the network stack.
	utilproxy.WriteLine(proxier.natRules, []string{
		"-A", string(kubePostroutingChain),
		// XOR proxier.masqueradeMark to unset it
		"-j", "MARK", "--xor-mark", proxier.masqueradeMark,
	}...)
	masqRule := []string{
		"-A", string(kubePostroutingChain),
		"-m", "comment", "--comment", `"kubernetes service traffic requiring SNAT"`,
		"-j", "MASQUERADE",
	}
	if proxier.iptables.HasRandomFully() {
		masqRule = append(masqRule, "--random-fully")
	}
	utilproxy.WriteLine(proxier.natRules, masqRule...)

	// Install the kubernetes-specific masquerade mark rule. We use a whole chain for
	// this so that it is easier to flush and change, for example if the mark
	// value should ever change.
	utilproxy.WriteLine(proxier.natRules, []string{
		"-A", string(KubeMarkMasqChain),
		"-j", "MARK", "--or-mark", proxier.masqueradeMark,
	}...)

	// Write the end-of-table markers.
	utilproxy.WriteLine(proxier.filterRules, "COMMIT")
	utilproxy.WriteLine(proxier.natRules, "COMMIT")
}

func (proxier *Proxier) acceptIPVSTraffic() {
	sets := []string{kubeClusterIPSet, kubeLoadBalancerSet}
	for _, set := range sets {
		var matchType string
		if !proxier.ipsetList[set].isEmpty() {
			switch proxier.ipsetList[set].SetType {
			case utilipset.BitmapPort:
				matchType = "dst"
			default:
				matchType = "dst,dst"
			}
			utilproxy.WriteLine(proxier.natRules, []string{
				"-A", string(kubeServicesChain),
				"-m", "set", "--match-set", proxier.ipsetList[set].Name, matchType,
				"-j", "ACCEPT",
			}...)
		}
	}
}

// createAndLinkKubeChain create all kube chains that ipvs proxier need and write basic link.
func (proxier *Proxier) createAndLinkKubeChain() {
	existingFilterChains := proxier.getExistingChains(proxier.filterChainsData, utiliptables.TableFilter)
	existingNATChains := proxier.getExistingChains(proxier.iptablesData, utiliptables.TableNAT)

	// ensure KUBE-MARK-DROP chain exist but do not change any rules
	for _, ch := range iptablesEnsureChains {
		if _, err := proxier.iptables.EnsureChain(ch.table, ch.chain); err != nil {
			klog.ErrorS(err, "Failed to ensure chain exists", "table", ch.table, "chain", ch.chain)
			return
		}
	}

	// Make sure we keep stats for the top-level chains
	for _, ch := range iptablesChains {
		if _, err := proxier.iptables.EnsureChain(ch.table, ch.chain); err != nil {
			klog.ErrorS(err, "Failed to ensure chain exists", "table", ch.table, "chain", ch.chain)
			return
		}
		if ch.table == utiliptables.TableNAT {
			if chain, ok := existingNATChains[ch.chain]; ok {
				utilproxy.WriteBytesLine(proxier.natChains, chain)
			} else {
				utilproxy.WriteLine(proxier.natChains, utiliptables.MakeChainLine(ch.chain))
			}
		} else {
			if chain, ok := existingFilterChains[ch.chain]; ok {
				utilproxy.WriteBytesLine(proxier.filterChains, chain)
			} else {
				utilproxy.WriteLine(proxier.filterChains, utiliptables.MakeChainLine(ch.chain))
			}
		}
	}

	for _, jc := range iptablesJumpChain {
		args := []string{"-m", "comment", "--comment", jc.comment, "-j", string(jc.to)}
		if _, err := proxier.iptables.EnsureRule(utiliptables.Prepend, jc.table, jc.from, args...); err != nil {
			klog.ErrorS(err, "Failed to ensure chain jumps", "table", jc.table, "srcChain", jc.from, "dstChain", jc.to)
		}
	}

}

// getExistingChains get iptables-save output so we can check for existing chains and rules.
// This will be a map of chain name to chain with rules as stored in iptables-save/iptables-restore
// Result may SHARE memory with contents of buffer.
func (proxier *Proxier) getExistingChains(buffer *bytes.Buffer, table utiliptables.Table) map[utiliptables.Chain][]byte {
	buffer.Reset()
	err := proxier.iptables.SaveInto(table, buffer)
	if err != nil { // if we failed to get any rules
		klog.ErrorS(err, "Failed to execute iptables-save, syncing all rules")
	} else { // otherwise parse the output
		return utiliptables.GetChainLines(table, buffer.Bytes())
	}
	return nil
}

// After a UDP or SCTP endpoint has been removed, we must flush any pending conntrack entries to it, or else we
// risk sending more traffic to it, all of which will be lost (because UDP).
// This assumes the proxier mutex is held
func (proxier *Proxier) deleteEndpointConnections(connectionMap []proxy.ServiceEndpoint) {
	for _, epSvcPair := range connectionMap {
		if svcInfo, ok := proxier.serviceMap[epSvcPair.ServicePortName]; ok && conntrack.IsClearConntrackNeeded(svcInfo.Protocol()) {
			endpointIP := utilproxy.IPPart(epSvcPair.Endpoint)
			svcProto := svcInfo.Protocol()
			err := conntrack.ClearEntriesForNAT(proxier.exec, svcInfo.ClusterIP().String(), endpointIP, svcProto)
			if err != nil {
				klog.ErrorS(err, "Failed to delete endpoint connections", "servicePortName", epSvcPair.ServicePortName.String())
			}
			for _, extIP := range svcInfo.ExternalIPStrings() {
				err := conntrack.ClearEntriesForNAT(proxier.exec, extIP, endpointIP, svcProto)
				if err != nil {
					klog.ErrorS(err, "Failed to delete endpoint connections for externalIP", "servicePortName", epSvcPair.ServicePortName.String(), "ip", extIP)
				}
			}
			for _, lbIP := range svcInfo.LoadBalancerIPStrings() {
				err := conntrack.ClearEntriesForNAT(proxier.exec, lbIP, endpointIP, svcProto)
				if err != nil {
					klog.ErrorS(err, "Failed to delete endpoint connections for LoadBalancerIP", "servicePortName", epSvcPair.ServicePortName.String(), "ip", lbIP)
				}
			}
		}
	}
}

func (proxier *Proxier) syncService(svcName string, vs *utilipvs.VirtualServer, bindAddr bool, bindedAddresses sets.String) error {
	appliedVirtualServer, _ := proxier.ipvs.GetVirtualServer(vs)
	if appliedVirtualServer == nil || !appliedVirtualServer.Equal(vs) {
		if appliedVirtualServer == nil {
			// IPVS service is not found, create a new service
			klog.V(3).InfoS("Adding new service", "svcName", svcName, "address", fmt.Sprintf("%s:%d/%s", vs.Address, vs.Port, vs.Protocol))
			if err := proxier.ipvs.AddVirtualServer(vs); err != nil {
				klog.ErrorS(err, "Failed to add IPVS service", "svcName", svcName)
				return err
			}
		} else {
			// IPVS service was changed, update the existing one
			// During updates, service VIP will not go down
			klog.V(3).InfoS("IPVS service was changed", "svcName", svcName)
			if err := proxier.ipvs.UpdateVirtualServer(vs); err != nil {
				klog.ErrorS(err, "Failed to update IPVS service")
				return err
			}
		}
	}

	// bind service address to dummy interface
	if bindAddr {
		// always attempt to bind if bindedAddresses is nil,
		// otherwise check if it's already binded and return early
		if bindedAddresses != nil && bindedAddresses.Has(vs.Address.String()) {
			return nil
		}

		klog.V(4).InfoS("Bind addr", "address", vs.Address.String())
		_, err := proxier.netlinkHandle.EnsureAddressBind(vs.Address.String(), DefaultDummyDevice)
		if err != nil {
			klog.ErrorS(err, "Failed to bind service address to dummy device", "svcName", svcName)
			return err
		}
	}

	return nil
}

func (proxier *Proxier) syncEndpoint(svcPortName proxy.ServicePortName, onlyNodeLocalEndpoints bool, vs *utilipvs.VirtualServer) error {
	appliedVirtualServer, err := proxier.ipvs.GetVirtualServer(vs)
	if err != nil {
		klog.Errorf("Failed to get IPVS service, error: %v", err)
		return err
	}
	if appliedVirtualServer == nil {
		return errors.New("IPVS virtual service does not exist")
	}

	// curEndpoints represents IPVS destinations listed from current system.
	curEndpoints := sets.NewString()
	// readyEndpoints represents Endpoints watched from API Server.
	readyEndpoints := sets.NewString()
	// localReadyEndpoints represents local endpoints that are ready and NOT terminating.
	localReadyEndpoints := sets.NewString()
	// localReadyTerminatingEndpoints represents local endpoints that are ready AND terminating.
	// Fall back to these endpoints if no non-terminating ready endpoints exist for node-local traffic.
	localReadyTerminatingEndpoints := sets.NewString()

	curDests, err := proxier.ipvs.GetRealServers(appliedVirtualServer)
	if err != nil {
		klog.ErrorS(err, "Failed to list IPVS destinations")
		return err
	}
	for _, des := range curDests {
		curEndpoints.Insert(des.String())
	}

	endpoints := proxier.endpointsMap[svcPortName]

	// Filtering for topology aware endpoints. This function will only
	// filter endpoints if appropriate feature gates are enabled and the
	// Service does not have conflicting configuration such as
	// externalTrafficPolicy=Local.
	svcInfo, ok := proxier.serviceMap[svcPortName]
	if !ok {
		klog.InfoS("Unable to filter endpoints due to missing Service info", "svcPortName", svcPortName)
	} else {
		endpoints = proxy.FilterEndpoints(endpoints, svcInfo, proxier.nodeLabels)
	}

	for _, epInfo := range endpoints {
		if epInfo.IsReady() {
			readyEndpoints.Insert(epInfo.String())
		}

		if onlyNodeLocalEndpoints && epInfo.GetIsLocal() {
			if epInfo.IsReady() {
				localReadyEndpoints.Insert(epInfo.String())
			} else if epInfo.IsServing() && epInfo.IsTerminating() {
				localReadyTerminatingEndpoints.Insert(epInfo.String())
			}
		}
	}

	newEndpoints := readyEndpoints
	if onlyNodeLocalEndpoints {
		newEndpoints = localReadyEndpoints

		if utilfeature.DefaultFeatureGate.Enabled(features.ProxyTerminatingEndpoints) {
			if len(newEndpoints) == 0 && localReadyTerminatingEndpoints.Len() > 0 {
				newEndpoints = localReadyTerminatingEndpoints
			}
		}
	}

	// Create new endpoints
	for _, ep := range newEndpoints.List() {
		ip, port, err := net.SplitHostPort(ep)
		if err != nil {
			klog.ErrorS(err, "Failed to parse endpoint", "endpoint", ep)
			continue
		}
		portNum, err := strconv.Atoi(port)
		if err != nil {
			klog.ErrorS(err, "Failed to parse endpoint port", "port", port)
			continue
		}

		newDest := &utilipvs.RealServer{
			Address: net.ParseIP(ip),
			Port:    uint16(portNum),
			Weight:  1,
		}

		if curEndpoints.Has(ep) {
			// check if newEndpoint is in gracefulDelete list, if true, delete this ep immediately
			uniqueRS := GetUniqueRSName(vs, newDest)
			if !proxier.gracefuldeleteManager.InTerminationList(uniqueRS) {
				continue
			}
			klog.V(5).InfoS("new ep is in graceful delete list", "uniqueRS", uniqueRS)
			err := proxier.gracefuldeleteManager.MoveRSOutofGracefulDeleteList(uniqueRS)
			if err != nil {
				klog.ErrorS(err, "Failed to delete endpoint in gracefulDeleteQueue", "endpoint", ep)
				continue
			}
		}
		err = proxier.ipvs.AddRealServer(appliedVirtualServer, newDest)
		if err != nil {
			klog.ErrorS(err, "Failed to add destination", "newDest", newDest)
			continue
		}
	}
	// Delete old endpoints
	for _, ep := range curEndpoints.Difference(newEndpoints).UnsortedList() {
		// if curEndpoint is in gracefulDelete, skip
		uniqueRS := vs.String() + "/" + ep
		if proxier.gracefuldeleteManager.InTerminationList(uniqueRS) {
			continue
		}
		ip, port, err := net.SplitHostPort(ep)
		if err != nil {
			klog.ErrorS(err, "Failed to parse endpoint", "endpoint", ep)
			continue
		}
		portNum, err := strconv.Atoi(port)
		if err != nil {
			klog.ErrorS(err, "Failed to parse endpoint port", "port", port)
			continue
		}

		delDest := &utilipvs.RealServer{
			Address: net.ParseIP(ip),
			Port:    uint16(portNum),
		}

		klog.V(5).InfoS("Using graceful delete", "uniqueRS", uniqueRS)
		err = proxier.gracefuldeleteManager.GracefulDeleteRS(appliedVirtualServer, delDest)
		if err != nil {
			klog.ErrorS(err, "Failed to delete destination", "uniqueRS", uniqueRS)
			continue
		}
	}
	return nil
}

func (proxier *Proxier) cleanLegacyService(activeServices map[string]bool, currentServices map[string]*utilipvs.VirtualServer, legacyBindAddrs map[string]bool) {
	isIPv6 := utilnet.IsIPv6(proxier.nodeIP)
	for cs := range currentServices {
		svc := currentServices[cs]
		if proxier.isIPInExcludeCIDRs(svc.Address) {
			continue
		}
		if utilnet.IsIPv6(svc.Address) != isIPv6 {
			// Not our family
			continue
		}
		if _, ok := activeServices[cs]; !ok {
			klog.V(4).InfoS("Delete service", "service", svc.String())
			if err := proxier.ipvs.DeleteVirtualServer(svc); err != nil {
				klog.ErrorS(err, "Failed to delete service", "service", svc.String())
			}
			addr := svc.Address.String()
			if _, ok := legacyBindAddrs[addr]; ok {
				klog.V(4).InfoS("Unbinding address", "address", addr)
				if err := proxier.netlinkHandle.UnbindAddress(addr, DefaultDummyDevice); err != nil {
					klog.ErrorS(err, "Failed to unbind service from dummy interface", "interface", DefaultDummyDevice, "address", addr)
				} else {
					// In case we delete a multi-port service, avoid trying to unbind multiple times
					delete(legacyBindAddrs, addr)
				}
			}
		}
	}
}

func (proxier *Proxier) isIPInExcludeCIDRs(ip net.IP) bool {
	// make sure it does not fall within an excluded CIDR range.
	for _, excludedCIDR := range proxier.excludeCIDRs {
		if excludedCIDR.Contains(ip) {
			return true
		}
	}
	return false
}

func (proxier *Proxier) getLegacyBindAddr(activeBindAddrs map[string]bool, currentBindAddrs []string) map[string]bool {
	legacyAddrs := make(map[string]bool)
	isIPv6 := utilnet.IsIPv6(proxier.nodeIP)
	for _, addr := range currentBindAddrs {
		addrIsIPv6 := utilnet.IsIPv6(net.ParseIP(addr))
		if addrIsIPv6 && !isIPv6 || !addrIsIPv6 && isIPv6 {
			continue
		}
		if _, ok := activeBindAddrs[addr]; !ok {
			legacyAddrs[addr] = true
		}
	}
	return legacyAddrs
}

// ipvs Proxier fall back on iptables when it needs to do SNAT for engress packets
// It will only operate iptables *nat table.
// Create and link the kube postrouting chain for SNAT packets.
// Chain POSTROUTING (policy ACCEPT)
// target     prot opt source               destination
// KUBE-POSTROUTING  all  --  0.0.0.0/0            0.0.0.0/0            /* kubernetes postrouting rules *
// Maintain by kubelet network sync loop

// *nat
// :KUBE-POSTROUTING - [0:0]
// Chain KUBE-POSTROUTING (1 references)
// target     prot opt source               destination
// MASQUERADE  all  --  0.0.0.0/0            0.0.0.0/0            /* kubernetes service traffic requiring SNAT */ mark match 0x4000/0x4000

// :KUBE-MARK-MASQ - [0:0]
// Chain KUBE-MARK-MASQ (0 references)
// target     prot opt source               destination
// MARK       all  --  0.0.0.0/0            0.0.0.0/0            MARK or 0x4000
