//go:build windows

package netpol

import (
	"fmt"
	"net/netip"
	"slices"
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type policyDirection string

const (
	policyIngress policyDirection = "In"
	policyEgress  policyDirection = "Out"

	policyAllowPriority uint16 = 222
	policyNodePriority  uint16 = 100
	policyDenyPriority  uint16 = 3000
)

type policyRule struct {
	action          string
	direction       policyDirection
	protocol        string
	remoteAddresses string
	ports           string
	priority        uint16
}

type peerSelection struct {
	all       bool
	addresses []string
	pods      []*corev1.Pod
}

type portSelection struct {
	protocol            string
	ports               string
	remoteAddresses     string
	restrictRemotePeers bool
}

func compilePolicyRules(target *corev1.Pod, pods []*corev1.Pod, namespaces map[string]*corev1.Namespace, nodeIPs []string, policies []*networkingv1.NetworkPolicy) ([]policyRule, error) {
	var ingressIsolated, egressIsolated bool
	var rules []policyRule

	for _, policy := range policies {
		if policy.Namespace != target.Namespace {
			continue
		}

		targetSelector, err := metav1.LabelSelectorAsSelector(&policy.Spec.PodSelector)
		if err != nil {
			return nil, fmt.Errorf("invalid pod selector in NetworkPolicy %s/%s: %w", policy.Namespace, policy.Name, err)
		}
		if !targetSelector.Matches(labels.Set(target.Labels)) {
			continue
		}

		policyTypes := policy.Spec.PolicyTypes
		if len(policyTypes) == 0 {
			policyTypes = []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}
			if len(policy.Spec.Egress) > 0 {
				policyTypes = append(policyTypes, networkingv1.PolicyTypeEgress)
			}
		}
		hasIngress, hasEgress := false, false
		for _, policyType := range policyTypes {
			switch policyType {
			case networkingv1.PolicyTypeIngress:
				ingressIsolated = true
				hasIngress = true
			case networkingv1.PolicyTypeEgress:
				egressIsolated = true
				hasEgress = true
			}
		}

		if hasIngress {
			for _, rule := range policy.Spec.Ingress {
				selection, err := selectPeers(rule.From, policy.Namespace, pods, namespaces)
				if err != nil {
					return nil, fmt.Errorf("selecting ingress peers for NetworkPolicy %s/%s: %w", policy.Namespace, policy.Name, err)
				}
				portRules, err := selectPorts(rule.Ports, target, selection, policyIngress)
				if err != nil {
					return nil, fmt.Errorf("selecting ingress ports for NetworkPolicy %s/%s: %w", policy.Namespace, policy.Name, err)
				}
				rules = appendAllowedRules(rules, policyIngress, selection, portRules)
			}
		}

		if hasEgress {
			for _, rule := range policy.Spec.Egress {
				selection, err := selectPeers(rule.To, policy.Namespace, pods, namespaces)
				if err != nil {
					return nil, fmt.Errorf("selecting egress peers for NetworkPolicy %s/%s: %w", policy.Namespace, policy.Name, err)
				}
				portRules, err := selectPorts(rule.Ports, target, selection, policyEgress)
				if err != nil {
					return nil, fmt.Errorf("selecting egress ports for NetworkPolicy %s/%s: %w", policy.Namespace, policy.Name, err)
				}
				rules = appendAllowedRules(rules, policyEgress, selection, portRules)
			}
		}
	}

	hasPolicy := ingressIsolated || egressIsolated
	filteredRules := rules[:0]
	for _, rule := range rules {
		allAddresses := rule.remoteAddresses == "" || slices.Contains(strings.Split(rule.remoteAddresses, ","), "0.0.0.0/0")
		if allAddresses && rule.protocol == "" && rule.ports == "" {
			// A wildcard allow needs no fallback deny with the same match.
			switch rule.direction {
			case policyIngress:
				ingressIsolated = false
			case policyEgress:
				egressIsolated = false
			}
			continue
		}
		filteredRules = append(filteredRules, rule)
	}
	rules = filteredRules

	if ingressIsolated {
		if addresses := ipv4Addresses(nodeIPs); addresses != "" {
			rules = append(rules, policyRule{
				action:          "Allow",
				direction:       policyIngress,
				remoteAddresses: addresses,
				priority:        policyNodePriority,
			})
		}
		rules = append(rules, policyRule{action: "Block", direction: policyIngress, priority: policyDenyPriority})
	} else if hasPolicy {
		// Host ACLs require an explicit allow for the non-isolated direction.
		rules = append(rules, policyRule{action: "Allow", direction: policyIngress, priority: policyAllowPriority})
	}
	if egressIsolated {
		if addresses := ipv4Addresses(nodeIPs); addresses != "" {
			rules = append(rules, policyRule{
				action:          "Allow",
				direction:       policyEgress,
				remoteAddresses: addresses,
				priority:        policyNodePriority,
			})
		}
		rules = append(rules, policyRule{action: "Block", direction: policyEgress, priority: policyDenyPriority})
	} else if hasPolicy {
		rules = append(rules, policyRule{action: "Allow", direction: policyEgress, priority: policyAllowPriority})
	}

	return rules, nil
}

