package v1

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"k8s.io/api/rbac/v1"
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
	ClusterRoleBindingGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterRoleBinding",
	}
	ClusterRoleBindingResource = metav1.APIResource{
		Name:         "clusterrolebindings",
		SingularName: "clusterrolebinding",
		Namespaced:   false,
		Kind:         ClusterRoleBindingGroupVersionKind.Kind,
	}
)

func NewClusterRoleBinding(namespace, name string, obj v1.ClusterRoleBinding) *v1.ClusterRoleBinding {
	obj.APIVersion, obj.Kind = ClusterRoleBindingGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ClusterRoleBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.ClusterRoleBinding
}

type ClusterRoleBindingHandlerFunc func(key string, obj *v1.ClusterRoleBinding) (runtime.Object, error)

type ClusterRoleBindingChangeHandlerFunc func(obj *v1.ClusterRoleBinding) (runtime.Object, error)

type ClusterRoleBindingLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.ClusterRoleBinding, err error)
	Get(namespace, name string) (*v1.ClusterRoleBinding, error)
}

type ClusterRoleBindingController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ClusterRoleBindingLister
	AddHandler(ctx context.Context, name string, handler ClusterRoleBindingHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ClusterRoleBindingHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ClusterRoleBindingInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ClusterRoleBinding, error)
	Get(name string, opts metav1.GetOptions) (*v1.ClusterRoleBinding, error)
	Update(*v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ClusterRoleBindingList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterRoleBindingController
	AddHandler(ctx context.Context, name string, sync ClusterRoleBindingHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ClusterRoleBindingLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterRoleBindingHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterRoleBindingLifecycle)
}

type clusterRoleBindingLister struct {
	controller *clusterRoleBindingController
}

func (l *clusterRoleBindingLister) List(namespace string, selector labels.Selector) (ret []*v1.ClusterRoleBinding, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.ClusterRoleBinding))
	})
	return
}

func (l *clusterRoleBindingLister) Get(namespace, name string) (*v1.ClusterRoleBinding, error) {
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
			Group:    ClusterRoleBindingGroupVersionKind.Group,
			Resource: "clusterRoleBinding",
		}, key)
	}
	return obj.(*v1.ClusterRoleBinding), nil
}

type clusterRoleBindingController struct {
	controller.GenericController
}

func (c *clusterRoleBindingController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterRoleBindingController) Lister() ClusterRoleBindingLister {
	return &clusterRoleBindingLister{
		controller: c,
	}
}

func (c *clusterRoleBindingController) AddHandler(ctx context.Context, name string, handler ClusterRoleBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ClusterRoleBinding); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterRoleBindingController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ClusterRoleBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ClusterRoleBinding); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type clusterRoleBindingFactory struct {
}

func (c clusterRoleBindingFactory) Object() runtime.Object {
	return &v1.ClusterRoleBinding{}
}

func (c clusterRoleBindingFactory) List() runtime.Object {
	return &ClusterRoleBindingList{}
}

func (s *clusterRoleBindingClient) Controller() ClusterRoleBindingController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.clusterRoleBindingControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ClusterRoleBindingGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &clusterRoleBindingController{
		GenericController: genericController,
	}

	s.client.clusterRoleBindingControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type clusterRoleBindingClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterRoleBindingController
}

func (s *clusterRoleBindingClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterRoleBindingClient) Create(o *v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.ClusterRoleBinding), err
}

func (s *clusterRoleBindingClient) Get(name string, opts metav1.GetOptions) (*v1.ClusterRoleBinding, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.ClusterRoleBinding), err
}

func (s *clusterRoleBindingClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ClusterRoleBinding, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.ClusterRoleBinding), err
}

func (s *clusterRoleBindingClient) Update(o *v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.ClusterRoleBinding), err
}

func (s *clusterRoleBindingClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterRoleBindingClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterRoleBindingClient) List(opts metav1.ListOptions) (*ClusterRoleBindingList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ClusterRoleBindingList), err
}

func (s *clusterRoleBindingClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterRoleBindingClient) Patch(o *v1.ClusterRoleBinding, patchType types.PatchType, data []byte, subresources ...string) (*v1.ClusterRoleBinding, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.ClusterRoleBinding), err
}

