package cloudprovider

import (
	"context"
	"fmt"

	"github.com/rancher/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
)

var (
	InternalIPLabel = version.Program + ".io/internal-ip"
	ExternalIPLabel = version.Program + ".io/external-ip"
	HostnameLabel   = version.Program + ".io/hostname"
)

func (k *k3s) AddSSHKeyToAllInstances(ctx context.Context, user string, keyData []byte) error {
	return cloudprovider.NotImplemented
}

func (k *k3s) CurrentNodeName(ctx context.Context, hostname string) (types.NodeName, error) {
	return types.NodeName(hostname), nil
}

func (k *k3s) InstanceExistsByProviderID(ctx context.Context, providerID string) (bool, error) {
	return true, nil
}

func (k *k3s) InstanceID(ctx context.Context, nodeName types.NodeName) (string, error) {
	_, err := k.NodeCache.Get(string(nodeName))
	if err != nil {
		return "", fmt.Errorf("Failed to find node %s: %v", nodeName, err)
	}
	return string(nodeName), nil
}

func (k *k3s) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	return true, cloudprovider.NotImplemented
}

func (k *k3s) InstanceType(ctx context.Context, name types.NodeName) (string, error) {
	_, err := k.InstanceID(ctx, name)
	if err != nil {
		return "", err
	}
	return version.Program, nil
}

func (k *k3s) InstanceTypeByProviderID(ctx context.Context, providerID string) (string, error) {
	return "", cloudprovider.NotImplemented
}

func (k *k3s) NodeAddresses(ctx context.Context, name types.NodeName) ([]corev1.NodeAddress, error) {
	addresses := []corev1.NodeAddress{}
	node, err := k.NodeCache.Get(string(name))
	if err != nil {
		return nil, fmt.Errorf("Failed to find node %s: %v", name, err)
	}
	// check internal address
	if node.Labels[InternalIPLabel] != "" {
		addresses = append(addresses, corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: node.Labels[InternalIPLabel]})
	} else {
		logrus.Infof("couldn't find node internal ip label on node %s", name)
	}

	// check external address
	if node.Labels[ExternalIPLabel] != "" {
		addresses = append(addresses, corev1.NodeAddress{Type: corev1.NodeExternalIP, Address: node.Labels[ExternalIPLabel]})
	}

	// check hostname
	if node.Labels[HostnameLabel] != "" {
		addresses = append(addresses, corev1.NodeAddress{Type: corev1.NodeHostName, Address: node.Labels[HostnameLabel]})
	} else {
		logrus.Infof("couldn't find node hostname label on node %s", name)
	}

	return addresses, nil
}

func (k *k3s) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]corev1.NodeAddress, error) {
	return nil, cloudprovider.NotImplemented
}
