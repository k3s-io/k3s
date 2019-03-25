package v1

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"k8s.io/api/core/v1"
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
	ServiceGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Service",
	}
	ServiceResource = metav1.APIResource{
		Name:         "services",
		SingularName: "service",
		Namespaced:   true,

		Kind: ServiceGroupVersionKind.Kind,
	}
)

func NewService(namespace, name string, obj v1.Service) *v1.Service {
	obj.APIVersion, obj.Kind = ServiceGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.Service
}

type ServiceHandlerFunc func(key string, obj *v1.Service) (runtime.Object, error)

type ServiceChangeHandlerFunc func(obj *v1.Service) (runtime.Object, error)

type ServiceLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Service, err error)
	Get(namespace, name string) (*v1.Service, error)
}

type ServiceController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ServiceLister
	AddHandler(ctx context.Context, name string, handler ServiceHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ServiceHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ServiceInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.Service) (*v1.Service, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Service, error)
	Get(name string, opts metav1.GetOptions) (*v1.Service, error)
	Update(*v1.Service) (*v1.Service, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ServiceList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ServiceController
	AddHandler(ctx context.Context, name string, sync ServiceHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ServiceLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ServiceHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ServiceLifecycle)
}

type serviceLister struct {
	controller *serviceController
}

func (l *serviceLister) List(namespace string, selector labels.Selector) (ret []*v1.Service, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.Service))
	})
	return
}

func (l *serviceLister) Get(namespace, name string) (*v1.Service, error) {
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
			Group:    ServiceGroupVersionKind.Group,
			Resource: "service",
		}, key)
	}
	return obj.(*v1.Service), nil
}

type serviceController struct {
	controller.GenericController
}

func (c *serviceController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *serviceController) Lister() ServiceLister {
	return &serviceLister{
		controller: c,
	}
}

func (c *serviceController) AddHandler(ctx context.Context, name string, handler ServiceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Service); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *serviceController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ServiceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Service); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type serviceFactory struct {
}

func (c serviceFactory) Object() runtime.Object {
	return &v1.Service{}
}

func (c serviceFactory) List() runtime.Object {
	return &ServiceList{}
}

func (s *serviceClient) Controller() ServiceController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.serviceControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ServiceGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &serviceController{
		GenericController: genericController,
	}

	s.client.serviceControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type serviceClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ServiceController
}

func (s *serviceClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *serviceClient) Create(o *v1.Service) (*v1.Service, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.Service), err
}

func (s *serviceClient) Get(name string, opts metav1.GetOptions) (*v1.Service, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.Service), err
}

func (s *serviceClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Service, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.Service), err
}

func (s *serviceClient) Update(o *v1.Service) (*v1.Service, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.Service), err
}

func (s *serviceClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *serviceClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *serviceClient) List(opts metav1.ListOptions) (*ServiceList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ServiceList), err
}

func (s *serviceClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *serviceClient) Patch(o *v1.Service, patchType types.PatchType, data []byte, subresources ...string) (*v1.Service, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.Service), err
}

func (s *serviceClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *serviceClient) AddHandler(ctx context.Context, name string, sync ServiceHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *serviceClient) AddLifecycle(ctx context.Context, name string, lifecycle ServiceLifecycle) {
	sync := NewServiceLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *serviceClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ServiceHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *serviceClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ServiceLifecycle) {
	sync := NewServiceLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

type ServiceIndexer func(obj *v1.Service) ([]string, error)

type ServiceClientCache interface {
	Get(namespace, name string) (*v1.Service, error)
	List(namespace string, selector labels.Selector) ([]*v1.Service, error)

	Index(name string, indexer ServiceIndexer)
	GetIndexed(name, key string) ([]*v1.Service, error)
}

type ServiceClient interface {
	Create(*v1.Service) (*v1.Service, error)
	Get(namespace, name string, opts metav1.GetOptions) (*v1.Service, error)
	Update(*v1.Service) (*v1.Service, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*ServiceList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() ServiceClientCache

	OnCreate(ctx context.Context, name string, sync ServiceChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync ServiceChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync ServiceChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() ServiceInterface
}

type serviceClientCache struct {
	client *serviceClient2
}

type serviceClient2 struct {
	iface      ServiceInterface
	controller ServiceController
}

func (n *serviceClient2) Interface() ServiceInterface {
	return n.iface
}

func (n *serviceClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *serviceClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *serviceClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *serviceClient2) Create(obj *v1.Service) (*v1.Service, error) {
	return n.iface.Create(obj)
}

func (n *serviceClient2) Get(namespace, name string, opts metav1.GetOptions) (*v1.Service, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *serviceClient2) Update(obj *v1.Service) (*v1.Service, error) {
	return n.iface.Update(obj)
}

func (n *serviceClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *serviceClient2) List(namespace string, opts metav1.ListOptions) (*ServiceList, error) {
	return n.iface.List(opts)
}

func (n *serviceClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *serviceClientCache) Get(namespace, name string) (*v1.Service, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *serviceClientCache) List(namespace string, selector labels.Selector) ([]*v1.Service, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *serviceClient2) Cache() ServiceClientCache {
	n.loadController()
	return &serviceClientCache{
		client: n,
	}
}

func (n *serviceClient2) OnCreate(ctx context.Context, name string, sync ServiceChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &serviceLifecycleDelegate{create: sync})
}

func (n *serviceClient2) OnChange(ctx context.Context, name string, sync ServiceChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &serviceLifecycleDelegate{update: sync})
}

func (n *serviceClient2) OnRemove(ctx context.Context, name string, sync ServiceChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &serviceLifecycleDelegate{remove: sync})
}

func (n *serviceClientCache) Index(name string, indexer ServiceIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*v1.Service); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *serviceClientCache) GetIndexed(name, key string) ([]*v1.Service, error) {
	var result []*v1.Service
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*v1.Service); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *serviceClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type serviceLifecycleDelegate struct {
	create ServiceChangeHandlerFunc
	update ServiceChangeHandlerFunc
	remove ServiceChangeHandlerFunc
}

func (n *serviceLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *serviceLifecycleDelegate) Create(obj *v1.Service) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *serviceLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *serviceLifecycleDelegate) Remove(obj *v1.Service) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *serviceLifecycleDelegate) Updated(obj *v1.Service) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
