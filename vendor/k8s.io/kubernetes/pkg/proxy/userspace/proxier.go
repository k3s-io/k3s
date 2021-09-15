/*
Copyright 2014 The Kubernetes Authors.

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

package userspace

import (
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	libcontaineruserns "github.com/opencontainers/runc/libcontainer/userns"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	servicehelper "k8s.io/cloud-provider/service/helpers"
	"k8s.io/klog/v2"
	kubefeatures "k8s.io/kubernetes/pkg/features"
	"k8s.io/kubernetes/pkg/proxy"
	"k8s.io/kubernetes/pkg/proxy/config"
	utilproxy "k8s.io/kubernetes/pkg/proxy/util"
	"k8s.io/kubernetes/pkg/util/async"
	"k8s.io/kubernetes/pkg/util/conntrack"
	"k8s.io/kubernetes/pkg/util/iptables"
	utilexec "k8s.io/utils/exec"
	netutils "k8s.io/utils/net"
)

type portal struct {
	ip         net.IP
	port       int
	isExternal bool
}

// ServiceInfo contains information and state for a particular proxied service
type ServiceInfo struct {
	// Timeout is the read/write timeout (used for UDP connections)
	Timeout time.Duration
	// ActiveClients is the cache of active UDP clients being proxied by this proxy for this service
	ActiveClients *ClientCache

	isAliveAtomic       int32 // Only access this with atomic ops
	portal              portal
	protocol            v1.Protocol
	proxyPort           int
	socket              ProxySocket
	nodePort            int
	loadBalancerStatus  v1.LoadBalancerStatus
	sessionAffinityType v1.ServiceAffinity
	stickyMaxAgeSeconds int
	// Deprecated, but required for back-compat (including e2e)
	externalIPs []string

	// isStartedAtomic is set to non-zero when the service's socket begins
	// accepting requests. Used in testcases. Only access this with atomic ops.
	isStartedAtomic int32
	// isFinishedAtomic is set to non-zero when the service's socket shuts
	// down. Used in testcases. Only access this with atomic ops.
	isFinishedAtomic int32
}

func (info *ServiceInfo) setStarted() {
	atomic.StoreInt32(&info.isStartedAtomic, 1)
}

func (info *ServiceInfo) IsStarted() bool {
	return atomic.LoadInt32(&info.isStartedAtomic) != 0
}

func (info *ServiceInfo) setFinished() {
	atomic.StoreInt32(&info.isFinishedAtomic, 1)
}

func (info *ServiceInfo) IsFinished() bool {
	return atomic.LoadInt32(&info.isFinishedAtomic) != 0
}

func (info *ServiceInfo) setAlive(b bool) {
	var i int32
	if b {
		i = 1
	}
	atomic.StoreInt32(&info.isAliveAtomic, i)
}

func (info *ServiceInfo) IsAlive() bool {
	return atomic.LoadInt32(&info.isAliveAtomic) != 0
}

func logTimeout(err error) bool {
	if e, ok := err.(net.Error); ok {
		if e.Timeout() {
			klog.V(3).InfoS("Connection to endpoint closed due to inactivity")
			return true
		}
	}
	return false
}

// ProxySocketFunc is a function which constructs a ProxySocket from a protocol, ip, and port
type ProxySocketFunc func(protocol v1.Protocol, ip net.IP, port int) (ProxySocket, error)

const numBurstSyncs int = 2

type serviceChange struct {
	current  *v1.Service
	previous *v1.Service
}

// Interface for async runner; abstracted for testing
type asyncRunnerInterface interface {
	Run()
	Loop(<-chan struct{})
}

// Proxier is a simple proxy for TCP connections between a localhost:lport
// and services that provide the actual implementations.
type Proxier struct {
	// EndpointSlice support has not been added for this proxier yet.
	config.NoopEndpointSliceHandler
	// TODO(imroc): implement node handler for userspace proxier.
	config.NoopNodeHandler

	loadBalancer    LoadBalancer
	mu              sync.Mutex // protects serviceMap
	serviceMap      map[proxy.ServicePortName]*ServiceInfo
	syncPeriod      time.Duration
	minSyncPeriod   time.Duration
	udpIdleTimeout  time.Duration
	portMapMutex    sync.Mutex
	portMap         map[portMapKey]*portMapValue
	listenIP        net.IP
	iptables        iptables.Interface
	hostIP          net.IP
	localAddrs      netutils.IPSet
	proxyPorts      PortAllocator
	makeProxySocket ProxySocketFunc
	exec            utilexec.Interface
	// endpointsSynced and servicesSynced are set to 1 when the corresponding
	// objects are synced after startup. This is used to avoid updating iptables
	// with some partial data after kube-proxy restart.
	endpointsSynced int32
	servicesSynced  int32
	initialized     int32
	// protects serviceChanges
	serviceChangesLock sync.Mutex
	serviceChanges     map[types.NamespacedName]*serviceChange // map of service changes
	syncRunner         asyncRunnerInterface                    // governs calls to syncProxyRules

	stopChan chan struct{}
}

// assert Proxier is a proxy.Provider
var _ proxy.Provider = &Proxier{}

// A key for the portMap.  The ip has to be a string because slices can't be map
// keys.
type portMapKey struct {
	ip       string
	port     int
	protocol v1.Protocol
}

func (k *portMapKey) String() string {
	return fmt.Sprintf("%s/%s", net.JoinHostPort(k.ip, strconv.Itoa(k.port)), k.protocol)
}

// A value for the portMap
type portMapValue struct {
	owner  proxy.ServicePortName
	socket interface {
		Close() error
	}
}

var (
	// ErrProxyOnLocalhost is returned by NewProxier if the user requests a proxier on
	// the loopback address. May be checked for by callers of NewProxier to know whether
	// the caller provided invalid input.
	ErrProxyOnLocalhost = fmt.Errorf("cannot proxy on localhost")
)

// NewProxier returns a new Proxier given a LoadBalancer and an address on
// which to listen.  Because of the iptables logic, It is assumed that there
// is only a single Proxier active on a machine. An error will be returned if
// the proxier cannot be started due to an invalid ListenIP (loopback) or
// if iptables fails to update or acquire the initial lock. Once a proxier is
// created, it will keep iptables up to date in the background and will not
// terminate if a particular iptables call fails.
func NewProxier(loadBalancer LoadBalancer, listenIP net.IP, iptables iptables.Interface, exec utilexec.Interface, pr utilnet.PortRange, syncPeriod, minSyncPeriod, udpIdleTimeout time.Duration, nodePortAddresses []string) (*Proxier, error) {
	return NewCustomProxier(loadBalancer, listenIP, iptables, exec, pr, syncPeriod, minSyncPeriod, udpIdleTimeout, nodePortAddresses, newProxySocket)
}

// NewCustomProxier functions similarly to NewProxier, returning a new Proxier
// for the given LoadBalancer and address.  The new proxier is constructed using
// the ProxySocket constructor provided, however, instead of constructing the
// default ProxySockets.
func NewCustomProxier(loadBalancer LoadBalancer, listenIP net.IP, iptables iptables.Interface, exec utilexec.Interface, pr utilnet.PortRange, syncPeriod, minSyncPeriod, udpIdleTimeout time.Duration, nodePortAddresses []string, makeProxySocket ProxySocketFunc) (*Proxier, error) {
	if listenIP.Equal(localhostIPv4) || listenIP.Equal(localhostIPv6) {
		return nil, ErrProxyOnLocalhost
	}

	// If listenIP is given, assume that is the intended host IP.  Otherwise
	// try to find a suitable host IP address from network interfaces.
	var err error
	hostIP := listenIP
	if hostIP.Equal(net.IPv4zero) || hostIP.Equal(net.IPv6zero) {
		hostIP, err = utilnet.ChooseHostInterface()
		if err != nil {
			return nil, fmt.Errorf("failed to select a host interface: %v", err)
		}
	}

	err = setRLimit(64 * 1000)
	if err != nil {
		if utilfeature.DefaultFeatureGate.Enabled(kubefeatures.KubeletInUserNamespace) && libcontaineruserns.RunningInUserNS() {
			klog.V(2).InfoS("Failed to set open file handler limit to 64000 (running in UserNS, ignoring)", "err", err)
		} else {
			return nil, fmt.Errorf("failed to set open file handler limit to 64000: %w", err)
		}
	}

	proxyPorts := newPortAllocator(pr)

	klog.V(2).InfoS("Setting proxy IP and initializing iptables", "ip", hostIP)
	return createProxier(loadBalancer, listenIP, iptables, exec, hostIP, proxyPorts, syncPeriod, minSyncPeriod, udpIdleTimeout, makeProxySocket)
}

func createProxier(loadBalancer LoadBalancer, listenIP net.IP, iptables iptables.Interface, exec utilexec.Interface, hostIP net.IP, proxyPorts PortAllocator, syncPeriod, minSyncPeriod, udpIdleTimeout time.Duration, makeProxySocket ProxySocketFunc) (*Proxier, error) {
	// convenient to pass nil for tests..
	if proxyPorts == nil {
		proxyPorts = newPortAllocator(utilnet.PortRange{})
	}
	// Set up the iptables foundations we need.
	if err := iptablesInit(iptables); err != nil {
		return nil, fmt.Errorf("failed to initialize iptables: %v", err)
	}
	// Flush old iptables rules (since the bound ports will be invalid after a restart).
	// When OnUpdate() is first called, the rules will be recreated.
	if err := iptablesFlush(iptables); err != nil {
		return nil, fmt.Errorf("failed to flush iptables: %v", err)
	}
	proxier := &Proxier{
		loadBalancer:    loadBalancer,
		serviceMap:      make(map[proxy.ServicePortName]*ServiceInfo),
		serviceChanges:  make(map[types.NamespacedName]*serviceChange),
		portMap:         make(map[portMapKey]*portMapValue),
		syncPeriod:      syncPeriod,
		minSyncPeriod:   minSyncPeriod,
		udpIdleTimeout:  udpIdleTimeout,
		listenIP:        listenIP,
		iptables:        iptables,
		hostIP:          hostIP,
		proxyPorts:      proxyPorts,
		makeProxySocket: makeProxySocket,
		exec:            exec,
		stopChan:        make(chan struct{}),
	}
	klog.V(3).InfoS("Record sync param", "minSyncPeriod", minSyncPeriod, "syncPeriod", syncPeriod, "burstSyncs", numBurstSyncs)
	proxier.syncRunner = async.NewBoundedFrequencyRunner("userspace-proxy-sync-runner", proxier.syncProxyRules, minSyncPeriod, syncPeriod, numBurstSyncs)
	return proxier, nil
}

// CleanupLeftovers removes all iptables rules and chains created by the Proxier
// It returns true if an error was encountered. Errors are logged.
func CleanupLeftovers(ipt iptables.Interface) (encounteredError bool) {
	// NOTE: Warning, this needs to be kept in sync with the userspace Proxier,
	// we want to ensure we remove all of the iptables rules it creates.
	// Currently they are all in iptablesInit()
	// Delete Rules first, then Flush and Delete Chains
	args := []string{"-m", "comment", "--comment", "handle ClusterIPs; NOTE: this must be before the NodePort rules"}
	if err := ipt.DeleteRule(iptables.TableNAT, iptables.ChainOutput, append(args, "-j", string(iptablesHostPortalChain))...); err != nil {
		if !iptables.IsNotFoundError(err) {
			klog.ErrorS(err, "Error removing userspace rule")
			encounteredError = true
		}
	}
	if err := ipt.DeleteRule(iptables.TableNAT, iptables.ChainPrerouting, append(args, "-j", string(iptablesContainerPortalChain))...); err != nil {
		if !iptables.IsNotFoundError(err) {
			klog.ErrorS(err, "Error removing userspace rule")
			encounteredError = true
		}
	}
	args = []string{"-m", "addrtype", "--dst-type", "LOCAL"}
	args = append(args, "-m", "comment", "--comment", "handle service NodePorts; NOTE: this must be the last rule in the chain")
	if err := ipt.DeleteRule(iptables.TableNAT, iptables.ChainOutput, append(args, "-j", string(iptablesHostNodePortChain))...); err != nil {
		if !iptables.IsNotFoundError(err) {
			klog.ErrorS(err, "Error removing userspace rule")
			encounteredError = true
		}
	}
	if err := ipt.DeleteRule(iptables.TableNAT, iptables.ChainPrerouting, append(args, "-j", string(iptablesContainerNodePortChain))...); err != nil {
		if !iptables.IsNotFoundError(err) {
			klog.ErrorS(err, "Error removing userspace rule")
			encounteredError = true
		}
	}
	args = []string{"-m", "comment", "--comment", "Ensure that non-local NodePort traffic can flow"}
	if err := ipt.DeleteRule(iptables.TableFilter, iptables.ChainInput, append(args, "-j", string(iptablesNonLocalNodePortChain))...); err != nil {
		if !iptables.IsNotFoundError(err) {
			klog.ErrorS(err, "Error removing userspace rule")
			encounteredError = true
		}
	}

	// flush and delete chains.
	tableChains := map[iptables.Table][]iptables.Chain{
		iptables.TableNAT:    {iptablesContainerPortalChain, iptablesHostPortalChain, iptablesHostNodePortChain, iptablesContainerNodePortChain},
		iptables.TableFilter: {iptablesNonLocalNodePortChain},
	}
	for table, chains := range tableChains {
		for _, c := range chains {
			// flush chain, then if successful delete, delete will fail if flush fails.
			if err := ipt.FlushChain(table, c); err != nil {
				if !iptables.IsNotFoundError(err) {
					klog.ErrorS(err, "Error flushing userspace chain")
					encounteredError = true
				}
			} else {
				if err = ipt.DeleteChain(table, c); err != nil {
					if !iptables.IsNotFoundError(err) {
						klog.ErrorS(err, "Error deleting userspace chain")
						encounteredError = true
					}
				}
			}
		}
	}
	return encounteredError
}

// shutdown closes all service port proxies and returns from the proxy's
// sync loop. Used from testcases.
func (proxier *Proxier) shutdown() {
	proxier.mu.Lock()
	defer proxier.mu.Unlock()

	for serviceName, info := range proxier.serviceMap {
		proxier.stopProxy(serviceName, info)
	}
	proxier.cleanupStaleStickySessions()
	close(proxier.stopChan)
}

func (proxier *Proxier) isInitialized() bool {
	return atomic.LoadInt32(&proxier.initialized) > 0
}

// Sync is called to synchronize the proxier state to iptables as soon as possible.
func (proxier *Proxier) Sync() {
	proxier.syncRunner.Run()
}

func (proxier *Proxier) syncProxyRules() {
	start := time.Now()
	defer func() {
		klog.V(4).InfoS("Userspace syncProxyRules complete", "elapsed", time.Since(start))
	}()

	// don't sync rules till we've received services and endpoints
	if !proxier.isInitialized() {
		klog.V(2).InfoS("Not syncing userspace proxy until Services and Endpoints have been received from master")
		return
	}

	if err := iptablesInit(proxier.iptables); err != nil {
		klog.ErrorS(err, "Failed to ensure iptables")
	}

	proxier.serviceChangesLock.Lock()
	changes := proxier.serviceChanges
	proxier.serviceChanges = make(map[types.NamespacedName]*serviceChange)
	proxier.serviceChangesLock.Unlock()

	proxier.mu.Lock()
	defer proxier.mu.Unlock()

	klog.V(4).InfoS("userspace proxy: processing service events", "count", len(changes))
	for _, change := range changes {
		existingPorts := proxier.mergeService(change.current)
		proxier.unmergeService(change.previous, existingPorts)
	}

	proxier.localAddrs = utilproxy.GetLocalAddrSet()

	proxier.ensurePortals()
	proxier.cleanupStaleStickySessions()
}

// SyncLoop runs periodic work.  This is expected to run as a goroutine or as the main loop of the app.  It does not return.
func (proxier *Proxier) SyncLoop() {
	proxier.syncRunner.Loop(proxier.stopChan)
}

// Ensure that portals exist for all services.
func (proxier *Proxier) ensurePortals() {
	// NB: This does not remove rules that should not be present.
	for name, info := range proxier.serviceMap {
		err := proxier.openPortal(name, info)
		if err != nil {
			klog.ErrorS(err, "Failed to ensure portal", "servicePortName", name)
		}
	}
}

// clean up any stale sticky session records in the hash map.
func (proxier *Proxier) cleanupStaleStickySessions() {
	for name := range proxier.serviceMap {
		proxier.loadBalancer.CleanupStaleStickySessions(name)
	}
}

func (proxier *Proxier) stopProxy(service proxy.ServicePortName, info *ServiceInfo) error {
	delete(proxier.serviceMap, service)
	info.setAlive(false)
	err := info.socket.Close()
	port := info.socket.ListenPort()
	proxier.proxyPorts.Release(port)
	return err
}

func (proxier *Proxier) getServiceInfo(service proxy.ServicePortName) (*ServiceInfo, bool) {
	proxier.mu.Lock()
	defer proxier.mu.Unlock()
	info, ok := proxier.serviceMap[service]
	return info, ok
}

// addServiceOnPortInternal starts listening for a new service, returning the ServiceInfo.
// Pass proxyPort=0 to allocate a random port. The timeout only applies to UDP
// connections, for now.
func (proxier *Proxier) addServiceOnPortInternal(service proxy.ServicePortName, protocol v1.Protocol, proxyPort int, timeout time.Duration) (*ServiceInfo, error) {
	sock, err := proxier.makeProxySocket(protocol, proxier.listenIP, proxyPort)
	if err != nil {
		return nil, err
	}
	_, portStr, err := net.SplitHostPort(sock.Addr().String())
	if err != nil {
		sock.Close()
		return nil, err
	}
	portNum, err := strconv.Atoi(portStr)
	if err != nil {
		sock.Close()
		return nil, err
	}
	si := &ServiceInfo{
		Timeout:             timeout,
		ActiveClients:       newClientCache(),
		isAliveAtomic:       1,
		proxyPort:           portNum,
		protocol:            protocol,
		socket:              sock,
		sessionAffinityType: v1.ServiceAffinityNone, // default
	}
	proxier.serviceMap[service] = si

	klog.V(2).InfoS("Proxying for service", "service", service, "protocol", protocol, "portNum", portNum)
	go func() {
		defer runtime.HandleCrash()
		sock.ProxyLoop(service, si, proxier.loadBalancer)
	}()

	return si, nil
}

func (proxier *Proxier) cleanupPortalAndProxy(serviceName proxy.ServicePortName, info *ServiceInfo) error {
	if err := proxier.closePortal(serviceName, info); err != nil {
		return fmt.Errorf("Failed to close portal for %q: %v", serviceName, err)
	}
	if err := proxier.stopProxy(serviceName, info); err != nil {
		return fmt.Errorf("Failed to stop service %q: %v", serviceName, err)
	}
	return nil
}

func (proxier *Proxier) mergeService(service *v1.Service) sets.String {
	if service == nil {
		return nil
	}
	if utilproxy.ShouldSkipService(service) {
		return nil
	}
	existingPorts := sets.NewString()
	svcName := types.NamespacedName{Namespace: service.Namespace, Name: service.Name}
	for i := range service.Spec.Ports {
		servicePort := &service.Spec.Ports[i]
		serviceName := proxy.ServicePortName{NamespacedName: svcName, Port: servicePort.Name}
		existingPorts.Insert(servicePort.Name)
		info, exists := proxier.serviceMap[serviceName]
		// TODO: check health of the socket? What if ProxyLoop exited?
		if exists && sameConfig(info, service, servicePort) {
			// Nothing changed.
			continue
		}
		if exists {
			klog.V(4).InfoS("Something changed for service: stopping it", "serviceName", serviceName)
			if err := proxier.cleanupPortalAndProxy(serviceName, info); err != nil {
				klog.ErrorS(err, "Failed to cleanup portal and proxy")
			}
			info.setFinished()
		}
		proxyPort, err := proxier.proxyPorts.AllocateNext()
		if err != nil {
			klog.ErrorS(err, "Failed to allocate proxy port", "serviceName", serviceName)
			continue
		}

		serviceIP := net.ParseIP(service.Spec.ClusterIP)
		klog.V(1).InfoS("Adding new service", "serviceName", serviceName, "addr", net.JoinHostPort(serviceIP.String(), strconv.Itoa(int(servicePort.Port))), "protocol", servicePort.Protocol)
		info, err = proxier.addServiceOnPortInternal(serviceName, servicePort.Protocol, proxyPort, proxier.udpIdleTimeout)
		if err != nil {
			klog.ErrorS(err, "Failed to start proxy", "serviceName", serviceName)
			continue
		}
		info.portal.ip = serviceIP
		info.portal.port = int(servicePort.Port)
		info.externalIPs = service.Spec.ExternalIPs
		// Deep-copy in case the service instance changes
		info.loadBalancerStatus = *service.Status.LoadBalancer.DeepCopy()
		info.nodePort = int(servicePort.NodePort)
		info.sessionAffinityType = service.Spec.SessionAffinity
		// Kube-apiserver side guarantees SessionAffinityConfig won't be nil when session affinity type is ClientIP
		if service.Spec.SessionAffinity == v1.ServiceAffinityClientIP {
			info.stickyMaxAgeSeconds = int(*service.Spec.SessionAffinityConfig.ClientIP.TimeoutSeconds)
		}

		klog.V(4).InfoS("Record serviceInfo", "serviceInfo", info)

		if err := proxier.openPortal(serviceName, info); err != nil {
			klog.ErrorS(err, "Failed to open portal", "serviceName", serviceName)
		}
		proxier.loadBalancer.NewService(serviceName, info.sessionAffinityType, info.stickyMaxAgeSeconds)

		info.setStarted()
	}

	return existingPorts
}

func (proxier *Proxier) unmergeService(service *v1.Service, existingPorts sets.String) {
	if service == nil {
		return
	}

	if utilproxy.ShouldSkipService(service) {
		return
	}
	staleUDPServices := sets.NewString()
	svcName := types.NamespacedName{Namespace: service.Namespace, Name: service.Name}
	for i := range service.Spec.Ports {
		servicePort := &service.Spec.Ports[i]
		if existingPorts.Has(servicePort.Name) {
			continue
		}
		serviceName := proxy.ServicePortName{NamespacedName: svcName, Port: servicePort.Name}

		klog.V(1).InfoS("Stopping service", "serviceName", serviceName)
		info, exists := proxier.serviceMap[serviceName]
		if !exists {
			klog.ErrorS(nil, "Service is being removed but doesn't exist", "serviceName", serviceName)
			continue
		}

		if proxier.serviceMap[serviceName].protocol == v1.ProtocolUDP {
			staleUDPServices.Insert(proxier.serviceMap[serviceName].portal.ip.String())
		}

		if err := proxier.cleanupPortalAndProxy(serviceName, info); err != nil {
			klog.ErrorS(err, "Clean up portal and proxy")
		}
		proxier.loadBalancer.DeleteService(serviceName)
		info.setFinished()
	}
	for _, svcIP := range staleUDPServices.UnsortedList() {
		if err := conntrack.ClearEntriesForIP(proxier.exec, svcIP, v1.ProtocolUDP); err != nil {
			klog.ErrorS(err, "Failed to delete stale service IP connections", "ip", svcIP)
		}
	}
}

func (proxier *Proxier) serviceChange(previous, current *v1.Service, detail string) {
	var svcName types.NamespacedName
	if current != nil {
		svcName = types.NamespacedName{Namespace: current.Namespace, Name: current.Name}
	} else {
		svcName = types.NamespacedName{Namespace: previous.Namespace, Name: previous.Name}
	}
	klog.V(4).InfoS("Record service change", "action", detail, "svcName", svcName)

	proxier.serviceChangesLock.Lock()
	defer proxier.serviceChangesLock.Unlock()

	change, exists := proxier.serviceChanges[svcName]
	if !exists {
		// change.previous is only set for new changes. We must keep
		// the oldest service info (or nil) because correct unmerging
		// depends on the next update/del after a merge, not subsequent
		// updates.
		change = &serviceChange{previous: previous}
		proxier.serviceChanges[svcName] = change
	}

	// Always use the most current service (or nil) as change.current
	change.current = current

	if reflect.DeepEqual(change.previous, change.current) {
		// collapsed change had no effect
		delete(proxier.serviceChanges, svcName)
	} else if proxier.isInitialized() {
		// change will have an effect, ask the proxy to sync
		proxier.syncRunner.Run()
	}
}

// OnServiceAdd is called whenever creation of new service object
// is observed.
func (proxier *Proxier) OnServiceAdd(service *v1.Service) {
	proxier.serviceChange(nil, service, "OnServiceAdd")
}

// OnServiceUpdate is called whenever modification of an existing
// service object is observed.
func (proxier *Proxier) OnServiceUpdate(oldService, service *v1.Service) {
	proxier.serviceChange(oldService, service, "OnServiceUpdate")
}

// OnServiceDelete is called whenever deletion of an existing service
// object is observed.
func (proxier *Proxier) OnServiceDelete(service *v1.Service) {
	proxier.serviceChange(service, nil, "OnServiceDelete")
}

// OnServiceSynced is called once all the initial event handlers were
// called and the state is fully propagated to local cache.
func (proxier *Proxier) OnServiceSynced() {
	klog.V(2).InfoS("Userspace OnServiceSynced")

	// Mark services as initialized and (if endpoints are already
	// initialized) the entire proxy as initialized
	atomic.StoreInt32(&proxier.servicesSynced, 1)
	if atomic.LoadInt32(&proxier.endpointsSynced) > 0 {
		atomic.StoreInt32(&proxier.initialized, 1)
	}

	// Must sync from a goroutine to avoid blocking the
	// service event handler on startup with large numbers
	// of initial objects
	go proxier.syncProxyRules()
}

// OnEndpointsAdd is called whenever creation of new endpoints object
// is observed.
func (proxier *Proxier) OnEndpointsAdd(endpoints *v1.Endpoints) {
	proxier.loadBalancer.OnEndpointsAdd(endpoints)
}

// OnEndpointsUpdate is called whenever modification of an existing
// endpoints object is observed.
func (proxier *Proxier) OnEndpointsUpdate(oldEndpoints, endpoints *v1.Endpoints) {
	proxier.loadBalancer.OnEndpointsUpdate(oldEndpoints, endpoints)
}

// OnEndpointsDelete is called whenever deletion of an existing endpoints
// object is observed.
func (proxier *Proxier) OnEndpointsDelete(endpoints *v1.Endpoints) {
	proxier.loadBalancer.OnEndpointsDelete(endpoints)
}

// OnEndpointsSynced is called once all the initial event handlers were
// called and the state is fully propagated to local cache.
func (proxier *Proxier) OnEndpointsSynced() {
	klog.V(2).InfoS("Userspace OnEndpointsSynced")
	proxier.loadBalancer.OnEndpointsSynced()

	// Mark endpoints as initialized and (if services are already
	// initialized) the entire proxy as initialized
	atomic.StoreInt32(&proxier.endpointsSynced, 1)
	if atomic.LoadInt32(&proxier.servicesSynced) > 0 {
		atomic.StoreInt32(&proxier.initialized, 1)
	}

	// Must sync from a goroutine to avoid blocking the
	// service event handler on startup with large numbers
	// of initial objects
	go proxier.syncProxyRules()
}

func sameConfig(info *ServiceInfo, service *v1.Service, port *v1.ServicePort) bool {
	if info.protocol != port.Protocol || info.portal.port != int(port.Port) || info.nodePort != int(port.NodePort) {
		return false
	}
	if !info.portal.ip.Equal(net.ParseIP(service.Spec.ClusterIP)) {
		return false
	}
	if !ipsEqual(info.externalIPs, service.Spec.ExternalIPs) {
		return false
	}
	if !servicehelper.LoadBalancerStatusEqual(&info.loadBalancerStatus, &service.Status.LoadBalancer) {
		return false
	}
	if info.sessionAffinityType != service.Spec.SessionAffinity {
		return false
	}
	return true
}

func ipsEqual(lhs, rhs []string) bool {
	if len(lhs) != len(rhs) {
		return false
	}
	for i := range lhs {
		if lhs[i] != rhs[i] {
			return false
		}
	}
	return true
}

func (proxier *Proxier) openPortal(service proxy.ServicePortName, info *ServiceInfo) error {
	err := proxier.openOnePortal(info.portal, info.protocol, proxier.listenIP, info.proxyPort, service)
	if err != nil {
		return err
	}
	for _, publicIP := range info.externalIPs {
		err = proxier.openOnePortal(portal{net.ParseIP(publicIP), info.portal.port, true}, info.protocol, proxier.listenIP, info.proxyPort, service)
		if err != nil {
			return err
		}
	}
	for _, ingress := range info.loadBalancerStatus.Ingress {
		if ingress.IP != "" {
			err = proxier.openOnePortal(portal{net.ParseIP(ingress.IP), info.portal.port, false}, info.protocol, proxier.listenIP, info.proxyPort, service)
			if err != nil {
				return err
			}
		}
	}
	if info.nodePort != 0 {
		err = proxier.openNodePort(info.nodePort, info.protocol, proxier.listenIP, info.proxyPort, service)
		if err != nil {
			return err
		}
	}
	return nil
}

func (proxier *Proxier) openOnePortal(portal portal, protocol v1.Protocol, proxyIP net.IP, proxyPort int, name proxy.ServicePortName) error {
	if proxier.localAddrs.Has(portal.ip) {
		err := proxier.claimNodePort(portal.ip, portal.port, protocol, name)
		if err != nil {
			return err
		}
	}

	// Handle traffic from containers.
	args := proxier.iptablesContainerPortalArgs(portal.ip, portal.isExternal, false, portal.port, protocol, proxyIP, proxyPort, name)
	portalAddress := net.JoinHostPort(portal.ip.String(), strconv.Itoa(portal.port))
	existed, err := proxier.iptables.EnsureRule(iptables.Append, iptables.TableNAT, iptablesContainerPortalChain, args...)
	if err != nil {
		klog.ErrorS(err, "Failed to install iptables rule for service", "chain", iptablesContainerPortalChain, "servicePortName", name, "args", args)
		return err
	}
	if !existed {
		klog.V(3).InfoS("Opened iptables from-containers portal for service", "servicePortName", name, "protocol", protocol, "portalAddress", portalAddress)
	}
	if portal.isExternal {
		args := proxier.iptablesContainerPortalArgs(portal.ip, false, true, portal.port, protocol, proxyIP, proxyPort, name)
		existed, err := proxier.iptables.EnsureRule(iptables.Append, iptables.TableNAT, iptablesContainerPortalChain, args...)
		if err != nil {
			klog.ErrorS(err, "Failed to install iptables rule that opens service for local traffic", "chain", iptablesContainerPortalChain, "servicePortName", name, "args", args)
			return err
		}
		if !existed {
			klog.V(3).InfoS("Opened iptables from-containers portal for service for local traffic", "servicePortName", name, "protocol", protocol, "portalAddress", portalAddress)
		}

		args = proxier.iptablesHostPortalArgs(portal.ip, true, portal.port, protocol, proxyIP, proxyPort, name)
		existed, err = proxier.iptables.EnsureRule(iptables.Append, iptables.TableNAT, iptablesHostPortalChain, args...)
		if err != nil {
			klog.ErrorS(err, "Failed to install iptables rule for service for dst-local traffic", "chain", iptablesHostPortalChain, "servicePortName", name)
			return err
		}
		if !existed {
			klog.V(3).InfoS("Opened iptables from-host portal for service for dst-local traffic", "servicePortName", name, "protocol", protocol, "portalAddress", portalAddress)
		}
		return nil
	}

	// Handle traffic from the host.
	args = proxier.iptablesHostPortalArgs(portal.ip, false, portal.port, protocol, proxyIP, proxyPort, name)
	existed, err = proxier.iptables.EnsureRule(iptables.Append, iptables.TableNAT, iptablesHostPortalChain, args...)
	if err != nil {
		klog.ErrorS(err, "Failed to install iptables rule for service", "chain", iptablesHostPortalChain, "servicePortName", name)
		return err
	}
	if !existed {
		klog.V(3).InfoS("Opened iptables from-host portal for service", "servicePortName", name, "protocol", protocol, "portalAddress", portalAddress)
	}
	return nil
}

// Marks a port as being owned by a particular service, or returns error if already claimed.
// Idempotent: reclaiming with the same owner is not an error
func (proxier *Proxier) claimNodePort(ip net.IP, port int, protocol v1.Protocol, owner proxy.ServicePortName) error {
	proxier.portMapMutex.Lock()
	defer proxier.portMapMutex.Unlock()

	// TODO: We could pre-populate some reserved ports into portMap and/or blacklist some well-known ports

	key := portMapKey{ip: ip.String(), port: port, protocol: protocol}
	existing, found := proxier.portMap[key]
	if !found {
		// Hold the actual port open, even though we use iptables to redirect
		// it.  This ensures that a) it's safe to take and b) that stays true.
		// NOTE: We should not need to have a real listen()ing socket - bind()
		// should be enough, but I can't figure out a way to e2e test without
		// it.  Tools like 'ss' and 'netstat' do not show sockets that are
		// bind()ed but not listen()ed, and at least the default debian netcat
		// has no way to avoid about 10 seconds of retries.
		socket, err := proxier.makeProxySocket(protocol, ip, port)
		if err != nil {
			return fmt.Errorf("can't open node port for %s: %v", key.String(), err)
		}
		proxier.portMap[key] = &portMapValue{owner: owner, socket: socket}
		klog.V(2).InfoS("Claimed local port", "port", key.String())
		return nil
	}
	if existing.owner == owner {
		// We are idempotent
		return nil
	}
	return fmt.Errorf("Port conflict detected on port %s.  %v vs %v", key.String(), owner, existing)
}

// Release a claim on a port.  Returns an error if the owner does not match the claim.
// Tolerates release on an unclaimed port, to simplify .
func (proxier *Proxier) releaseNodePort(ip net.IP, port int, protocol v1.Protocol, owner proxy.ServicePortName) error {
	proxier.portMapMutex.Lock()
	defer proxier.portMapMutex.Unlock()

	key := portMapKey{ip: ip.String(), port: port, protocol: protocol}
	existing, found := proxier.portMap[key]
	if !found {
		// We tolerate this, it happens if we are cleaning up a failed allocation
		klog.InfoS("Ignoring release on unowned port", "port", key)
		return nil
	}
	if existing.owner != owner {
		return fmt.Errorf("Port conflict detected on port %v (unowned unlock).  %v vs %v", key, owner, existing)
	}
	delete(proxier.portMap, key)
	existing.socket.Close()
	return nil
}

func (proxier *Proxier) openNodePort(nodePort int, protocol v1.Protocol, proxyIP net.IP, proxyPort int, name proxy.ServicePortName) error {
	// TODO: Do we want to allow containers to access public services?  Probably yes.
	// TODO: We could refactor this to be the same code as portal, but with IP == nil

	err := proxier.claimNodePort(nil, nodePort, protocol, name)
	if err != nil {
		return err
	}

	// Handle traffic from containers.
	args := proxier.iptablesContainerPortalArgs(nil, false, false, nodePort, protocol, proxyIP, proxyPort, name)
	existed, err := proxier.iptables.EnsureRule(iptables.Append, iptables.TableNAT, iptablesContainerNodePortChain, args...)
	if err != nil {
		klog.ErrorS(err, "Failed to install iptables rule for service", "chain", iptablesContainerNodePortChain, "servicePortName", name)
		return err
	}
	if !existed {
		klog.InfoS("Opened iptables from-containers public port for service", "servicePortName", name, "protocol", protocol, "nodePort", nodePort)
	}

	// Handle traffic from the host.
	args = proxier.iptablesHostNodePortArgs(nodePort, protocol, proxyIP, proxyPort, name)
	existed, err = proxier.iptables.EnsureRule(iptables.Append, iptables.TableNAT, iptablesHostNodePortChain, args...)
	if err != nil {
		klog.ErrorS(err, "Failed to install iptables rule for service", "chain", iptablesHostNodePortChain, "servicePortName", name)
		return err
	}
	if !existed {
		klog.InfoS("Opened iptables from-host public port for service", "servicePortName", name, "protocol", protocol, "nodePort", nodePort)
	}

	args = proxier.iptablesNonLocalNodePortArgs(nodePort, protocol, proxyIP, proxyPort, name)
	existed, err = proxier.iptables.EnsureRule(iptables.Append, iptables.TableFilter, iptablesNonLocalNodePortChain, args...)
	if err != nil {
		klog.ErrorS(err, "Failed to install iptables rule for service", "chain", iptablesNonLocalNodePortChain, "servicePortName", name)
		return err
	}
	if !existed {
		klog.InfoS("Opened iptables from-non-local public port for service", "servicePortName", name, "protocol", protocol, "nodePort", nodePort)
	}

	return nil
}

func (proxier *Proxier) closePortal(service proxy.ServicePortName, info *ServiceInfo) error {
	// Collect errors and report them all at the end.
	el := proxier.closeOnePortal(info.portal, info.protocol, proxier.listenIP, info.proxyPort, service)
	for _, publicIP := range info.externalIPs {
		el = append(el, proxier.closeOnePortal(portal{net.ParseIP(publicIP), info.portal.port, true}, info.protocol, proxier.listenIP, info.proxyPort, service)...)
	}
	for _, ingress := range info.loadBalancerStatus.Ingress {
		if ingress.IP != "" {
			el = append(el, proxier.closeOnePortal(portal{net.ParseIP(ingress.IP), info.portal.port, false}, info.protocol, proxier.listenIP, info.proxyPort, service)...)
		}
	}
	if info.nodePort != 0 {
		el = append(el, proxier.closeNodePort(info.nodePort, info.protocol, proxier.listenIP, info.proxyPort, service)...)
	}
	if len(el) == 0 {
		klog.V(3).InfoS("Closed iptables portals for service", "servicePortName", service)
	} else {
		klog.ErrorS(nil, "Some errors closing iptables portals for service", "servicePortName", service)
	}
	return utilerrors.NewAggregate(el)
}

func (proxier *Proxier) closeOnePortal(portal portal, protocol v1.Protocol, proxyIP net.IP, proxyPort int, name proxy.ServicePortName) []error {
	el := []error{}
	if proxier.localAddrs.Has(portal.ip) {
		if err := proxier.releaseNodePort(portal.ip, portal.port, protocol, name); err != nil {
			el = append(el, err)
		}
	}

	// Handle traffic from containers.
	args := proxier.iptablesContainerPortalArgs(portal.ip, portal.isExternal, false, portal.port, protocol, proxyIP, proxyPort, name)
	if err := proxier.iptables.DeleteRule(iptables.TableNAT, iptablesContainerPortalChain, args...); err != nil {
		klog.ErrorS(err, "Failed to delete iptables rule for service", "chain", iptablesContainerPortalChain, "servicePortName", name)
		el = append(el, err)
	}

	if portal.isExternal {
		args := proxier.iptablesContainerPortalArgs(portal.ip, false, true, portal.port, protocol, proxyIP, proxyPort, name)
		if err := proxier.iptables.DeleteRule(iptables.TableNAT, iptablesContainerPortalChain, args...); err != nil {
			klog.ErrorS(err, "Failed to delete iptables rule for service", "chain", iptablesContainerPortalChain, "servicePortName", name)
			el = append(el, err)
		}

		args = proxier.iptablesHostPortalArgs(portal.ip, true, portal.port, protocol, proxyIP, proxyPort, name)
		if err := proxier.iptables.DeleteRule(iptables.TableNAT, iptablesHostPortalChain, args...); err != nil {
			klog.ErrorS(err, "Failed to delete iptables rule for service", "chain", iptablesHostPortalChain, "servicePortName", name)
			el = append(el, err)
		}
		return el
	}

	// Handle traffic from the host (portalIP is not external).
	args = proxier.iptablesHostPortalArgs(portal.ip, false, portal.port, protocol, proxyIP, proxyPort, name)
	if err := proxier.iptables.DeleteRule(iptables.TableNAT, iptablesHostPortalChain, args...); err != nil {
		klog.ErrorS(err, "Failed to delete iptables rule for service", "chain", iptablesHostPortalChain, "servicePortName", name)
		el = append(el, err)
	}

	return el
}

func (proxier *Proxier) closeNodePort(nodePort int, protocol v1.Protocol, proxyIP net.IP, proxyPort int, name proxy.ServicePortName) []error {
	el := []error{}

	// Handle traffic from containers.
	args := proxier.iptablesContainerPortalArgs(nil, false, false, nodePort, protocol, proxyIP, proxyPort, name)
	if err := proxier.iptables.DeleteRule(iptables.TableNAT, iptablesContainerNodePortChain, args...); err != nil {
		klog.ErrorS(err, "Failed to delete iptables rule for service", "chain", iptablesContainerNodePortChain, "servicePortName", name)
		el = append(el, err)
	}

	// Handle traffic from the host.
	args = proxier.iptablesHostNodePortArgs(nodePort, protocol, proxyIP, proxyPort, name)
	if err := proxier.iptables.DeleteRule(iptables.TableNAT, iptablesHostNodePortChain, args...); err != nil {
		klog.ErrorS(err, "Failed to delete iptables rule for service", "chain", iptablesHostNodePortChain, "servicePortName", name)
		el = append(el, err)
	}

	// Handle traffic not local to the host
	args = proxier.iptablesNonLocalNodePortArgs(nodePort, protocol, proxyIP, proxyPort, name)
	if err := proxier.iptables.DeleteRule(iptables.TableFilter, iptablesNonLocalNodePortChain, args...); err != nil {
		klog.ErrorS(err, "Failed to delete iptables rule for service", "chain", iptablesNonLocalNodePortChain, "servicePortName", name)
		el = append(el, err)
	}

	if err := proxier.releaseNodePort(nil, nodePort, protocol, name); err != nil {
		el = append(el, err)
	}

	return el
}

// See comments in the *PortalArgs() functions for some details about why we
// use two chains for portals.
var iptablesContainerPortalChain iptables.Chain = "KUBE-PORTALS-CONTAINER"
var iptablesHostPortalChain iptables.Chain = "KUBE-PORTALS-HOST"

// Chains for NodePort services
var iptablesContainerNodePortChain iptables.Chain = "KUBE-NODEPORT-CONTAINER"
var iptablesHostNodePortChain iptables.Chain = "KUBE-NODEPORT-HOST"
var iptablesNonLocalNodePortChain iptables.Chain = "KUBE-NODEPORT-NON-LOCAL"

// Ensure that the iptables infrastructure we use is set up.  This can safely be called periodically.
func iptablesInit(ipt iptables.Interface) error {
	// TODO: There is almost certainly room for optimization here.  E.g. If
	// we knew the service-cluster-ip-range CIDR we could fast-track outbound packets not
	// destined for a service. There's probably more, help wanted.

	// Danger - order of these rules matters here:
	//
	// We match portal rules first, then NodePort rules.  For NodePort rules, we filter primarily on --dst-type LOCAL,
	// because we want to listen on all local addresses, but don't match internet traffic with the same dst port number.
	//
	// There is one complication (per thockin):
	// -m addrtype --dst-type LOCAL is what we want except that it is broken (by intent without foresight to our usecase)
	// on at least GCE. Specifically, GCE machines have a daemon which learns what external IPs are forwarded to that
	// machine, and configure a local route for that IP, making a match for --dst-type LOCAL when we don't want it to.
	// Removing the route gives correct behavior until the daemon recreates it.
	// Killing the daemon is an option, but means that any non-kubernetes use of the machine with external IP will be broken.
	//
	// This applies to IPs on GCE that are actually from a load-balancer; they will be categorized as LOCAL.
	// _If_ the chains were in the wrong order, and the LB traffic had dst-port == a NodePort on some other service,
	// the NodePort would take priority (incorrectly).
	// This is unlikely (and would only affect outgoing traffic from the cluster to the load balancer, which seems
	// doubly-unlikely), but we need to be careful to keep the rules in the right order.
	args := []string{ /* service-cluster-ip-range matching could go here */ }
	args = append(args, "-m", "comment", "--comment", "handle ClusterIPs; NOTE: this must be before the NodePort rules")
	if _, err := ipt.EnsureChain(iptables.TableNAT, iptablesContainerPortalChain); err != nil {
		return err
	}
	if _, err := ipt.EnsureRule(iptables.Prepend, iptables.TableNAT, iptables.ChainPrerouting, append(args, "-j", string(iptablesContainerPortalChain))...); err != nil {
		return err
	}
	if _, err := ipt.EnsureChain(iptables.TableNAT, iptablesHostPortalChain); err != nil {
		return err
	}
	if _, err := ipt.EnsureRule(iptables.Prepend, iptables.TableNAT, iptables.ChainOutput, append(args, "-j", string(iptablesHostPortalChain))...); err != nil {
		return err
	}

	// This set of rules matches broadly (addrtype & destination port), and therefore must come after the portal rules
	args = []string{"-m", "addrtype", "--dst-type", "LOCAL"}
	args = append(args, "-m", "comment", "--comment", "handle service NodePorts; NOTE: this must be the last rule in the chain")
	if _, err := ipt.EnsureChain(iptables.TableNAT, iptablesContainerNodePortChain); err != nil {
		return err
	}
	if _, err := ipt.EnsureRule(iptables.Append, iptables.TableNAT, iptables.ChainPrerouting, append(args, "-j", string(iptablesContainerNodePortChain))...); err != nil {
		return err
	}
	if _, err := ipt.EnsureChain(iptables.TableNAT, iptablesHostNodePortChain); err != nil {
		return err
	}
	if _, err := ipt.EnsureRule(iptables.Append, iptables.TableNAT, iptables.ChainOutput, append(args, "-j", string(iptablesHostNodePortChain))...); err != nil {
		return err
	}

	// Create a chain intended to explicitly allow non-local NodePort
	// traffic to work around default-deny iptables configurations
	// that would otherwise reject such traffic.
	args = []string{"-m", "comment", "--comment", "Ensure that non-local NodePort traffic can flow"}
	if _, err := ipt.EnsureChain(iptables.TableFilter, iptablesNonLocalNodePortChain); err != nil {
		return err
	}
	if _, err := ipt.EnsureRule(iptables.Prepend, iptables.TableFilter, iptables.ChainInput, append(args, "-j", string(iptablesNonLocalNodePortChain))...); err != nil {
		return err
	}

	// TODO: Verify order of rules.
	return nil
}

