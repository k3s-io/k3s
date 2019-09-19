package apply

import (
	"fmt"
	"sync"

	"github.com/rancher/wrangler/pkg/apply/injectors"
	"github.com/rancher/wrangler/pkg/objectset"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const (
	defaultNamespace = "default"
)

type Patcher func(namespace, name string, pt types.PatchType, data []byte) (runtime.Object, error)

type ClientFactory func(gvk schema.GroupVersionKind) (dynamic.NamespaceableResourceInterface, error)

type InformerGetter interface {
	Informer() cache.SharedIndexInformer
	GroupVersionKind() schema.GroupVersionKind
}

type Apply interface {
	Apply(set *objectset.ObjectSet) error
	ApplyObjects(objs ...runtime.Object) error
	WithCacheTypes(igs ...InformerGetter) Apply
	WithSetID(id string) Apply
	WithOwner(obj runtime.Object) Apply
	WithInjector(injs ...injectors.ConfigInjector) Apply
	WithInjectorName(injs ...string) Apply
	WithPatcher(gvk schema.GroupVersionKind, patchers Patcher) Apply
	WithStrictCaching() Apply
	WithDefaultNamespace(ns string) Apply
	WithListerNamespace(ns string) Apply
	WithRateLimiting(ratelimitingQps float32) Apply
	WithNoDelete() Apply
}

func NewForConfig(cfg *rest.Config) (Apply, error) {
	k8s, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return New(k8s.Discovery(), NewClientFactory(cfg)), nil
}

func New(discovery discovery.DiscoveryInterface, cf ClientFactory, igs ...InformerGetter) Apply {
	a := &apply{
		clients: &clients{
			clientFactory: cf,
			discovery:     discovery,
			namespaced:    map[schema.GroupVersionKind]bool{},
			clients:       map[schema.GroupVersionKind]dynamic.NamespaceableResourceInterface{},
		},
		informers: map[schema.GroupVersionKind]cache.SharedIndexInformer{},
	}

	for _, ig := range igs {
		a.informers[ig.GroupVersionKind()] = ig.Informer()
	}

	return a
}

type apply struct {
	clients   *clients
	informers map[schema.GroupVersionKind]cache.SharedIndexInformer
}

type clients struct {
	sync.Mutex

	clientFactory ClientFactory
	discovery     discovery.DiscoveryInterface
	namespaced    map[schema.GroupVersionKind]bool
	clients       map[schema.GroupVersionKind]dynamic.NamespaceableResourceInterface
}

func (c *clients) IsNamespaced(gvk schema.GroupVersionKind) bool {
	c.Lock()
	defer c.Unlock()
	return c.namespaced[gvk]
}

func (c *clients) client(gvk schema.GroupVersionKind) (dynamic.NamespaceableResourceInterface, error) {
	c.Lock()
	defer c.Unlock()

	if client, ok := c.clients[gvk]; ok {
		return client, nil
	}

	resources, err := c.discovery.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
	if err != nil {
		return nil, err
	}

	for _, resource := range resources.APIResources {
		if resource.Kind != gvk.Kind {
			continue
		}

		client, err := c.clientFactory(gvk)
		if err != nil {
			return nil, err
		}

		c.namespaced[gvk] = resource.Namespaced
		c.clients[gvk] = client
		return client, nil
	}

	return nil, fmt.Errorf("failed to discover client for %s", gvk)
}

func (a *apply) newDesiredSet() desiredSet {
	return desiredSet{
		a:                a,
		defaultNamespace: defaultNamespace,
		ratelimitingQps:  1,
	}
}

func (a *apply) Apply(set *objectset.ObjectSet) error {
	return a.newDesiredSet().Apply(set)
}

func (a *apply) ApplyObjects(objs ...runtime.Object) error {
	os := objectset.NewObjectSet()
	os.Add(objs...)
	return a.newDesiredSet().Apply(os)
}

func (a *apply) WithSetID(id string) Apply {
	return a.newDesiredSet().WithSetID(id)
}

func (a *apply) WithOwner(obj runtime.Object) Apply {
	return a.newDesiredSet().WithOwner(obj)
}

func (a *apply) WithInjector(injs ...injectors.ConfigInjector) Apply {
	return a.newDesiredSet().WithInjector(injs...)
}

func (a *apply) WithInjectorName(injs ...string) Apply {
	return a.newDesiredSet().WithInjectorName(injs...)
}

func (a *apply) WithCacheTypes(igs ...InformerGetter) Apply {
	return a.newDesiredSet().WithCacheTypes(igs...)
}

func (a *apply) WithPatcher(gvk schema.GroupVersionKind, patcher Patcher) Apply {
	return a.newDesiredSet().WithPatcher(gvk, patcher)
}

func (a *apply) WithStrictCaching() Apply {
	return a.newDesiredSet().WithStrictCaching()
}

func (a *apply) WithDefaultNamespace(ns string) Apply {
	return a.newDesiredSet().WithDefaultNamespace(ns)
}

func (a *apply) WithListerNamespace(ns string) Apply {
	return a.newDesiredSet().WithListerNamespace(ns)
}

func (a *apply) WithRateLimiting(ratelimitingQps float32) Apply {
	return a.newDesiredSet().WithRateLimiting(ratelimitingQps)
}

func (a *apply) WithNoDelete() Apply {
	return a.newDesiredSet().WithNoDelete()
}