func appendAllowedRules(rules []policyRule, direction policyDirection, selection peerSelection, ports []portSelection) []policyRule {
	addresses := ""
	if !selection.all {
		addresses = strings.Join(selection.addresses, ",")
		if addresses == "" {
			return rules
		}
	}
	for _, port := range ports {
		remoteAddresses := addresses
		if port.restrictRemotePeers {
			remoteAddresses = port.remoteAddresses
			if remoteAddresses == "" {
				continue
			}
		}
		rules = append(rules, policyRule{
			action:          "Allow",
			direction:       direction,
			protocol:        port.protocol,
			remoteAddresses: remoteAddresses,
			ports:           port.ports,
			priority:        policyAllowPriority,
		})
	}
	return rules
}

func selectPeers(peers []networkingv1.NetworkPolicyPeer, policyNamespace string, pods []*corev1.Pod, namespaces map[string]*corev1.Namespace) (peerSelection, error) {
	if len(peers) == 0 {
		return peerSelection{all: true, pods: pods}, nil
	}

	selection := peerSelection{}
	addresses := make(map[string]struct{})
	podUIDs := make(map[string]struct{})
	addPod := func(pod *corev1.Pod) {
		if _, ok := podUIDs[string(pod.UID)]; !ok {
			podUIDs[string(pod.UID)] = struct{}{}
			selection.pods = append(selection.pods, pod)
		}
		for _, address := range podAddresses(pod) {
			addresses[address] = struct{}{}
		}
	}

	for _, peer := range peers {
		if peer.IPBlock != nil {
			if peer.PodSelector != nil || peer.NamespaceSelector != nil {
				return peerSelection{}, fmt.Errorf("IPBlock cannot be combined with pod or namespace selectors")
			}
			cidrs, err := subtractIPv4CIDRs(peer.IPBlock.CIDR, peer.IPBlock.Except)
			if err != nil {
				return peerSelection{}, err
			}
			for _, cidr := range cidrs {
				addresses[cidr] = struct{}{}
			}
			continue
		}
		if peer.PodSelector == nil && peer.NamespaceSelector == nil {
			return peerSelection{}, fmt.Errorf("NetworkPolicy peer has no IPBlock or selectors")
		}

		var namespaceSelector labels.Selector
		if peer.NamespaceSelector != nil {
			selector, err := metav1.LabelSelectorAsSelector(peer.NamespaceSelector)
			if err != nil {
				return peerSelection{}, fmt.Errorf("invalid namespace selector: %w", err)
			}
			namespaceSelector = selector
		}

		var podSelector labels.Selector
		if peer.PodSelector == nil {
			podSelector = labels.Everything()
		} else {
			selector, err := metav1.LabelSelectorAsSelector(peer.PodSelector)
			if err != nil {
				return peerSelection{}, fmt.Errorf("invalid pod selector: %w", err)
			}
			podSelector = selector
		}

		for _, pod := range pods {
			namespace, ok := namespaces[pod.Namespace]
			if !ok || (namespaceSelector == nil && pod.Namespace != policyNamespace) || (namespaceSelector != nil && !namespaceSelector.Matches(labels.Set(namespace.Labels))) || !podSelector.Matches(labels.Set(pod.Labels)) {
				continue
			}
			addPod(pod)
		}
	}

	selection.addresses = sortedKeys(addresses)
	return selection, nil
}

