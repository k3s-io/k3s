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

package proxy

import (
	"fmt"
	"net"
	"reflect"
	"strings"
	"sync"

	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	apiservice "k8s.io/kubernetes/pkg/api/v1/service"
	"k8s.io/kubernetes/pkg/features"
	"k8s.io/kubernetes/pkg/proxy/metrics"
	utilproxy "k8s.io/kubernetes/pkg/proxy/util"
)

// BaseServiceInfo contains base information that defines a service.
// This could be used directly by proxier while processing services,
// or can be used for constructing a more specific ServiceInfo struct
// defined by the proxier if needed.
type BaseServiceInfo struct {
	clusterIP                net.IP
	port                     int
	protocol                 v1.Protocol
	nodePort                 int
	loadBalancerStatus       v1.LoadBalancerStatus
	sessionAffinityType      v1.ServiceAffinity
	stickyMaxAgeSeconds      int
	externalIPs              []string
	loadBalancerSourceRanges []string
	healthCheckNodePort      int
	nodeLocalExternal        bool
	nodeLocalInternal        bool
	internalTrafficPolicy    *v1.ServiceInternalTrafficPolicyType
	hintsAnnotation          string
}

var _ ServicePort = &BaseServiceInfo{}

// String is part of ServicePort interface.
func (info *BaseServiceInfo) String() string {
	return fmt.Sprintf("%s:%d/%s", info.clusterIP, info.port, info.protocol)
}

// ClusterIP is part of ServicePort interface.
func (info *BaseServiceInfo) ClusterIP() net.IP {
	return info.clusterIP
}

// Port is part of ServicePort interface.
func (info *BaseServiceInfo) Port() int {
	return info.port
}

// SessionAffinityType is part of the ServicePort interface.
func (info *BaseServiceInfo) SessionAffinityType() v1.ServiceAffinity {
	return info.sessionAffinityType
}

// StickyMaxAgeSeconds is part of the ServicePort interface
func (info *BaseServiceInfo) StickyMaxAgeSeconds() int {
	return info.stickyMaxAgeSeconds
}

// Protocol is part of ServicePort interface.
func (info *BaseServiceInfo) Protocol() v1.Protocol {
	return info.protocol
}

// LoadBalancerSourceRanges is part of ServicePort interface
func (info *BaseServiceInfo) LoadBalancerSourceRanges() []string {
	return info.loadBalancerSourceRanges
}

// HealthCheckNodePort is part of ServicePort interface.
func (info *BaseServiceInfo) HealthCheckNodePort() int {
	return info.healthCheckNodePort
}

// NodePort is part of the ServicePort interface.
func (info *BaseServiceInfo) NodePort() int {
	return info.nodePort
}

// ExternalIPStrings is part of ServicePort interface.
func (info *BaseServiceInfo) ExternalIPStrings() []string {
	return info.externalIPs
}

// LoadBalancerIPStrings is part of ServicePort interface.
func (info *BaseServiceInfo) LoadBalancerIPStrings() []string {
	var ips []string
	for _, ing := range info.loadBalancerStatus.Ingress {
		ips = append(ips, ing.IP)
	}
	return ips
}

// NodeLocalExternal is part of ServicePort interface.
func (info *BaseServiceInfo) NodeLocalExternal() bool {
	return info.nodeLocalExternal
}

// NodeLocalInternal is part of ServicePort interface
func (info *BaseServiceInfo) NodeLocalInternal() bool {
	return info.nodeLocalInternal
}

// InternalTrafficPolicy is part of ServicePort interface
func (info *BaseServiceInfo) InternalTrafficPolicy() *v1.ServiceInternalTrafficPolicyType {
	return info.internalTrafficPolicy
}

// HintsAnnotation is part of ServicePort interface.
func (info *BaseServiceInfo) HintsAnnotation() string {
	return info.hintsAnnotation
}