// Flush all of our custom iptables rules.
func iptablesFlush(ipt iptables.Interface) error {
	el := []error{}
	if err := ipt.FlushChain(iptables.TableNAT, iptablesContainerPortalChain); err != nil {
		el = append(el, err)
	}
	if err := ipt.FlushChain(iptables.TableNAT, iptablesHostPortalChain); err != nil {
		el = append(el, err)
	}
	if err := ipt.FlushChain(iptables.TableNAT, iptablesContainerNodePortChain); err != nil {
		el = append(el, err)
	}
	if err := ipt.FlushChain(iptables.TableNAT, iptablesHostNodePortChain); err != nil {
		el = append(el, err)
	}
	if err := ipt.FlushChain(iptables.TableFilter, iptablesNonLocalNodePortChain); err != nil {
		el = append(el, err)
	}
	if len(el) != 0 {
		klog.ErrorS(utilerrors.NewAggregate(el), "Some errors flushing old iptables portals")
	}
	return utilerrors.NewAggregate(el)
}

// Used below.
var zeroIPv4 = net.ParseIP("0.0.0.0")
var localhostIPv4 = net.ParseIP("127.0.0.1")

var zeroIPv6 = net.ParseIP("::")
var localhostIPv6 = net.ParseIP("::1")

// Build a slice of iptables args that are common to from-container and from-host portal rules.
func iptablesCommonPortalArgs(destIP net.IP, addPhysicalInterfaceMatch bool, addDstLocalMatch bool, destPort int, protocol v1.Protocol, service proxy.ServicePortName) []string {
	// This list needs to include all fields as they are eventually spit out
	// by iptables-save.  This is because some systems do not support the
	// 'iptables -C' arg, and so fall back on parsing iptables-save output.
	// If this does not match, it will not pass the check.  For example:
	// adding the /32 on the destination IP arg is not strictly required,
	// but causes this list to not match the final iptables-save output.
	// This is fragile and I hope one day we can stop supporting such old
	// iptables versions.
	args := []string{
		"-m", "comment",
		"--comment", service.String(),
		"-p", strings.ToLower(string(protocol)),
		"-m", strings.ToLower(string(protocol)),
		"--dport", fmt.Sprintf("%d", destPort),
	}

	if destIP != nil {
		args = append(args, "-d", utilproxy.ToCIDR(destIP))
	}

	if addPhysicalInterfaceMatch {
		args = append(args, "-m", "physdev", "!", "--physdev-is-in")
	}

	if addDstLocalMatch {
		args = append(args, "-m", "addrtype", "--dst-type", "LOCAL")
	}

	return args
}

