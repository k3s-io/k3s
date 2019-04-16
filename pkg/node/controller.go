package node

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	coreclient "github.com/rancher/k3s/types/apis/core/v1"
	"github.com/sirupsen/logrus"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func Register(ctx context.Context) error {
	clients := coreclient.ClientsFrom(ctx)
	h := &handler{
		configCache:  clients.ConfigMap.Cache(),
		configClient: clients.ConfigMap,
	}
	clients.Node.OnCreate(ctx, "node", h.onChange)
	clients.Node.OnChange(ctx, "node", h.onChange)
	clients.Node.OnRemove(ctx, "node", h.onRemove)

	return nil
}

type handler struct {
	configCache  coreclient.ConfigMapClientCache
	configClient coreclient.ConfigMapClient
}

func (h *handler) onChange(node *core.Node) (runtime.Object, error) {
	return h.updateHosts(node, false)
}

func (h *handler) onRemove(node *core.Node) (runtime.Object, error) {
	return h.updateHosts(node, true)
}

func (h *handler) updateHosts(node *core.Node, removed bool) (runtime.Object, error) {
	var (
		newHosts    string
		nodeUID     string
		nodeAddress string
		nodeEntry   string
		uidHostsMap map[string]string
	)
	nodeUID = string(node.UID)
	uidHostsMap = make(map[string]string)

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
	nodeEntry = nodeAddress + " " + node.Name

	configMap, err := h.configCache.Get("kube-system", "coredns")
	if err != nil || configMap == nil {
		logrus.Warn(errors.Wrap(err, "Unable to fetch coredns config map"))
		return nil, nil
	}

	hosts := configMap.Data["NodeHosts"]
	for _, line := range strings.Split(hosts, "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 4 || fields[2] != "#" {
			logrus.Warnf("Unknown format for hosts line [%s]", line)
			continue
		}
		ip := fields[0]
		host := fields[1]
		uid := fields[3]
		hostEntry := ip + " " + host
		if uid == nodeUID {
			if removed {
				continue
			}
			if hostEntry == nodeEntry {
				return nil, nil
			}
		}
		uidHostsMap[uid] = hostEntry
	}

	if !removed {
		uidHostsMap[nodeUID] = nodeEntry
	}
	for uid, hostEntry := range uidHostsMap {
		newHosts += hostEntry + " # " + uid + "\n"
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
	logrus.Infof("%s coredns node hosts entry [%s]", actionType, nodeEntry)

	return nil, nil
}
