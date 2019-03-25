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
	ServiceAccountGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ServiceAccount",
	}
	ServiceAccountResource = metav1.APIResource{
		Name:         "serviceaccounts",
		SingularName: "serviceaccount",
		Namespaced:   true,

		Kind: ServiceAccountGroupVersionKind.Kind,
	}
)

func NewServiceAccount(namespace, name string, obj v1.ServiceAccount) *v1.ServiceAccount {
	obj.APIVersion, obj.Kind = ServiceAccountGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ServiceAccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.ServiceAccount
}

type ServiceAccountHandlerFunc func(key string, obj *v1.ServiceAccount) (runtime.Object, error)

type ServiceAccountChangeHandlerFunc func(obj *v1.ServiceAccount) (runtime.Object, error)

type ServiceAccountLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.ServiceAccount, err error)
	Get(namespace, name string) (*v1.ServiceAccount, error)
}

type ServiceAccountController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ServiceAccountLister
	AddHandler(ctx context.Context, name string, handler ServiceAccountHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ServiceAccountHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ServiceAccountInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.ServiceAccount) (*v1.ServiceAccount, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ServiceAccount, error)
	Get(name string, opts metav1.GetOptions) (*v1.ServiceAccount, error)
	Update(*v1.ServiceAccount) (*v1.ServiceAccount, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ServiceAccountList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ServiceAccountController
	AddHandler(ctx context.Context, name string, sync ServiceAccountHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ServiceAccountLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ServiceAccountHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ServiceAccountLifecycle)
}

type serviceAccountLister struct {
	controller *serviceAccountController
}

func (l *serviceAccountLister) List(namespace string, selector labels.Selector) (ret []*v1.ServiceAccount, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.ServiceAccount))
	})
	return
}

func (l *serviceAccountLister) Get(namespace, name string) (*v1.ServiceAccount, error) {
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
			Group:    ServiceAccountGroupVersionKind.Group,
			Resource: "serviceAccount",
		}, key)
	}
	return obj.(*v1.ServiceAccount), nil
}

type serviceAccountController struct {
	controller.GenericController
}

func (c *serviceAccountController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *serviceAccountController) Lister() ServiceAccountLister {
	return &serviceAccountLister{
		controller: c,
	}
}

func (c *serviceAccountController) AddHandler(ctx context.Context, name string, handler ServiceAccountHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ServiceAccount); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *serviceAccountController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ServiceAccountHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ServiceAccount); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type serviceAccountFactory struct {
}

func (c serviceAccountFactory) Object() runtime.Object {
	return &v1.ServiceAccount{}
}

func (c serviceAccountFactory) List() runtime.Object {
	return &ServiceAccountList{}
}

func (s *serviceAccountClient) Controller() ServiceAccountController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.serviceAccountControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ServiceAccountGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &serviceAccountController{
		GenericController: genericController,
	}

	s.client.serviceAccountControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type serviceAccountClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ServiceAccountController
}

func (s *serviceAccountClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *serviceAccountClient) Create(o *v1.ServiceAccount) (*v1.ServiceAccount, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.ServiceAccount), err
}

func (s *serviceAccountClient) Get(name string, opts metav1.GetOptions) (*v1.ServiceAccount, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.ServiceAccount), err
}

func (s *serviceAccountClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ServiceAccount, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.ServiceAccount), err
}

func (s *serviceAccountClient) Update(o *v1.ServiceAccount) (*v1.ServiceAccount, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.ServiceAccount), err
}

func (s *serviceAccountClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *serviceAccountClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *serviceAccountClient) List(opts metav1.ListOptions) (*ServiceAccountList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ServiceAccountList), err
}

func (s *serviceAccountClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *serviceAccountClient) Patch(o *v1.ServiceAccount, patchType types.PatchType, data []byte, subresources ...string) (*v1.ServiceAccount, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.ServiceAccount), err
}

