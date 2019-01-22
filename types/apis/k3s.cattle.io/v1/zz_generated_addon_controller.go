package v1

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var (
	AddonGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Addon",
	}
	AddonResource = metav1.APIResource{
		Name:         "addons",
		SingularName: "addon",
		Namespaced:   true,

		Kind: AddonGroupVersionKind.Kind,
	}
)

func NewAddon(namespace, name string, obj Addon) *Addon {
	obj.APIVersion, obj.Kind = AddonGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type AddonList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Addon
}

type AddonHandlerFunc func(key string, obj *Addon) (runtime.Object, error)

type AddonChangeHandlerFunc func(obj *Addon) (runtime.Object, error)

type AddonLister interface {
	List(namespace string, selector labels.Selector) (ret []*Addon, err error)
	Get(namespace, name string) (*Addon, error)
}

type AddonController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() AddonLister
	AddHandler(ctx context.Context, name string, handler AddonHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler AddonHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type AddonInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*Addon) (*Addon, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Addon, error)
	Get(name string, opts metav1.GetOptions) (*Addon, error)
	Update(*Addon) (*Addon, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*AddonList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() AddonController
	AddHandler(ctx context.Context, name string, sync AddonHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle AddonLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync AddonHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle AddonLifecycle)
}

type addonLister struct {
	controller *addonController
}

func (l *addonLister) List(namespace string, selector labels.Selector) (ret []*Addon, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*Addon))
	})
	return
}

func (l *addonLister) Get(namespace, name string) (*Addon, error) {
	var key string
	if namespace != "" {
		key = namespace + "/" + name
	} else {
		key = name
	}
	obj, exists, err := l.controller.Informer().GetIndexer().GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(schema.GroupResource{
			Group:    AddonGroupVersionKind.Group,
			Resource: "addon",
		}, key)
	}
	return obj.(*Addon), nil
}

type addonController struct {
	controller.GenericController
}

func (c *addonController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *addonController) Lister() AddonLister {
	return &addonLister{
		controller: c,
	}
}

func (c *addonController) AddHandler(ctx context.Context, name string, handler AddonHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Addon); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *addonController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler AddonHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Addon); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type addonFactory struct {
}

func (c addonFactory) Object() runtime.Object {
	return &Addon{}
}

func (c addonFactory) List() runtime.Object {
	return &AddonList{}
}

func (s *addonClient) Controller() AddonController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.addonControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(AddonGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &addonController{
		GenericController: genericController,
	}

	s.client.addonControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type addonClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   AddonController
}

func (s *addonClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *addonClient) Create(o *Addon) (*Addon, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*Addon), err
}

func (s *addonClient) Get(name string, opts metav1.GetOptions) (*Addon, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*Addon), err
}

func (s *addonClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Addon, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*Addon), err
}

func (s *addonClient) Update(o *Addon) (*Addon, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Addon), err
}

func (s *addonClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *addonClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *addonClient) List(opts metav1.ListOptions) (*AddonList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*AddonList), err
}

func (s *addonClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *addonClient) Patch(o *Addon, patchType types.PatchType, data []byte, subresources ...string) (*Addon, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*Addon), err
}

func (s *addonClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *addonClient) AddHandler(ctx context.Context, name string, sync AddonHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *addonClient) AddLifecycle(ctx context.Context, name string, lifecycle AddonLifecycle) {
	sync := NewAddonLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *addonClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync AddonHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *addonClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle AddonLifecycle) {
	sync := NewAddonLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

type AddonIndexer func(obj *Addon) ([]string, error)

type AddonClientCache interface {
	Get(namespace, name string) (*Addon, error)
	List(namespace string, selector labels.Selector) ([]*Addon, error)

	Index(name string, indexer AddonIndexer)
	GetIndexed(name, key string) ([]*Addon, error)
}

type AddonClient interface {
	Create(*Addon) (*Addon, error)
	Get(namespace, name string, opts metav1.GetOptions) (*Addon, error)
	Update(*Addon) (*Addon, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*AddonList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() AddonClientCache

	OnCreate(ctx context.Context, name string, sync AddonChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync AddonChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync AddonChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() AddonInterface
}

type addonClientCache struct {
	client *addonClient2
}

type addonClient2 struct {
	iface      AddonInterface
	controller AddonController
}

func (n *addonClient2) Interface() AddonInterface {
	return n.iface
}

func (n *addonClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *addonClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *addonClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *addonClient2) Create(obj *Addon) (*Addon, error) {
	return n.iface.Create(obj)
}

func (n *addonClient2) Get(namespace, name string, opts metav1.GetOptions) (*Addon, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *addonClient2) Update(obj *Addon) (*Addon, error) {
	return n.iface.Update(obj)
}

func (n *addonClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *addonClient2) List(namespace string, opts metav1.ListOptions) (*AddonList, error) {
	return n.iface.List(opts)
}

func (n *addonClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *addonClientCache) Get(namespace, name string) (*Addon, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *addonClientCache) List(namespace string, selector labels.Selector) ([]*Addon, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *addonClient2) Cache() AddonClientCache {
	n.loadController()
	return &addonClientCache{
		client: n,
	}
}

func (n *addonClient2) OnCreate(ctx context.Context, name string, sync AddonChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &addonLifecycleDelegate{create: sync})
}

func (n *addonClient2) OnChange(ctx context.Context, name string, sync AddonChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &addonLifecycleDelegate{update: sync})
}

func (n *addonClient2) OnRemove(ctx context.Context, name string, sync AddonChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &addonLifecycleDelegate{remove: sync})
}

func (n *addonClientCache) Index(name string, indexer AddonIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*Addon); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *addonClientCache) GetIndexed(name, key string) ([]*Addon, error) {
	var result []*Addon
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*Addon); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *addonClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type addonLifecycleDelegate struct {
	create AddonChangeHandlerFunc
	update AddonChangeHandlerFunc
	remove AddonChangeHandlerFunc
}

func (n *addonLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *addonLifecycleDelegate) Create(obj *Addon) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *addonLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *addonLifecycleDelegate) Remove(obj *Addon) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *addonLifecycleDelegate) Updated(obj *Addon) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
