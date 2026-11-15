package util

import (
	"net"

	v1 "k8s.io/api/core/v1"
)

// DuplicateNodeAddress returns the first of the given IP addresses that a node other than
// nodeName already reports as an InternalIP or ExternalIP, along with the name of that node.
// If nodeName is in the list, only nodes created before it are considered, so the node that
// had an address first keeps it. Loopback and unspecified addresses are ignored, as every
// node has them. It returns two empty strings when there is no conflict.
func DuplicateNodeAddress(nodes []*v1.Node, nodeName string, addresses []string) (string, string) {
	var self *v1.Node
	for _, node := range nodes {
		if node != nil && node.Name == nodeName {
			self = node
		}
	}

	owners := map[string]*v1.Node{}
	for _, node := range nodes {
		if node == nil || node.Name == nodeName {
			continue
		}
		if self != nil && !node.CreationTimestamp.Before(&self.CreationTimestamp) {
			continue
		}
		for _, address := range node.Status.Addresses {
			if address.Type != v1.NodeInternalIP && address.Type != v1.NodeExternalIP {
				continue
			}
			ip := net.ParseIP(address.Address)
			if ip == nil {
				continue
			}
			// report the oldest owner, the cache lists nodes in no particular order
			if owner, ok := owners[ip.String()]; !ok || node.CreationTimestamp.Before(&owner.CreationTimestamp) {
				owners[ip.String()] = node
			}
		}
	}

	for _, address := range addresses {
		ip := net.ParseIP(address)
		if ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
			continue
		}
		if owner, ok := owners[ip.String()]; ok {
			return address, owner.Name
		}
	}
	return "", ""
}