func (s *clusterRoleBindingClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterRoleBindingClient) AddHandler(ctx context.Context, name string, sync ClusterRoleBindingHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterRoleBindingClient) AddLifecycle(ctx context.Context, name string, lifecycle ClusterRoleBindingLifecycle) {
	sync := NewClusterRoleBindingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterRoleBindingClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterRoleBindingHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterRoleBindingClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterRoleBindingLifecycle) {
	sync := NewClusterRoleBindingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

type ClusterRoleBindingIndexer func(obj *v1.ClusterRoleBinding) ([]string, error)

type ClusterRoleBindingClientCache interface {
	Get(namespace, name string) (*v1.ClusterRoleBinding, error)
	List(namespace string, selector labels.Selector) ([]*v1.ClusterRoleBinding, error)

	Index(name string, indexer ClusterRoleBindingIndexer)
	GetIndexed(name, key string) ([]*v1.ClusterRoleBinding, error)
}

type ClusterRoleBindingClient interface {
	Create(*v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error)
	Get(namespace, name string, opts metav1.GetOptions) (*v1.ClusterRoleBinding, error)
	Update(*v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*ClusterRoleBindingList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() ClusterRoleBindingClientCache

	OnCreate(ctx context.Context, name string, sync ClusterRoleBindingChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync ClusterRoleBindingChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync ClusterRoleBindingChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() ClusterRoleBindingInterface
}

type clusterRoleBindingClientCache struct {
	client *clusterRoleBindingClient2
}

type clusterRoleBindingClient2 struct {
	iface      ClusterRoleBindingInterface
	controller ClusterRoleBindingController
}

func (n *clusterRoleBindingClient2) Interface() ClusterRoleBindingInterface {
	return n.iface
}

func (n *clusterRoleBindingClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *clusterRoleBindingClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *clusterRoleBindingClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *clusterRoleBindingClient2) Create(obj *v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error) {
	return n.iface.Create(obj)
}

func (n *clusterRoleBindingClient2) Get(namespace, name string, opts metav1.GetOptions) (*v1.ClusterRoleBinding, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *clusterRoleBindingClient2) Update(obj *v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error) {
	return n.iface.Update(obj)
}

func (n *clusterRoleBindingClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *clusterRoleBindingClient2) List(namespace string, opts metav1.ListOptions) (*ClusterRoleBindingList, error) {
	return n.iface.List(opts)
}

func (n *clusterRoleBindingClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *clusterRoleBindingClientCache) Get(namespace, name string) (*v1.ClusterRoleBinding, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *clusterRoleBindingClientCache) List(namespace string, selector labels.Selector) ([]*v1.ClusterRoleBinding, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *clusterRoleBindingClient2) Cache() ClusterRoleBindingClientCache {
	n.loadController()
	return &clusterRoleBindingClientCache{
		client: n,
	}
}

func (n *clusterRoleBindingClient2) OnCreate(ctx context.Context, name string, sync ClusterRoleBindingChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &clusterRoleBindingLifecycleDelegate{create: sync})
}

func (n *clusterRoleBindingClient2) OnChange(ctx context.Context, name string, sync ClusterRoleBindingChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &clusterRoleBindingLifecycleDelegate{update: sync})
}

func (n *clusterRoleBindingClient2) OnRemove(ctx context.Context, name string, sync ClusterRoleBindingChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &clusterRoleBindingLifecycleDelegate{remove: sync})
}

func (n *clusterRoleBindingClientCache) Index(name string, indexer ClusterRoleBindingIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*v1.ClusterRoleBinding); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *clusterRoleBindingClientCache) GetIndexed(name, key string) ([]*v1.ClusterRoleBinding, error) {
	var result []*v1.ClusterRoleBinding
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*v1.ClusterRoleBinding); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *clusterRoleBindingClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type clusterRoleBindingLifecycleDelegate struct {
	create ClusterRoleBindingChangeHandlerFunc
	update ClusterRoleBindingChangeHandlerFunc
	remove ClusterRoleBindingChangeHandlerFunc
}

func (n *clusterRoleBindingLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *clusterRoleBindingLifecycleDelegate) Create(obj *v1.ClusterRoleBinding) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *clusterRoleBindingLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *clusterRoleBindingLifecycleDelegate) Remove(obj *v1.ClusterRoleBinding) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *clusterRoleBindingLifecycleDelegate) Updated(obj *v1.ClusterRoleBinding) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
