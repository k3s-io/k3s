package objectset

import (
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/pkg/objectset/injectors"
	"github.com/rancher/norman/types"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

type DesiredSet struct {
	discoveredClients map[schema.GroupVersionKind]*objectclient.ObjectClient
	discovery         discovery.DiscoveryInterface
	restConfig        rest.Config
	remove            bool
	setID             string
	objs              *ObjectSet
	codeVersion       string
	clients           map[schema.GroupVersionKind]Client
	owner             runtime.Object
	injectors         []injectors.ConfigInjector
	errs              []error
}

func (o *DesiredSet) AddDiscoveredClient(gvk schema.GroupVersionKind, client *objectclient.ObjectClient) {
	if o.discoveredClients == nil {
		o.discoveredClients = map[schema.GroupVersionKind]*objectclient.ObjectClient{}
	}
	o.discoveredClients[gvk] = client
}

func (o *DesiredSet) DiscoveredClients() map[schema.GroupVersionKind]*objectclient.ObjectClient {
	return o.discoveredClients
}

func (o *DesiredSet) AddInjector(inj injectors.ConfigInjector) {
	o.injectors = append(o.injectors, inj)
}

func (o *DesiredSet) err(err error) error {
	o.errs = append(o.errs, err)
	return o.Err()
}

func (o *DesiredSet) Err() error {
	return types.NewErrors(append(o.objs.errs, o.errs...)...)
}