// Build a slice of iptables args for a from-container portal rule.
func (proxier *Proxier) iptablesContainerPortalArgs(destIP net.IP, addPhysicalInterfaceMatch bool, addDstLocalMatch bool, destPort int, protocol v1.Protocol, proxyIP net.IP, proxyPort int, service proxy.ServicePortName) []string {
	args := iptablesCommonPortalArgs(destIP, addPhysicalInterfaceMatch, addDstLocalMatch, destPort, protocol, service)

	// This is tricky.
	//
	// If the proxy is bound (see Proxier.listenIP) to 0.0.0.0 ("any
	// interface") we want to use REDIRECT, which sends traffic to the
	// "primary address of the incoming interface" which means the container
	// bridge, if there is one.  When the response comes, it comes from that
	// same interface, so the NAT matches and the response packet is
	// correct.  This matters for UDP, since there is no per-connection port
	// number.
	//
	// The alternative would be to use DNAT, except that it doesn't work
	// (empirically):
	//   * DNAT to 127.0.0.1 = Packets just disappear - this seems to be a
	//     well-known limitation of iptables.
	//   * DNAT to eth0's IP = Response packets come from the bridge, which
	//     breaks the NAT, and makes things like DNS not accept them.  If
	//     this could be resolved, it would simplify all of this code.
	//
	// If the proxy is bound to a specific IP, then we have to use DNAT to
	// that IP.  Unlike the previous case, this works because the proxy is
	// ONLY listening on that IP, not the bridge.
	//
	// Why would anyone bind to an address that is not inclusive of
	// localhost?  Apparently some cloud environments have their public IP
	// exposed as a real network interface AND do not have firewalling.  We
	// don't want to expose everything out to the world.
	//
	// Unfortunately, I don't know of any way to listen on some (N > 1)
	// interfaces but not ALL interfaces, short of doing it manually, and
	// this is simpler than that.
	//
	// If the proxy is bound to localhost only, all of this is broken.  Not
	// allowed.
	if proxyIP.Equal(zeroIPv4) || proxyIP.Equal(zeroIPv6) {
		// TODO: Can we REDIRECT with IPv6?
		args = append(args, "-j", "REDIRECT", "--to-ports", fmt.Sprintf("%d", proxyPort))
	} else {
		// TODO: Can we DNAT with IPv6?
		args = append(args, "-j", "DNAT", "--to-destination", net.JoinHostPort(proxyIP.String(), strconv.Itoa(proxyPort)))
	}
	return args
}