func (sct *ServiceChangeTracker) newBaseServiceInfo(port *v1.ServicePort, service *v1.Service) *BaseServiceInfo {
	nodeLocalExternal := false
	if apiservice.RequestsOnlyLocalTraffic(service) {
		nodeLocalExternal = true
	}
	nodeLocalInternal := false
	if utilfeature.DefaultFeatureGate.Enabled(features.ServiceInternalTrafficPolicy) {
		nodeLocalInternal = apiservice.RequestsOnlyLocalTrafficForInternal(service)
	}
	var stickyMaxAgeSeconds int
	if service.Spec.SessionAffinity == v1.ServiceAffinityClientIP {
		// Kube-apiserver side guarantees SessionAffinityConfig won't be nil when session affinity type is ClientIP
		stickyMaxAgeSeconds = int(*service.Spec.SessionAffinityConfig.ClientIP.TimeoutSeconds)
	}

	clusterIP := utilproxy.GetClusterIPByFamily(sct.ipFamily, service)
	info := &BaseServiceInfo{
		clusterIP:             net.ParseIP(clusterIP),
		port:                  int(port.Port),
		protocol:              port.Protocol,
		nodePort:              int(port.NodePort),
		sessionAffinityType:   service.Spec.SessionAffinity,
		stickyMaxAgeSeconds:   stickyMaxAgeSeconds,
		nodeLocalExternal:     nodeLocalExternal,
		nodeLocalInternal:     nodeLocalInternal,
		internalTrafficPolicy: service.Spec.InternalTrafficPolicy,
		hintsAnnotation:       service.Annotations[v1.AnnotationTopologyAwareHints],
	}

	loadBalancerSourceRanges := make([]string, len(service.Spec.LoadBalancerSourceRanges))
	for i, sourceRange := range service.Spec.LoadBalancerSourceRanges {
		loadBalancerSourceRanges[i] = strings.TrimSpace(sourceRange)
	}
	// filter external ips, source ranges and ingress ips
	// prior to dual stack services, this was considered an error, but with dual stack
	// services, this is actually expected. Hence we downgraded from reporting by events
	// to just log lines with high verbosity

	ipFamilyMap := utilproxy.MapIPsByIPFamily(service.Spec.ExternalIPs)
	info.externalIPs = ipFamilyMap[sct.ipFamily]

	// Log the IPs not matching the ipFamily
	if ips, ok := ipFamilyMap[utilproxy.OtherIPFamily(sct.ipFamily)]; ok && len(ips) > 0 {
		klog.V(4).Infof("service change tracker(%v) ignored the following external IPs(%s) for service %v/%v as they don't match IPFamily", sct.ipFamily, strings.Join(ips, ","), service.Namespace, service.Name)
	}

	ipFamilyMap = utilproxy.MapCIDRsByIPFamily(loadBalancerSourceRanges)
	info.loadBalancerSourceRanges = ipFamilyMap[sct.ipFamily]
	// Log the CIDRs not matching the ipFamily
	if cidrs, ok := ipFamilyMap[utilproxy.OtherIPFamily(sct.ipFamily)]; ok && len(cidrs) > 0 {
		klog.V(4).Infof("service change tracker(%v) ignored the following load balancer source ranges(%s) for service %v/%v as they don't match IPFamily", sct.ipFamily, strings.Join(cidrs, ","), service.Namespace, service.Name)
	}

	// Obtain Load Balancer Ingress IPs
	var ips []string
	for _, ing := range service.Status.LoadBalancer.Ingress {
		if ing.IP != "" {
			ips = append(ips, ing.IP)
		}
	}

	if len(ips) > 0 {
		ipFamilyMap = utilproxy.MapIPsByIPFamily(ips)

		if ipList, ok := ipFamilyMap[utilproxy.OtherIPFamily(sct.ipFamily)]; ok && len(ipList) > 0 {
			klog.V(4).Infof("service change tracker(%v) ignored the following load balancer(%s) ingress ips for service %v/%v as they don't match IPFamily", sct.ipFamily, strings.Join(ipList, ","), service.Namespace, service.Name)

		}
		// Create the LoadBalancerStatus with the filtered IPs
		for _, ip := range ipFamilyMap[sct.ipFamily] {
			info.loadBalancerStatus.Ingress = append(info.loadBalancerStatus.Ingress, v1.LoadBalancerIngress{IP: ip})
		}
	}

	if apiservice.NeedsHealthCheck(service) {
		p := service.Spec.HealthCheckNodePort
		if p == 0 {
			klog.Errorf("Service %s/%s has no healthcheck nodeport", service.Namespace, service.Name)
		} else {
			info.healthCheckNodePort = int(p)
		}
	}

	return info
}

type makeServicePortFunc func(*v1.ServicePort, *v1.Service, *BaseServiceInfo) ServicePort

// This handler is invoked by the apply function on every change. This function should not modify the
// ServiceMap's but just use the changes for any Proxier specific cleanup.
type processServiceMapChangeFunc func(previous, current ServiceMap)

// serviceChange contains all changes to services that happened since proxy rules were synced.  For a single object,
// changes are accumulated, i.e. previous is state from before applying the changes,
// current is state after applying all of the changes.
type serviceChange struct {
	previous ServiceMap
	current  ServiceMap
}

