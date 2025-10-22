package node

import (
	"bytes"
	"context"
	"net"
	"sort"
	"strings"

	coreclient "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	toolscache "k8s.io/client-go/tools/cache"
)

func Register(ctx context.Context,
	modCoreDNS bool,
	coreClient kubernetes.Interface,
	nodes coreclient.NodeController,
) error {
	// create a single-resource watch cache on the coredns configmap so that we
	// don't have to retrieve it from the apiserver every time a node changes.
	lw := toolscache.NewListWatchFromClient(coreClient.CoreV1().RESTClient(), "configmaps", metav1.NamespaceSystem, fields.OneTermEqualSelector(metav1.ObjectNameField, "coredns"))
	informerOpts := toolscache.InformerOptions{ListerWatcher: lw, ObjectType: &corev1.ConfigMap{}, Handler: &toolscache.ResourceEventHandlerFuncs{}}
	indexer, informer := toolscache.NewInformerWithOptions(informerOpts)
	go informer.Run(ctx.Done())

	h := &handler{
		modCoreDNS:      modCoreDNS,
		ctx:             ctx,
		configMaps:      coreClient.CoreV1().ConfigMaps(metav1.NamespaceSystem),
		configMapsStore: indexer,
	}
	nodes.OnChange(ctx, "node", h.updateHosts)
	nodes.OnRemove(ctx, "node", h.updateHosts)

	return nil
}

type handler struct {
	modCoreDNS      bool
	ctx             context.Context
	configMaps      typedcorev1.ConfigMapInterface
	configMapsStore toolscache.Store
}

func (h *handler) updateHosts(key string, node *corev1.Node) (*corev1.Node, error) {
	if h.modCoreDNS && node != nil {
		var (
			hostName string
			nodeIPv4 string
			nodeIPv6 string
		)
		for _, address := range node.Status.Addresses {
			switch address.Type {
			case corev1.NodeInternalIP:
				if strings.Contains(address.Address, ":") {
					nodeIPv6 = address.Address
				} else {
					nodeIPv4 = address.Address
				}
			case corev1.NodeHostName:
				hostName = address.Address
			}
		}
		if err := h.updateCoreDNSConfigMap(node.Name, hostName, nodeIPv4, nodeIPv6, node.DeletionTimestamp != nil); err != nil {
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
		logrus.Errorf("No InternalIP addresses found for node %s", nodeName)
		return nil
	}

	nodeNames := nodeName
	if hostName != nodeName {
		nodeNames += " " + hostName
	}

	var configMap *corev1.ConfigMap
	if val, ok, err := h.configMapsStore.GetByKey("kube-system/coredns"); err != nil {
		logrus.Errorf("Failed to get coredns ConfigMap from cache: %v", err)
	} else if ok {
		if cm, ok := val.(*corev1.ConfigMap); ok {
			configMap = cm
		}
	}
	if configMap == nil {
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

	// Something's out of sync, copy the ConfigMap for update and sync the desired entries
	configMap = configMap.DeepCopy()
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

	if _, err := h.configMaps.Update(h.ctx, configMap, metav1.UpdateOptions{}); err != nil {
		return err
	}

	var actionType string
	if removed {
		actionType = "Removed"
	} else {
		actionType = "Synced"
	}
	logrus.Infof("%s coredns NodeHosts entries for %s", actionType, nodeName)
	return nil
}