// Build a slice of iptables args for a from-host portal rule.
func (proxier *Proxier) iptablesHostPortalArgs(destIP net.IP, addDstLocalMatch bool, destPort int, protocol v1.Protocol, proxyIP net.IP, proxyPort int, service proxy.ServicePortName) []string {
	args := iptablesCommonPortalArgs(destIP, false, addDstLocalMatch, destPort, protocol, service)

	// This is tricky.
	//
	// If the proxy is bound (see Proxier.listenIP) to 0.0.0.0 ("any
	// interface") we want to do the same as from-container traffic and use
	// REDIRECT.  Except that it doesn't work (empirically).  REDIRECT on
	// local packets sends the traffic to localhost (special case, but it is
	// documented) but the response comes from the eth0 IP (not sure why,
	// truthfully), which makes DNS unhappy.
	//
	// So we have to use DNAT.  DNAT to 127.0.0.1 can't work for the same
	// reason.
	//
	// So we do our best to find an interface that is not a loopback and
	// DNAT to that.  This works (again, empirically).
	//
	// If the proxy is bound to a specific IP, then we have to use DNAT to
	// that IP.  Unlike the previous case, this works because the proxy is
	// ONLY listening on that IP, not the bridge.
	//
	// If the proxy is bound to localhost only, this should work, but we
	// don't allow it for now.
	if proxyIP.Equal(zeroIPv4) || proxyIP.Equal(zeroIPv6) {
		proxyIP = proxier.hostIP
	}
	// TODO: Can we DNAT with IPv6?
	args = append(args, "-j", "DNAT", "--to-destination", net.JoinHostPort(proxyIP.String(), strconv.Itoa(proxyPort)))
	return args
}