// ServiceChangeTracker carries state about uncommitted changes to an arbitrary number of
// Services, keyed by their namespace and name.
type ServiceChangeTracker struct {
	// lock protects items.
	lock sync.Mutex
	// items maps a service to its serviceChange.
	items map[types.NamespacedName]*serviceChange
	// makeServiceInfo allows proxier to inject customized information when processing service.
	makeServiceInfo         makeServicePortFunc
	processServiceMapChange processServiceMapChangeFunc
	ipFamily                v1.IPFamily

	recorder events.EventRecorder
}

// NewServiceChangeTracker initializes a ServiceChangeTracker
func NewServiceChangeTracker(makeServiceInfo makeServicePortFunc, ipFamily v1.IPFamily, recorder events.EventRecorder, processServiceMapChange processServiceMapChangeFunc) *ServiceChangeTracker {
	return &ServiceChangeTracker{
		items:                   make(map[types.NamespacedName]*serviceChange),
		makeServiceInfo:         makeServiceInfo,
		recorder:                recorder,
		ipFamily:                ipFamily,
		processServiceMapChange: processServiceMapChange,
	}
}

// Update updates given service's change map based on the <previous, current> service pair.  It returns true if items changed,
// otherwise return false.  Update can be used to add/update/delete items of ServiceChangeMap.  For example,
// Add item
//   - pass <nil, service> as the <previous, current> pair.
// Update item
//   - pass <oldService, service> as the <previous, current> pair.
// Delete item
//   - pass <service, nil> as the <previous, current> pair.
func (sct *ServiceChangeTracker) Update(previous, current *v1.Service) bool {
	svc := current
	if svc == nil {
		svc = previous
	}
	// previous == nil && current == nil is unexpected, we should return false directly.
	if svc == nil {
		return false
	}
	metrics.ServiceChangesTotal.Inc()
	namespacedName := types.NamespacedName{Namespace: svc.Namespace, Name: svc.Name}

	sct.lock.Lock()
	defer sct.lock.Unlock()

	change, exists := sct.items[namespacedName]
	if !exists {
		change = &serviceChange{}
		change.previous = sct.serviceToServiceMap(previous)
		sct.items[namespacedName] = change
	}
	change.current = sct.serviceToServiceMap(current)
	// if change.previous equal to change.current, it means no change
	if reflect.DeepEqual(change.previous, change.current) {
		delete(sct.items, namespacedName)
	} else {
		klog.V(2).Infof("Service %s updated: %d ports", namespacedName, len(change.current))
	}
	metrics.ServiceChangesPending.Set(float64(len(sct.items)))
	return len(sct.items) > 0
}

// UpdateServiceMapResult is the updated results after applying service changes.
type UpdateServiceMapResult struct {
	// HCServiceNodePorts is a map of Service names to node port numbers which indicate the health of that Service on this Node.
	// The value(uint16) of HCServices map is the service health check node port.
	HCServiceNodePorts map[types.NamespacedName]uint16
	// UDPStaleClusterIP holds stale (no longer assigned to a Service) Service IPs that had UDP ports.
	// Callers can use this to abort timeout-waits or clear connection-tracking information.
	UDPStaleClusterIP sets.String
}

// Update updates ServiceMap base on the given changes.
func (sm ServiceMap) Update(changes *ServiceChangeTracker) (result UpdateServiceMapResult) {
	result.UDPStaleClusterIP = sets.NewString()
	sm.apply(changes, result.UDPStaleClusterIP)

	// TODO: If this will appear to be computationally expensive, consider
	// computing this incrementally similarly to serviceMap.
	result.HCServiceNodePorts = make(map[types.NamespacedName]uint16)
	for svcPortName, info := range sm {
		if info.HealthCheckNodePort() != 0 {
			result.HCServiceNodePorts[svcPortName.NamespacedName] = uint16(info.HealthCheckNodePort())
		}
	}

	return result
}

// ServiceMap maps a service to its ServicePort.
type ServiceMap map[ServicePortName]ServicePort

// serviceToServiceMap translates a single Service object to a ServiceMap.
//
// NOTE: service object should NOT be modified.
func (sct *ServiceChangeTracker) serviceToServiceMap(service *v1.Service) ServiceMap {
	if service == nil {
		return nil
	}

	if utilproxy.ShouldSkipService(service) {
		return nil
	}

	clusterIP := utilproxy.GetClusterIPByFamily(sct.ipFamily, service)
	if clusterIP == "" {
		return nil
	}

	serviceMap := make(ServiceMap)
	svcName := types.NamespacedName{Namespace: service.Namespace, Name: service.Name}
	for i := range service.Spec.Ports {
		servicePort := &service.Spec.Ports[i]
		svcPortName := ServicePortName{NamespacedName: svcName, Port: servicePort.Name, Protocol: servicePort.Protocol}
		baseSvcInfo := sct.newBaseServiceInfo(servicePort, service)
		if sct.makeServiceInfo != nil {
			serviceMap[svcPortName] = sct.makeServiceInfo(servicePort, service, baseSvcInfo)
		} else {
			serviceMap[svcPortName] = baseSvcInfo
		}
	}
	return serviceMap
}

