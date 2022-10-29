package cloudprovider

import (
	"context"
	"fmt"
	"strings"

	"github.com/k3s-io/k3s/pkg/version"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
)

var (
	InternalIPKey = version.Program + ".io/internal-ip"
	ExternalIPKey = version.Program + ".io/external-ip"
	HostnameKey   = version.Program + ".io/hostname"
)

var _ cloudprovider.InstancesV2 = &k3s{}

// InstanceExists returns true if the instance for the given node exists according to the cloud provider.
// K3s nodes always exist.
func (k *k3s) InstanceExists(ctx context.Context, node *corev1.Node) (bool, error) {
	return true, nil
}

// InstanceShutdown returns true if the instance is shutdown according to the cloud provider.
// K3s nodes are never shutdown.
func (k *k3s) InstanceShutdown(ctx context.Context, node *v1.Node) (bool, error) {
	return false, nil
}

// InstanceMetadata returns the instance's metadata.
func (k *k3s) InstanceMetadata(ctx context.Context, node *v1.Node) (*cloudprovider.InstanceMetadata, error) {
	if (node.Annotations[InternalIPKey] == "") && (node.Labels[InternalIPKey] == "") {
		return nil, errors.New("address annotations not yet set")
	}

	addresses := []corev1.NodeAddress{}

	// check internal address
	if address := node.Annotations[InternalIPKey]; address != "" {
		for _, v := range strings.Split(address, ",") {
			addresses = append(addresses, corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: v})
		}
	} else if address = node.Labels[InternalIPKey]; address != "" {
		addresses = append(addresses, corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: address})
	} else {
		logrus.Infof("Couldn't find node internal ip annotation or label on node %s", node.Name)
	}

	// check external address
	if address := node.Annotations[ExternalIPKey]; address != "" {
		for _, v := range strings.Split(address, ",") {
			addresses = append(addresses, corev1.NodeAddress{Type: corev1.NodeExternalIP, Address: v})
		}
	} else if address = node.Labels[ExternalIPKey]; address != "" {
		addresses = append(addresses, corev1.NodeAddress{Type: corev1.NodeExternalIP, Address: address})
	}

	// check hostname
	if address := node.Annotations[HostnameKey]; address != "" {
		addresses = append(addresses, corev1.NodeAddress{Type: corev1.NodeHostName, Address: address})
	} else if address = node.Labels[HostnameKey]; address != "" {
		addresses = append(addresses, corev1.NodeAddress{Type: corev1.NodeHostName, Address: address})
	} else {
		logrus.Infof("Couldn't find node hostname annotation or label on node %s", node.Name)
	}

	return &cloudprovider.InstanceMetadata{
		ProviderID:    fmt.Sprintf("%s://%s", version.Program, node.Name),
		InstanceType:  version.Program,
		NodeAddresses: addresses,
		Zone:          "",
		Region:        "",
	}, nil
}