// Build a slice of iptables args for a from-host public-port rule.
// See iptablesHostPortalArgs
// TODO: Should we just reuse iptablesHostPortalArgs?
func (proxier *Proxier) iptablesHostNodePortArgs(nodePort int, protocol v1.Protocol, proxyIP net.IP, proxyPort int, service proxy.ServicePortName) []string {
	args := iptablesCommonPortalArgs(nil, false, false, nodePort, protocol, service)

	if proxyIP.Equal(zeroIPv4) || proxyIP.Equal(zeroIPv6) {
		proxyIP = proxier.hostIP
	}
	// TODO: Can we DNAT with IPv6?
	args = append(args, "-j", "DNAT", "--to-destination", net.JoinHostPort(proxyIP.String(), strconv.Itoa(proxyPort)))
	return args
}

// Build a slice of iptables args for an from-non-local public-port rule.
func (proxier *Proxier) iptablesNonLocalNodePortArgs(nodePort int, protocol v1.Protocol, proxyIP net.IP, proxyPort int, service proxy.ServicePortName) []string {
	args := iptablesCommonPortalArgs(nil, false, false, proxyPort, protocol, service)
	args = append(args, "-m", "state", "--state", "NEW", "-j", "ACCEPT")
	return args
}

func isTooManyFDsError(err error) bool {
	return strings.Contains(err.Error(), "too many open files")
}

func isClosedError(err error) bool {
	// A brief discussion about handling closed error here:
	// https://code.google.com/p/go/issues/detail?id=4373#c14
	// TODO: maybe create a stoppable TCP listener that returns a StoppedError
	return strings.HasSuffix(err.Error(), "use of closed network connection")
}
