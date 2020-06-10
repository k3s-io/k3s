package cloudprovider

import (
	"context"
	"io"

	"github.com/rancher/k3s/pkg/version"
	"github.com/rancher/wrangler-api/pkg/generated/controllers/core"
	coreclient "github.com/rancher/wrangler-api/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/start"
	cloudprovider "k8s.io/cloud-provider"
)

type k3s struct {
	NodeCache coreclient.NodeCache
}

func init() {
	cloudprovider.RegisterCloudProvider(version.Program, func(config io.Reader) (cloudprovider.Interface, error) {
		return &k3s{}, nil
	})
}

func (k *k3s) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	coreFactory := core.NewFactoryFromConfigOrDie(clientBuilder.ConfigOrDie("cloud-controller-manager"))

	go start.All(context.Background(), 1, coreFactory)

	k.NodeCache = coreFactory.Core().V1().Node().Cache()
}

func (k *k3s) Instances() (cloudprovider.Instances, bool) {
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