// apply the changes to ServiceMap and update the stale udp cluster IP set. The UDPStaleClusterIP argument is passed in to store the
// udp protocol service cluster ip when service is deleted from the ServiceMap.
// apply triggers processServiceMapChange on every change.
func (sm *ServiceMap) apply(changes *ServiceChangeTracker, UDPStaleClusterIP sets.String) {
	changes.lock.Lock()
	defer changes.lock.Unlock()
	for _, change := range changes.items {
		if changes.processServiceMapChange != nil {
			changes.processServiceMapChange(change.previous, change.current)
		}
		sm.merge(change.current)
		// filter out the Update event of current changes from previous changes before calling unmerge() so that can
		// skip deleting the Update events.
		change.previous.filter(change.current)
		sm.unmerge(change.previous, UDPStaleClusterIP)
	}
	// clear changes after applying them to ServiceMap.
	changes.items = make(map[types.NamespacedName]*serviceChange)
	metrics.ServiceChangesPending.Set(0)
}

// merge adds other ServiceMap's elements to current ServiceMap.
// If collision, other ALWAYS win. Otherwise add the other to current.
// In other words, if some elements in current collisions with other, update the current by other.
// It returns a string type set which stores all the newly merged services' identifier, ServicePortName.String(), to help users
// tell if a service is deleted or updated.
// The returned value is one of the arguments of ServiceMap.unmerge().
// ServiceMap A Merge ServiceMap B will do following 2 things:
//   * update ServiceMap A.
//   * produce a string set which stores all other ServiceMap's ServicePortName.String().
// For example,
//   - A{}
//   - B{{"ns", "cluster-ip", "http"}: {"172.16.55.10", 1234, "TCP"}}
//     - A updated to be {{"ns", "cluster-ip", "http"}: {"172.16.55.10", 1234, "TCP"}}
//     - produce string set {"ns/cluster-ip:http"}
//   - A{{"ns", "cluster-ip", "http"}: {"172.16.55.10", 345, "UDP"}}
//   - B{{"ns", "cluster-ip", "http"}: {"172.16.55.10", 1234, "TCP"}}
//     - A updated to be {{"ns", "cluster-ip", "http"}: {"172.16.55.10", 1234, "TCP"}}
//     - produce string set {"ns/cluster-ip:http"}
func (sm *ServiceMap) merge(other ServiceMap) sets.String {
	// existingPorts is going to store all identifiers of all services in `other` ServiceMap.
	existingPorts := sets.NewString()
	for svcPortName, info := range other {
		// Take ServicePortName.String() as the newly merged service's identifier and put it into existingPorts.
		existingPorts.Insert(svcPortName.String())
		_, exists := (*sm)[svcPortName]
		if !exists {
			klog.V(1).Infof("Adding new service port %q at %s", svcPortName, info.String())
		} else {
			klog.V(1).Infof("Updating existing service port %q at %s", svcPortName, info.String())
		}
		(*sm)[svcPortName] = info
	}
	return existingPorts
}

// filter filters out elements from ServiceMap base on given ports string sets.
func (sm *ServiceMap) filter(other ServiceMap) {
	for svcPortName := range *sm {
		// skip the delete for Update event.
		if _, ok := other[svcPortName]; ok {
			delete(*sm, svcPortName)
		}
	}
}

// unmerge deletes all other ServiceMap's elements from current ServiceMap.  We pass in the UDPStaleClusterIP strings sets
// for storing the stale udp service cluster IPs. We will clear stale udp connection base on UDPStaleClusterIP later
func (sm *ServiceMap) unmerge(other ServiceMap, UDPStaleClusterIP sets.String) {
	for svcPortName := range other {
		info, exists := (*sm)[svcPortName]
		if exists {
			klog.V(1).Infof("Removing service port %q", svcPortName)
			if info.Protocol() == v1.ProtocolUDP {
				UDPStaleClusterIP.Insert(info.ClusterIP().String())
			}
			delete(*sm, svcPortName)
		} else {
			klog.Errorf("Service port %q doesn't exists", svcPortName)
		}
	}
}
