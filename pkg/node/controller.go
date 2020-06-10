package node

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	coreclient "github.com/rancher/wrangler-api/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	core "k8s.io/api/core/v1"
)

func Register(ctx context.Context, configMap coreclient.ConfigMapController, nodes coreclient.NodeController) error {
	h := &handler{
		configCache:  configMap.Cache(),
		configClient: configMap,
	}
	nodes.OnChange(ctx, "node", h.onChange)
	nodes.OnRemove(ctx, "node", h.onRemove)

	return nil
}

type handler struct {
	configCache  coreclient.ConfigMapCache
	configClient coreclient.ConfigMapClient
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
		newHosts    string
		nodeAddress string
		hostsMap    map[string]string
	)
	hostsMap = make(map[string]string)

	for _, address := range node.Status.Addresses {
		if address.Type == "InternalIP" {
			nodeAddress = address.Address
			break
		}
	}
	if nodeAddress == "" {
		logrus.Errorf("No InternalIP found for node %s", node.Name)
		return nil, nil
	}

	configMapCache, err := h.configCache.Get("kube-system", "coredns")
	if err != nil || configMapCache == nil {
		logrus.Warn(errors.Wrap(err, "Unable to fetch coredns config map"))
		return nil, nil
	}
	configMap := configMapCache.DeepCopy()

	hosts := configMap.Data["NodeHosts"]
	for _, line := range strings.Split(hosts, "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			logrus.Warnf("Unknown format for hosts line [%s]", line)
			continue
		}
		ip := fields[0]
		host := fields[1]
		if host == node.Name {
			if removed {
				continue
			}
			if ip == nodeAddress {
				return nil, nil
			}
		}
		hostsMap[host] = ip
	}

	if !removed {
		hostsMap[node.Name] = nodeAddress
	}
	for host, ip := range hostsMap {
		newHosts += ip + " " + host + "\n"
	}
	configMap.Data["NodeHosts"] = newHosts

	if _, err := h.configClient.Update(configMap); err != nil {
		return nil, err
	}

	var actionType string
	if removed {
		actionType = "Removed"
	} else {
		actionType = "Updated"
	}
	logrus.Infof("%s coredns node hosts entry [%s]", actionType, nodeAddress+" "+node.Name)

	return nil, nil
}
