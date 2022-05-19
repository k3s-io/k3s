package cloudprovider

import (
	"io"

	"github.com/k3s-io/k3s/pkg/version"
	"k8s.io/client-go/informers"
	informercorev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	cloudprovider "k8s.io/cloud-provider"
)

type k3s struct {
	nodeInformer          informercorev1.NodeInformer
	nodeInformerHasSynced cache.InformerSynced
}

var _ cloudprovider.Interface = &k3s{}
var _ cloudprovider.InformerUser = &k3s{}

func init() {
	cloudprovider.RegisterCloudProvider(version.Program, func(config io.Reader) (cloudprovider.Interface, error) {
		return &k3s{}, nil
	})
}

func (k *k3s) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
}

func (k *k3s) SetInformers(informerFactory informers.SharedInformerFactory) {
	k.nodeInformer = informerFactory.Core().V1().Nodes()
	k.nodeInformerHasSynced = k.nodeInformer.Informer().HasSynced
}

func (k *k3s) Instances() (cloudprovider.Instances, bool) {
	return k, true
}

func (k *k3s) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return nil, false
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
	return true
}
