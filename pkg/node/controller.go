package node

import (
	"bytes"
	"context"
	"net"
	"sort"
	"strings"

	"github.com/k3s-io/k3s/pkg/nodepassword"
	"github.com/pkg/errors"
	coreclient "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	core "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Register(ctx context.Context,
	modCoreDNS bool,
	secrets coreclient.SecretController,
	configMaps coreclient.ConfigMapController,
	nodes coreclient.NodeController,
) error {
	h := &handler{
		modCoreDNS: modCoreDNS,
		secrets:    secrets,
		configMaps: configMaps,
	}
	nodes.OnChange(ctx, "node", h.onChange)
	nodes.OnRemove(ctx, "node", h.onRemove)

	return nil
}

type handler struct {
	modCoreDNS bool
	secrets    coreclient.SecretController
	configMaps coreclient.ConfigMapController
}

func (h *handler) onChange(key string, node *core.Node) (*core.Node, error) {
	if node == nil {
		return nil, nil
	}
	return h.updateHosts(node, false)
}

func (h *handler) onRemove(key string, node *core.Node) (*core.Node, error) {
	return h.updateHosts(node, true)
}

func (h *handler) updateHosts(node *core.Node, removed bool) (*core.Node, error) {
	var (
		nodeName string
		hostName string
		nodeIPv4 string
		nodeIPv6 string
	)
	nodeName = node.Name
	for _, address := range node.Status.Addresses {
		switch address.Type {
		case v1.NodeInternalIP:
			if strings.Contains(address.Address, ":") {
				nodeIPv6 = address.Address
			} else {
				nodeIPv4 = address.Address
			}
		case v1.NodeHostName:
			hostName = address.Address
		}
	}
	if removed {
		if err := h.removeNodePassword(nodeName); err != nil {
			logrus.Warn(errors.Wrap(err, "Unable to remove node password"))
		}
	}
	if h.modCoreDNS {
		if err := h.updateCoreDNSConfigMap(nodeName, hostName, nodeIPv4, nodeIPv6, removed); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (h *handler) updateCoreDNSConfigMap(nodeName, hostName, nodeIPv4, nodeIPv6 string, removed bool) error {
	if removed {
		nodeIPv4 = ""
		nodeIPv6 = ""
	} else if nodeIPv4 == "" && nodeIPv6 == "" {
		logrus.Errorf("No InternalIP addresses found for node " + nodeName)
		return nil
	}

	nodeNames := nodeName
	if hostName != nodeName {
		nodeNames += " " + hostName
	}

	configMap, err := h.configMaps.Get("kube-system", "coredns", metav1.GetOptions{})
	if err != nil || configMap == nil {
		logrus.Warn(errors.Wrap(err, "Unable to fetch coredns config map"))
		return nil
	}

	addressMap := map[string]string{}

	// extract current entries from hosts file, skipping any entries that are
	// empty, unparsable, or hold an incorrect address for the current node.
	for _, line := range strings.Split(configMap.Data["NodeHosts"], "\n") {
		line, _, _ = strings.Cut(line, "#")
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			logrus.Warnf("Unknown format for hosts line [%s]", line)
			continue
		}
		ip := fields[0]
		if fields[1] == nodeName {
			if strings.Contains(ip, ":") {
				if ip != nodeIPv6 {
					continue
				}
			} else {
				if ip != nodeIPv4 {
					continue
				}
			}
		}
		names := strings.Join(fields[1:], " ")
		addressMap[ip] = names
	}

	// determine what names we should have for each address family
	var namesv6, namesv4 string
	if nodeIPv4 != "" {
		namesv4 = nodeNames
	}
	if nodeIPv6 != "" {
		namesv6 = nodeNames
	}

	// don't need to do anything if the addresses are in sync
	if !removed && addressMap[nodeIPv4] == namesv4 && addressMap[nodeIPv6] == namesv6 {
		return nil
	}

	// Something's out of sync, set the desired entries
	if nodeIPv4 != "" {
		addressMap[nodeIPv4] = namesv4
	}
	if nodeIPv6 != "" {
		addressMap[nodeIPv6] = namesv6
	}

	// sort addresses by IP
	addresses := make([]string, 0, len(addressMap))
	for ip := range addressMap {
		addresses = append(addresses, ip)
	}
	sort.Slice(addresses, func(i, j int) bool {
		return bytes.Compare(net.ParseIP(addresses[i]), net.ParseIP(addresses[j])) < 0
	})

	var newHosts string
	for _, ip := range addresses {
		newHosts += ip + " " + addressMap[ip] + "\n"
	}

	if configMap.Data == nil {
		configMap.Data = map[string]string{}
	}
	configMap.Data["NodeHosts"] = newHosts

	if _, err := h.configMaps.Update(configMap); err != nil {
		return err
	}

	var actionType string
	if removed {
		actionType = "Removed"
	} else {
		actionType = "Updated"
	}
	logrus.Infof("%s coredns NodeHosts entry for %s", actionType, nodeName)
	return nil
}

func (h *handler) removeNodePassword(nodeName string) error {
	return nodepassword.Delete(h.secrets, nodeName)
}