func (s *serviceAccountClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *serviceAccountClient) AddHandler(ctx context.Context, name string, sync ServiceAccountHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *serviceAccountClient) AddLifecycle(ctx context.Context, name string, lifecycle ServiceAccountLifecycle) {
	sync := NewServiceAccountLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *serviceAccountClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ServiceAccountHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *serviceAccountClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ServiceAccountLifecycle) {
	sync := NewServiceAccountLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

type ServiceAccountIndexer func(obj *v1.ServiceAccount) ([]string, error)

type ServiceAccountClientCache interface {
	Get(namespace, name string) (*v1.ServiceAccount, error)
	List(namespace string, selector labels.Selector) ([]*v1.ServiceAccount, error)

	Index(name string, indexer ServiceAccountIndexer)
	GetIndexed(name, key string) ([]*v1.ServiceAccount, error)
}

type ServiceAccountClient interface {
	Create(*v1.ServiceAccount) (*v1.ServiceAccount, error)
	Get(namespace, name string, opts metav1.GetOptions) (*v1.ServiceAccount, error)
	Update(*v1.ServiceAccount) (*v1.ServiceAccount, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*ServiceAccountList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() ServiceAccountClientCache

	OnCreate(ctx context.Context, name string, sync ServiceAccountChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync ServiceAccountChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync ServiceAccountChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() ServiceAccountInterface
}

type serviceAccountClientCache struct {
	client *serviceAccountClient2
}

type serviceAccountClient2 struct {
	iface      ServiceAccountInterface
	controller ServiceAccountController
}

func (n *serviceAccountClient2) Interface() ServiceAccountInterface {
	return n.iface
}

func (n *serviceAccountClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *serviceAccountClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *serviceAccountClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *serviceAccountClient2) Create(obj *v1.ServiceAccount) (*v1.ServiceAccount, error) {
	return n.iface.Create(obj)
}

func (n *serviceAccountClient2) Get(namespace, name string, opts metav1.GetOptions) (*v1.ServiceAccount, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *serviceAccountClient2) Update(obj *v1.ServiceAccount) (*v1.ServiceAccount, error) {
	return n.iface.Update(obj)
}

func (n *serviceAccountClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *serviceAccountClient2) List(namespace string, opts metav1.ListOptions) (*ServiceAccountList, error) {
	return n.iface.List(opts)
}

func (n *serviceAccountClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *serviceAccountClientCache) Get(namespace, name string) (*v1.ServiceAccount, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *serviceAccountClientCache) List(namespace string, selector labels.Selector) ([]*v1.ServiceAccount, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *serviceAccountClient2) Cache() ServiceAccountClientCache {
	n.loadController()
	return &serviceAccountClientCache{
		client: n,
	}
}

func (n *serviceAccountClient2) OnCreate(ctx context.Context, name string, sync ServiceAccountChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &serviceAccountLifecycleDelegate{create: sync})
}

func (n *serviceAccountClient2) OnChange(ctx context.Context, name string, sync ServiceAccountChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &serviceAccountLifecycleDelegate{update: sync})
}

func (n *serviceAccountClient2) OnRemove(ctx context.Context, name string, sync ServiceAccountChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &serviceAccountLifecycleDelegate{remove: sync})
}

func (n *serviceAccountClientCache) Index(name string, indexer ServiceAccountIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*v1.ServiceAccount); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *serviceAccountClientCache) GetIndexed(name, key string) ([]*v1.ServiceAccount, error) {
	var result []*v1.ServiceAccount
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*v1.ServiceAccount); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *serviceAccountClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type serviceAccountLifecycleDelegate struct {
	create ServiceAccountChangeHandlerFunc
	update ServiceAccountChangeHandlerFunc
	remove ServiceAccountChangeHandlerFunc
}

func (n *serviceAccountLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *serviceAccountLifecycleDelegate) Create(obj *v1.ServiceAccount) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *serviceAccountLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *serviceAccountLifecycleDelegate) Remove(obj *v1.ServiceAccount) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *serviceAccountLifecycleDelegate) Updated(obj *v1.ServiceAccount) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
