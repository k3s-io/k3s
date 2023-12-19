package cloudprovider

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	cloudprovider "k8s.io/cloud-provider"
)

var _ cloudprovider.LoadBalancer = &k3s{}

// GetLoadBalancer returns whether the specified load balancer exists, and if so, what its status is.
func (k *k3s) GetLoadBalancer(ctx context.Context, clusterName string, service *corev1.Service) (*corev1.LoadBalancerStatus, bool, error) {
	if _, err := k.getDaemonSet(service); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	status, err := k.getStatus(service)
	return status, true, err
}

// GetLoadBalancerName returns the name of the load balancer.
func (k *k3s) GetLoadBalancerName(ctx context.Context, clusterName string, service *corev1.Service) string {
	return generateName(service)
}

// EnsureLoadBalancer creates a new load balancer 'name', or updates the existing one. Returns the status of the balancer.
// The node list is unused; see the comment on UpdateLoadBalancer for information on why.
// This is called when the Service is created or changes.
func (k *k3s) EnsureLoadBalancer(ctx context.Context, clusterName string, service *corev1.Service, nodes []*corev1.Node) (*corev1.LoadBalancerStatus, error) {
	if err := k.deployDaemonSet(ctx, service); err != nil {
		return nil, err
	}
	return nil, cloudprovider.ImplementedElsewhere
}

// UpdateLoadBalancer updates hosts under the specified load balancer.
// This is not used, as it filters node updates based on criteria not compatible with how our DaemonSet selects
// nodes for inclusion.  It also does not provide any opportunity to update the load balancer status.
// https://github.com/kubernetes/kubernetes/blob/v1.25.0/staging/src/k8s.io/cloud-provider/controllers/service/controller.go#L985-L993
func (k *k3s) UpdateLoadBalancer(ctx context.Context, clusterName string, service *corev1.Service, nodes []*corev1.Node) error {
	return cloudprovider.ImplementedElsewhere
}

// EnsureLoadBalancerDeleted deletes the specified load balancer if it exists,
// returning nil if the load balancer specified either didn't exist or was successfully deleted.
func (k *k3s) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *corev1.Service) error {
	return k.deleteDaemonSet(ctx, service)
}
