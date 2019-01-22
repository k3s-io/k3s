package objectset

import (
	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

type Client interface {
	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
}

type Processor struct {
	setID         string
	codeVersion   string
	discovery     discovery.DiscoveryInterface
	restConfig    rest.Config
	allowSlowPath string
	slowClient    rest.HTTPClient
	clients       map[schema.GroupVersionKind]Client
}

func NewProcessor(setID string) *Processor {
	return &Processor{
		setID:   setID,
		clients: map[schema.GroupVersionKind]Client{},
	}
}

func (t *Processor) SetID() string {
	return t.setID
}

func (t *Processor) CodeVersion(version string) *Processor {
	t.codeVersion = version
	return t
}

func (t *Processor) AllowDiscovery(discovery discovery.DiscoveryInterface, restConfig rest.Config) *Processor {
	t.discovery = discovery
	t.restConfig = restConfig
	return t
}

func (t *Processor) Clients() map[schema.GroupVersionKind]Client {
	return t.clients
}

func (t *Processor) Client(clients ...Client) *Processor {
	// ensure cache is enabled
	for _, client := range clients {
		client.Generic()
		t.clients[client.ObjectClient().GroupVersionKind()] = client
	}
	return t
}

func (t Processor) Remove(owner runtime.Object) error {
	return t.NewDesiredSet(owner, nil).Apply()
}

func (t Processor) NewDesiredSet(owner runtime.Object, objs *ObjectSet) *DesiredSet {
	remove := false
	if objs == nil {
		remove = true
		objs = &ObjectSet{}
	}
	return &DesiredSet{
		discovery:   t.discovery,
		restConfig:  t.restConfig,
		remove:      remove,
		objs:        objs,
		setID:       t.setID,
		codeVersion: t.codeVersion,
		clients:     t.clients,
		owner:       owner,
	}
}