func selectPorts(ports []networkingv1.NetworkPolicyPort, target *corev1.Pod, peers peerSelection, direction policyDirection) ([]portSelection, error) {
	if len(ports) == 0 {
		return []portSelection{{}}, nil
	}

	var result []portSelection
	for _, policyPort := range ports {
		if policyPort.Port == nil && policyPort.EndPort != nil {
			return nil, fmt.Errorf("NetworkPolicy endPort requires a numeric port")
		}
		protocol := corev1.ProtocolTCP
		if policyPort.Protocol != nil {
			protocol = *policyPort.Protocol
		}
		protocolNumber, err := protocolNumber(protocol)
		if err != nil {
			return nil, err
		}

		if policyPort.Port == nil {
			result = append(result, portSelection{protocol: protocolNumber})
			continue
		}

		if policyPort.Port.Type == intstr.Int {
			port := policyPort.Port.IntValue()
			if port < 1 || port > 65535 {
				return nil, fmt.Errorf("NetworkPolicy port %d is outside the valid range 1-65535", port)
			}
			portString := strconv.Itoa(port)
			if policyPort.EndPort != nil {
				endPort := int(*policyPort.EndPort)
				if endPort < port || endPort > 65535 {
					return nil, fmt.Errorf("NetworkPolicy endPort %d must be between port %d and 65535", endPort, port)
				}
				var values []string
				for value := port; value <= endPort; value++ {
					values = append(values, strconv.Itoa(value))
				}
				portString = strings.Join(values, ",")
			}
			result = append(result, portSelection{protocol: protocolNumber, ports: portString})
			continue
		}
		if policyPort.Port.Type != intstr.String {
			return nil, fmt.Errorf("unsupported NetworkPolicy port type %d", policyPort.Port.Type)
		}

		if policyPort.EndPort != nil {
			return nil, fmt.Errorf("named NetworkPolicy port %q cannot specify endPort", policyPort.Port.String())
		}
		name := policyPort.Port.StrVal
		if name == "" {
			return nil, fmt.Errorf("NetworkPolicy named port cannot be empty")
		}
		portAddresses := make(map[int]map[string]struct{})
		if direction == policyIngress {
			portNumbers := make(map[int]struct{})
			addNamedContainerPorts(portNumbers, target, name, protocol)
			for port := range portNumbers {
				portAddresses[port] = nil
			}
		} else {
			for _, pod := range peers.pods {
				portNumbers := make(map[int]struct{})
				addNamedContainerPorts(portNumbers, pod, name, protocol)
				for _, address := range podAddresses(pod) {
					for port := range portNumbers {
						if portAddresses[port] == nil {
							portAddresses[port] = make(map[string]struct{})
						}
						portAddresses[port][address] = struct{}{}
					}
				}
			}
		}
		numbers := make([]int, 0, len(portAddresses))
		for port := range portAddresses {
			numbers = append(numbers, port)
		}
		sort.Ints(numbers)
		for _, port := range numbers {
			selection := portSelection{protocol: protocolNumber, ports: strconv.Itoa(port)}
			if direction == policyEgress {
				selection.remoteAddresses = strings.Join(sortedKeys(portAddresses[port]), ",")
				selection.restrictRemotePeers = true
			}
			result = append(result, selection)
		}
	}

	return result, nil
}

func addNamedContainerPorts(result map[int]struct{}, pod *corev1.Pod, name string, protocol corev1.Protocol) {
	for _, container := range pod.Spec.Containers {
		for _, port := range container.Ports {
			portProtocol := port.Protocol
			if portProtocol == "" {
				portProtocol = corev1.ProtocolTCP
			}
			if port.Name == name && portProtocol == protocol {
				result[int(port.ContainerPort)] = struct{}{}
			}
		}
	}
}

func protocolNumber(protocol corev1.Protocol) (string, error) {
	switch protocol {
	case corev1.ProtocolTCP:
		return "6", nil
	case corev1.ProtocolUDP:
		return "17", nil
	case corev1.ProtocolSCTP:
		return "132", nil
	default:
		return "", fmt.Errorf("unsupported NetworkPolicy protocol %q", protocol)
	}
}

func podAddresses(pod *corev1.Pod) []string {
	addresses := make(map[string]struct{})
	podIPs := pod.Status.PodIPs
	if len(podIPs) == 0 && pod.Status.PodIP != "" {
		podIPs = []corev1.PodIP{{IP: pod.Status.PodIP}}
	}
	for _, podIP := range podIPs {
		address, err := netip.ParseAddr(podIP.IP)
		if err != nil || !address.Is4() {
			continue
		}
		addresses[address.String()] = struct{}{}
	}
	return sortedKeys(addresses)
}

func ipv4Addresses(addresses []string) string {
	result := make(map[string]struct{})
	for _, value := range addresses {
		address, err := netip.ParseAddr(value)
		if err == nil && address.Is4() {
			result[address.String()] = struct{}{}
		}
	}
	return strings.Join(sortedKeys(result), ",")
}

func subtractIPv4CIDRs(cidr string, except []string) ([]string, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid IPBlock CIDR %q: %w", cidr, err)
	}
	if !prefix.Addr().Is4() {
		return nil, nil
	}
	prefix = prefix.Masked()

	var exclusions []netip.Prefix
	for _, value := range except {
		exclusion, err := netip.ParsePrefix(value)
		if err != nil {
			return nil, fmt.Errorf("invalid IPBlock exception CIDR %q: %w", value, err)
		}
		if exclusion.Addr().Is4() {
			exclusions = append(exclusions, exclusion.Masked())
		}
	}

	var result []string
	var subtract func(netip.Prefix)
	subtract = func(current netip.Prefix) {
		overlaps := false
		for _, exclusion := range exclusions {
			if exclusion.Bits() <= current.Bits() && exclusion.Contains(current.Addr()) {
				return
			}
			if current.Contains(exclusion.Addr()) {
				overlaps = true
			}
		}
		if !overlaps {
			result = append(result, current.String())
			return
		}

		left, right := splitIPv4Prefix(current)
		subtract(left)
		subtract(right)
	}
	subtract(prefix)
	sort.Strings(result)
	return result, nil
}

func splitIPv4Prefix(prefix netip.Prefix) (netip.Prefix, netip.Prefix) {
	bits := prefix.Bits() + 1
	left := netip.PrefixFrom(prefix.Masked().Addr(), bits).Masked()
	bytes := left.Addr().As4()
	bit := prefix.Bits()
	bytes[bit/8] |= 1 << (7 - uint(bit%8))
	right := netip.PrefixFrom(netip.AddrFrom4(bytes), bits).Masked()
	return left, right
}

func sortedKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
