package v1

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	v1 "k8s.io/api/apps/v1"
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
	DaemonSetGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "DaemonSet",
	}
	DaemonSetResource = metav1.APIResource{
		Name:         "daemonsets",
		SingularName: "daemonset",
		Namespaced:   true,

		Kind: DaemonSetGroupVersionKind.Kind,
	}
)

func NewDaemonSet(namespace, name string, obj v1.DaemonSet) *v1.DaemonSet {
	obj.APIVersion, obj.Kind = DaemonSetGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type DaemonSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.DaemonSet
}

type DaemonSetHandlerFunc func(key string, obj *v1.DaemonSet) (runtime.Object, error)

type DaemonSetChangeHandlerFunc func(obj *v1.DaemonSet) (runtime.Object, error)

type DaemonSetLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.DaemonSet, err error)
	Get(namespace, name string) (*v1.DaemonSet, error)
}

type DaemonSetController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() DaemonSetLister
	AddHandler(ctx context.Context, name string, handler DaemonSetHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler DaemonSetHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type DaemonSetInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.DaemonSet) (*v1.DaemonSet, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.DaemonSet, error)
	Get(name string, opts metav1.GetOptions) (*v1.DaemonSet, error)
	Update(*v1.DaemonSet) (*v1.DaemonSet, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*DaemonSetList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() DaemonSetController
	AddHandler(ctx context.Context, name string, sync DaemonSetHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle DaemonSetLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync DaemonSetHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle DaemonSetLifecycle)
}

type daemonSetLister struct {
	controller *daemonSetController
}

func (l *daemonSetLister) List(namespace string, selector labels.Selector) (ret []*v1.DaemonSet, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.DaemonSet))
	})
	return
}

func (l *daemonSetLister) Get(namespace, name string) (*v1.DaemonSet, error) {
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
			Group:    DaemonSetGroupVersionKind.Group,
			Resource: "daemonSet",
		}, key)
	}
	return obj.(*v1.DaemonSet), nil
}

type daemonSetController struct {
	controller.GenericController
}

func (c *daemonSetController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *daemonSetController) Lister() DaemonSetLister {
	return &daemonSetLister{
		controller: c,
	}
}

func (c *daemonSetController) AddHandler(ctx context.Context, name string, handler DaemonSetHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.DaemonSet); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *daemonSetController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler DaemonSetHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.DaemonSet); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type daemonSetFactory struct {
}

func (c daemonSetFactory) Object() runtime.Object {
	return &v1.DaemonSet{}
}

func (c daemonSetFactory) List() runtime.Object {
	return &DaemonSetList{}
}

func (s *daemonSetClient) Controller() DaemonSetController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.daemonSetControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(DaemonSetGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &daemonSetController{
		GenericController: genericController,
	}

	s.client.daemonSetControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type daemonSetClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   DaemonSetController
}

func (s *daemonSetClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *daemonSetClient) Create(o *v1.DaemonSet) (*v1.DaemonSet, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.DaemonSet), err
}

func (s *daemonSetClient) Get(name string, opts metav1.GetOptions) (*v1.DaemonSet, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.DaemonSet), err
}

func (s *daemonSetClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.DaemonSet, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.DaemonSet), err
}

func (s *daemonSetClient) Update(o *v1.DaemonSet) (*v1.DaemonSet, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.DaemonSet), err
}

func (s *daemonSetClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *daemonSetClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *daemonSetClient) List(opts metav1.ListOptions) (*DaemonSetList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*DaemonSetList), err
}

func (s *daemonSetClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *daemonSetClient) Patch(o *v1.DaemonSet, patchType types.PatchType, data []byte, subresources ...string) (*v1.DaemonSet, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.DaemonSet), err
}

func (s *daemonSetClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *daemonSetClient) AddHandler(ctx context.Context, name string, sync DaemonSetHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *daemonSetClient) AddLifecycle(ctx context.Context, name string, lifecycle DaemonSetLifecycle) {
	sync := NewDaemonSetLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *daemonSetClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync DaemonSetHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *daemonSetClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle DaemonSetLifecycle) {
	sync := NewDaemonSetLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

type DaemonSetIndexer func(obj *v1.DaemonSet) ([]string, error)

type DaemonSetClientCache interface {
	Get(namespace, name string) (*v1.DaemonSet, error)
	List(namespace string, selector labels.Selector) ([]*v1.DaemonSet, error)

	Index(name string, indexer DaemonSetIndexer)
	GetIndexed(name, key string) ([]*v1.DaemonSet, error)
}

type DaemonSetClient interface {
	Create(*v1.DaemonSet) (*v1.DaemonSet, error)
	Get(namespace, name string, opts metav1.GetOptions) (*v1.DaemonSet, error)
	Update(*v1.DaemonSet) (*v1.DaemonSet, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*DaemonSetList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() DaemonSetClientCache

	OnCreate(ctx context.Context, name string, sync DaemonSetChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync DaemonSetChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync DaemonSetChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() DaemonSetInterface
}

type daemonSetClientCache struct {
	client *daemonSetClient2
}

type daemonSetClient2 struct {
	iface      DaemonSetInterface
	controller DaemonSetController
}

func (n *daemonSetClient2) Interface() DaemonSetInterface {
	return n.iface
}

func (n *daemonSetClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *daemonSetClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *daemonSetClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *daemonSetClient2) Create(obj *v1.DaemonSet) (*v1.DaemonSet, error) {
	return n.iface.Create(obj)
}

func (n *daemonSetClient2) Get(namespace, name string, opts metav1.GetOptions) (*v1.DaemonSet, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *daemonSetClient2) Update(obj *v1.DaemonSet) (*v1.DaemonSet, error) {
	return n.iface.Update(obj)
}

func (n *daemonSetClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *daemonSetClient2) List(namespace string, opts metav1.ListOptions) (*DaemonSetList, error) {
	return n.iface.List(opts)
}

func (n *daemonSetClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *daemonSetClientCache) Get(namespace, name string) (*v1.DaemonSet, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *daemonSetClientCache) List(namespace string, selector labels.Selector) ([]*v1.DaemonSet, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *daemonSetClient2) Cache() DaemonSetClientCache {
	n.loadController()
	return &daemonSetClientCache{
		client: n,
	}
}

func (n *daemonSetClient2) OnCreate(ctx context.Context, name string, sync DaemonSetChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &daemonSetLifecycleDelegate{create: sync})
}

func (n *daemonSetClient2) OnChange(ctx context.Context, name string, sync DaemonSetChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &daemonSetLifecycleDelegate{update: sync})
}

func (n *daemonSetClient2) OnRemove(ctx context.Context, name string, sync DaemonSetChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &daemonSetLifecycleDelegate{remove: sync})
}

func (n *daemonSetClientCache) Index(name string, indexer DaemonSetIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*v1.DaemonSet); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *daemonSetClientCache) GetIndexed(name, key string) ([]*v1.DaemonSet, error) {
	var result []*v1.DaemonSet
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*v1.DaemonSet); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *daemonSetClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type daemonSetLifecycleDelegate struct {
	create DaemonSetChangeHandlerFunc
	update DaemonSetChangeHandlerFunc
	remove DaemonSetChangeHandlerFunc
}

func (n *daemonSetLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *daemonSetLifecycleDelegate) Create(obj *v1.DaemonSet) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *daemonSetLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *daemonSetLifecycleDelegate) Remove(obj *v1.DaemonSet) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *daemonSetLifecycleDelegate) Updated(obj *v1.DaemonSet) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
