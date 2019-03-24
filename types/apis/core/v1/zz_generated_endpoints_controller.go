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
	EndpointsGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Endpoints",
	}
	EndpointsResource = metav1.APIResource{
		Name:         "endpoints",
		SingularName: "endpoints",
		Namespaced:   true,

		Kind: EndpointsGroupVersionKind.Kind,
	}
)

func NewEndpoints(namespace, name string, obj v1.Endpoints) *v1.Endpoints {
	obj.APIVersion, obj.Kind = EndpointsGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type EndpointsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.Endpoints
}

type EndpointsHandlerFunc func(key string, obj *v1.Endpoints) (runtime.Object, error)

type EndpointsChangeHandlerFunc func(obj *v1.Endpoints) (runtime.Object, error)

type EndpointsLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Endpoints, err error)
	Get(namespace, name string) (*v1.Endpoints, error)
}

type EndpointsController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() EndpointsLister
	AddHandler(ctx context.Context, name string, handler EndpointsHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler EndpointsHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type EndpointsInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.Endpoints) (*v1.Endpoints, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Endpoints, error)
	Get(name string, opts metav1.GetOptions) (*v1.Endpoints, error)
	Update(*v1.Endpoints) (*v1.Endpoints, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*EndpointsList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() EndpointsController
	AddHandler(ctx context.Context, name string, sync EndpointsHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle EndpointsLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync EndpointsHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle EndpointsLifecycle)
}

type endpointsLister struct {
	controller *endpointsController
}

func (l *endpointsLister) List(namespace string, selector labels.Selector) (ret []*v1.Endpoints, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.Endpoints))
	})
	return
}

func (l *endpointsLister) Get(namespace, name string) (*v1.Endpoints, error) {
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
			Group:    EndpointsGroupVersionKind.Group,
			Resource: "endpoints",
		}, key)
	}
	return obj.(*v1.Endpoints), nil
}

type endpointsController struct {
	controller.GenericController
}

func (c *endpointsController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *endpointsController) Lister() EndpointsLister {
	return &endpointsLister{
		controller: c,
	}
}

func (c *endpointsController) AddHandler(ctx context.Context, name string, handler EndpointsHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Endpoints); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *endpointsController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler EndpointsHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Endpoints); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type endpointsFactory struct {
}

func (c endpointsFactory) Object() runtime.Object {
	return &v1.Endpoints{}
}

func (c endpointsFactory) List() runtime.Object {
	return &EndpointsList{}
}

func (s *endpointsClient) Controller() EndpointsController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.endpointsControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(EndpointsGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &endpointsController{
		GenericController: genericController,
	}

	s.client.endpointsControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type endpointsClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   EndpointsController
}

func (s *endpointsClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *endpointsClient) Create(o *v1.Endpoints) (*v1.Endpoints, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.Endpoints), err
}

func (s *endpointsClient) Get(name string, opts metav1.GetOptions) (*v1.Endpoints, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.Endpoints), err
}

func (s *endpointsClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Endpoints, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.Endpoints), err
}

func (s *endpointsClient) Update(o *v1.Endpoints) (*v1.Endpoints, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.Endpoints), err
}

func (s *endpointsClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *endpointsClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *endpointsClient) List(opts metav1.ListOptions) (*EndpointsList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*EndpointsList), err
}

func (s *endpointsClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *endpointsClient) Patch(o *v1.Endpoints, patchType types.PatchType, data []byte, subresources ...string) (*v1.Endpoints, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.Endpoints), err
}

func (s *endpointsClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *endpointsClient) AddHandler(ctx context.Context, name string, sync EndpointsHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *endpointsClient) AddLifecycle(ctx context.Context, name string, lifecycle EndpointsLifecycle) {
	sync := NewEndpointsLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *endpointsClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync EndpointsHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *endpointsClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle EndpointsLifecycle) {
	sync := NewEndpointsLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

type EndpointsIndexer func(obj *v1.Endpoints) ([]string, error)

type EndpointsClientCache interface {
	Get(namespace, name string) (*v1.Endpoints, error)
	List(namespace string, selector labels.Selector) ([]*v1.Endpoints, error)

	Index(name string, indexer EndpointsIndexer)
	GetIndexed(name, key string) ([]*v1.Endpoints, error)
}

type EndpointsClient interface {
	Create(*v1.Endpoints) (*v1.Endpoints, error)
	Get(namespace, name string, opts metav1.GetOptions) (*v1.Endpoints, error)
	Update(*v1.Endpoints) (*v1.Endpoints, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*EndpointsList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() EndpointsClientCache

	OnCreate(ctx context.Context, name string, sync EndpointsChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync EndpointsChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync EndpointsChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() EndpointsInterface
}

type endpointsClientCache struct {
	client *endpointsClient2
}

type endpointsClient2 struct {
	iface      EndpointsInterface
	controller EndpointsController
}

func (n *endpointsClient2) Interface() EndpointsInterface {
	return n.iface
}

func (n *endpointsClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *endpointsClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *endpointsClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *endpointsClient2) Create(obj *v1.Endpoints) (*v1.Endpoints, error) {
	return n.iface.Create(obj)
}

func (n *endpointsClient2) Get(namespace, name string, opts metav1.GetOptions) (*v1.Endpoints, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *endpointsClient2) Update(obj *v1.Endpoints) (*v1.Endpoints, error) {
	return n.iface.Update(obj)
}

func (n *endpointsClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *endpointsClient2) List(namespace string, opts metav1.ListOptions) (*EndpointsList, error) {
	return n.iface.List(opts)
}

func (n *endpointsClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *endpointsClientCache) Get(namespace, name string) (*v1.Endpoints, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *endpointsClientCache) List(namespace string, selector labels.Selector) ([]*v1.Endpoints, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *endpointsClient2) Cache() EndpointsClientCache {
	n.loadController()
	return &endpointsClientCache{
		client: n,
	}
}

func (n *endpointsClient2) OnCreate(ctx context.Context, name string, sync EndpointsChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &endpointsLifecycleDelegate{create: sync})
}

func (n *endpointsClient2) OnChange(ctx context.Context, name string, sync EndpointsChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &endpointsLifecycleDelegate{update: sync})
}

func (n *endpointsClient2) OnRemove(ctx context.Context, name string, sync EndpointsChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &endpointsLifecycleDelegate{remove: sync})
}

func (n *endpointsClientCache) Index(name string, indexer EndpointsIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*v1.Endpoints); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *endpointsClientCache) GetIndexed(name, key string) ([]*v1.Endpoints, error) {
	var result []*v1.Endpoints
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*v1.Endpoints); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *endpointsClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type endpointsLifecycleDelegate struct {
	create EndpointsChangeHandlerFunc
	update EndpointsChangeHandlerFunc
	remove EndpointsChangeHandlerFunc
}

func (n *endpointsLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *endpointsLifecycleDelegate) Create(obj *v1.Endpoints) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *endpointsLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *endpointsLifecycleDelegate) Remove(obj *v1.Endpoints) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *endpointsLifecycleDelegate) Updated(obj *v1.Endpoints) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
