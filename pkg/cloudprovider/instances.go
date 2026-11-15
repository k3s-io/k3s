package cloudprovider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	cloudprovider "k8s.io/cloud-provider"
)

var (
	InternalIPKey  = version.Program + ".io/internal-ip"
	ExternalIPKey  = version.Program + ".io/external-ip"
	InternalDNSKey = version.Program + ".io/internal-dns"
	ExternalDNSKey = version.Program + ".io/external-dns"
	HostnameKey    = version.Program + ".io/hostname"
)

var _ cloudprovider.InstancesV2 = &k3s{}

// InstanceExists returns true if the instance for the given node exists according to the cloud provider.
// K3s nodes always exist.
func (k *k3s) InstanceExists(ctx context.Context, node *corev1.Node) (bool, error) {
	return true, nil
}

// InstanceShutdown returns true if the instance is shutdown according to the cloud provider.
// K3s nodes are never shutdown.
func (k *k3s) InstanceShutdown(ctx context.Context, node *corev1.Node) (bool, error) {
	return false, nil
}

// InstanceMetadata returns the instance's metadata.
func (k *k3s) InstanceMetadata(ctx context.Context, node *corev1.Node) (*cloudprovider.InstanceMetadata, error) {
	if (node.Annotations[InternalIPKey] == "") && (node.Labels[InternalIPKey] == "") {
		return nil, errors.New("address annotations not yet set")
	}

	metadata := &cloudprovider.InstanceMetadata{
		ProviderID:   fmt.Sprintf("%s://%s", version.Program, node.Name),
		InstanceType: version.Program,
	}

	if node.Spec.ProviderID != "" {
		metadata.ProviderID = node.Spec.ProviderID
	}

	if instanceType := node.Labels[corev1.LabelInstanceTypeStable]; instanceType != "" {
		metadata.InstanceType = instanceType
	}

	if region := node.Labels[corev1.LabelTopologyRegion]; region != "" {
		metadata.Region = region
	}

	if zone := node.Labels[corev1.LabelTopologyZone]; zone != "" {
		metadata.Zone = zone
	}

	// check internal address
	if address := node.Annotations[InternalIPKey]; address != "" {
		for _, v := range strings.Split(address, ",") {
			metadata.NodeAddresses = append(metadata.NodeAddresses, corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: v})
		}
	} else if address = node.Labels[InternalIPKey]; address != "" {
		metadata.NodeAddresses = append(metadata.NodeAddresses, corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: address})
	} else {
		logrus.Infof("Couldn't find node internal ip annotation or label on node %s", node.Name)
	}

	// check external address
	if address := node.Annotations[ExternalIPKey]; address != "" {
		for _, v := range strings.Split(address, ",") {
			metadata.NodeAddresses = append(metadata.NodeAddresses, corev1.NodeAddress{Type: corev1.NodeExternalIP, Address: v})
		}
	} else if address = node.Labels[ExternalIPKey]; address != "" {
		metadata.NodeAddresses = append(metadata.NodeAddresses, corev1.NodeAddress{Type: corev1.NodeExternalIP, Address: address})
	}

	// check internal dns
	if address := node.Annotations[InternalDNSKey]; address != "" {
		for _, v := range strings.Split(address, ",") {
			metadata.NodeAddresses = append(metadata.NodeAddresses, corev1.NodeAddress{Type: corev1.NodeInternalDNS, Address: v})
		}
	}

	// check external dns
	if address := node.Annotations[ExternalDNSKey]; address != "" {
		for _, v := range strings.Split(address, ",") {
			metadata.NodeAddresses = append(metadata.NodeAddresses, corev1.NodeAddress{Type: corev1.NodeExternalDNS, Address: v})
		}
	}

	// check hostname
	if address := node.Annotations[HostnameKey]; address != "" {
		metadata.NodeAddresses = append(metadata.NodeAddresses, corev1.NodeAddress{Type: corev1.NodeHostName, Address: address})
	} else if address = node.Labels[HostnameKey]; address != "" {
		metadata.NodeAddresses = append(metadata.NodeAddresses, corev1.NodeAddress{Type: corev1.NodeHostName, Address: address})
	} else {
		logrus.Infof("Couldn't find node hostname annotation or label on node %s", node.Name)
	}

	if k.RejectDuplicateNodeAddresses {
		if err := k.checkDuplicateNodeAddresses(node.Name, metadata.NodeAddresses); err != nil {
			return nil, err
		}
	}

	return metadata, nil
}

// checkDuplicateNodeAddresses returns an error if any of the node's internal or external IPs
// already belong to another node.
func (k *k3s) checkDuplicateNodeAddresses(nodeName string, nodeAddresses []corev1.NodeAddress) error {
	if k.nodeCache == nil {
		return errors.New("node cache not ready")
	}
	nodes, err := k.nodeCache.List(labels.Everything())
	if err != nil {
		return err
	}
	addresses := []string{}
	for _, address := range nodeAddresses {
		if address.Type == corev1.NodeInternalIP || address.Type == corev1.NodeExternalIP {
			addresses = append(addresses, address.Address)
		}
	}
	if address, owner := util.DuplicateNodeAddress(nodes, nodeName, addresses); address != "" {
		return fmt.Errorf("refusing to set address %s on node %s: already assigned to node %s", address, nodeName, owner)
	}
	return nil
}
