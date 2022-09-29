package cloudprovider

import (
	"io"

	"github.com/k3s-io/k3s/pkg/version"
	cloudprovider "k8s.io/cloud-provider"
)

type k3s struct {
}

var _ cloudprovider.Interface = &k3s{}

func init() {
	cloudprovider.RegisterCloudProvider(version.Program, func(config io.Reader) (cloudprovider.Interface, error) {
		return &k3s{}, nil
	})
}

func (k *k3s) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
}

func (k *k3s) Instances() (cloudprovider.Instances, bool) {
	return nil, false
}

func (k *k3s) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return k, true
}

func (k *k3s) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return nil, false
}

func (k *k3s) Zones() (cloudprovider.Zones, bool) {
	return nil, false
}

func (k *k3s) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

func (k *k3s) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

func (k *k3s) ProviderName() string {
	return version.Program
}

func (k *k3s) HasClusterID() bool {
	return false
}
