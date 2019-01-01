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
	ListenerConfigGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ListenerConfig",
	}
	ListenerConfigResource = metav1.APIResource{
		Name:         "listenerconfigs",
		SingularName: "listenerconfig",
		Namespaced:   true,

		Kind: ListenerConfigGroupVersionKind.Kind,
	}
)

func NewListenerConfig(namespace, name string, obj ListenerConfig) *ListenerConfig {
	obj.APIVersion, obj.Kind = ListenerConfigGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ListenerConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ListenerConfig
}

type ListenerConfigHandlerFunc func(key string, obj *ListenerConfig) (runtime.Object, error)

type ListenerConfigChangeHandlerFunc func(obj *ListenerConfig) (runtime.Object, error)

type ListenerConfigLister interface {
	List(namespace string, selector labels.Selector) (ret []*ListenerConfig, err error)
	Get(namespace, name string) (*ListenerConfig, error)
}

type ListenerConfigController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ListenerConfigLister
	AddHandler(ctx context.Context, name string, handler ListenerConfigHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ListenerConfigHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ListenerConfigInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ListenerConfig) (*ListenerConfig, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ListenerConfig, error)
	Get(name string, opts metav1.GetOptions) (*ListenerConfig, error)
	Update(*ListenerConfig) (*ListenerConfig, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ListenerConfigList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ListenerConfigController
	AddHandler(ctx context.Context, name string, sync ListenerConfigHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ListenerConfigLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ListenerConfigHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ListenerConfigLifecycle)
}

type listenerConfigLister struct {
	controller *listenerConfigController
}

func (l *listenerConfigLister) List(namespace string, selector labels.Selector) (ret []*ListenerConfig, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ListenerConfig))
	})
	return
}

func (l *listenerConfigLister) Get(namespace, name string) (*ListenerConfig, error) {
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
			Group:    ListenerConfigGroupVersionKind.Group,
			Resource: "listenerConfig",
		}, key)
	}
	return obj.(*ListenerConfig), nil
}

type listenerConfigController struct {
	controller.GenericController
}

func (c *listenerConfigController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *listenerConfigController) Lister() ListenerConfigLister {
	return &listenerConfigLister{
		controller: c,
	}
}

func (c *listenerConfigController) AddHandler(ctx context.Context, name string, handler ListenerConfigHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ListenerConfig); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *listenerConfigController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ListenerConfigHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ListenerConfig); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type listenerConfigFactory struct {
}

func (c listenerConfigFactory) Object() runtime.Object {
	return &ListenerConfig{}
}

func (c listenerConfigFactory) List() runtime.Object {
	return &ListenerConfigList{}
}

func (s *listenerConfigClient) Controller() ListenerConfigController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.listenerConfigControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ListenerConfigGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &listenerConfigController{
		GenericController: genericController,
	}

	s.client.listenerConfigControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type listenerConfigClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ListenerConfigController
}

func (s *listenerConfigClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *listenerConfigClient) Create(o *ListenerConfig) (*ListenerConfig, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ListenerConfig), err
}

func (s *listenerConfigClient) Get(name string, opts metav1.GetOptions) (*ListenerConfig, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ListenerConfig), err
}

func (s *listenerConfigClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ListenerConfig, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ListenerConfig), err
}

func (s *listenerConfigClient) Update(o *ListenerConfig) (*ListenerConfig, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ListenerConfig), err
}

func (s *listenerConfigClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *listenerConfigClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *listenerConfigClient) List(opts metav1.ListOptions) (*ListenerConfigList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ListenerConfigList), err
}

func (s *listenerConfigClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *listenerConfigClient) Patch(o *ListenerConfig, patchType types.PatchType, data []byte, subresources ...string) (*ListenerConfig, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*ListenerConfig), err
}

func (s *listenerConfigClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *listenerConfigClient) AddHandler(ctx context.Context, name string, sync ListenerConfigHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *listenerConfigClient) AddLifecycle(ctx context.Context, name string, lifecycle ListenerConfigLifecycle) {
	sync := NewListenerConfigLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *listenerConfigClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ListenerConfigHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *listenerConfigClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ListenerConfigLifecycle) {
	sync := NewListenerConfigLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

type ListenerConfigIndexer func(obj *ListenerConfig) ([]string, error)

type ListenerConfigClientCache interface {
	Get(namespace, name string) (*ListenerConfig, error)
	List(namespace string, selector labels.Selector) ([]*ListenerConfig, error)

	Index(name string, indexer ListenerConfigIndexer)
	GetIndexed(name, key string) ([]*ListenerConfig, error)
}

type ListenerConfigClient interface {
	Create(*ListenerConfig) (*ListenerConfig, error)
	Get(namespace, name string, opts metav1.GetOptions) (*ListenerConfig, error)
	Update(*ListenerConfig) (*ListenerConfig, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*ListenerConfigList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() ListenerConfigClientCache

	OnCreate(ctx context.Context, name string, sync ListenerConfigChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync ListenerConfigChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync ListenerConfigChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() ListenerConfigInterface
}

type listenerConfigClientCache struct {
	client *listenerConfigClient2
}

type listenerConfigClient2 struct {
	iface      ListenerConfigInterface
	controller ListenerConfigController
}

func (n *listenerConfigClient2) Interface() ListenerConfigInterface {
	return n.iface
}

func (n *listenerConfigClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *listenerConfigClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *listenerConfigClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *listenerConfigClient2) Create(obj *ListenerConfig) (*ListenerConfig, error) {
	return n.iface.Create(obj)
}

func (n *listenerConfigClient2) Get(namespace, name string, opts metav1.GetOptions) (*ListenerConfig, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *listenerConfigClient2) Update(obj *ListenerConfig) (*ListenerConfig, error) {
	return n.iface.Update(obj)
}

func (n *listenerConfigClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *listenerConfigClient2) List(namespace string, opts metav1.ListOptions) (*ListenerConfigList, error) {
	return n.iface.List(opts)
}

func (n *listenerConfigClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *listenerConfigClientCache) Get(namespace, name string) (*ListenerConfig, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *listenerConfigClientCache) List(namespace string, selector labels.Selector) ([]*ListenerConfig, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *listenerConfigClient2) Cache() ListenerConfigClientCache {
	n.loadController()
	return &listenerConfigClientCache{
		client: n,
	}
}

func (n *listenerConfigClient2) OnCreate(ctx context.Context, name string, sync ListenerConfigChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &listenerConfigLifecycleDelegate{create: sync})
}

func (n *listenerConfigClient2) OnChange(ctx context.Context, name string, sync ListenerConfigChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &listenerConfigLifecycleDelegate{update: sync})
}

func (n *listenerConfigClient2) OnRemove(ctx context.Context, name string, sync ListenerConfigChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &listenerConfigLifecycleDelegate{remove: sync})
}

func (n *listenerConfigClientCache) Index(name string, indexer ListenerConfigIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*ListenerConfig); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *listenerConfigClientCache) GetIndexed(name, key string) ([]*ListenerConfig, error) {
	var result []*ListenerConfig
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*ListenerConfig); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *listenerConfigClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type listenerConfigLifecycleDelegate struct {
	create ListenerConfigChangeHandlerFunc
	update ListenerConfigChangeHandlerFunc
	remove ListenerConfigChangeHandlerFunc
}

func (n *listenerConfigLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *listenerConfigLifecycleDelegate) Create(obj *ListenerConfig) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *listenerConfigLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *listenerConfigLifecycleDelegate) Remove(obj *ListenerConfig) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *listenerConfigLifecycleDelegate) Updated(obj *ListenerConfig) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
