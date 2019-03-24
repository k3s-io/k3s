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
	PodGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Pod",
	}
	PodResource = metav1.APIResource{
		Name:         "pods",
		SingularName: "pod",
		Namespaced:   true,

		Kind: PodGroupVersionKind.Kind,
	}
)

func NewPod(namespace, name string, obj v1.Pod) *v1.Pod {
	obj.APIVersion, obj.Kind = PodGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type PodList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.Pod
}

type PodHandlerFunc func(key string, obj *v1.Pod) (runtime.Object, error)

type PodChangeHandlerFunc func(obj *v1.Pod) (runtime.Object, error)

type PodLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Pod, err error)
	Get(namespace, name string) (*v1.Pod, error)
}

type PodController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() PodLister
	AddHandler(ctx context.Context, name string, handler PodHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler PodHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type PodInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.Pod) (*v1.Pod, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Pod, error)
	Get(name string, opts metav1.GetOptions) (*v1.Pod, error)
	Update(*v1.Pod) (*v1.Pod, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*PodList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() PodController
	AddHandler(ctx context.Context, name string, sync PodHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle PodLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PodHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PodLifecycle)
}

type podLister struct {
	controller *podController
}

func (l *podLister) List(namespace string, selector labels.Selector) (ret []*v1.Pod, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.Pod))
	})
	return
}

func (l *podLister) Get(namespace, name string) (*v1.Pod, error) {
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
			Group:    PodGroupVersionKind.Group,
			Resource: "pod",
		}, key)
	}
	return obj.(*v1.Pod), nil
}

type podController struct {
	controller.GenericController
}

func (c *podController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *podController) Lister() PodLister {
	return &podLister{
		controller: c,
	}
}

func (c *podController) AddHandler(ctx context.Context, name string, handler PodHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Pod); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *podController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler PodHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Pod); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type podFactory struct {
}

func (c podFactory) Object() runtime.Object {
	return &v1.Pod{}
}

func (c podFactory) List() runtime.Object {
	return &PodList{}
}

func (s *podClient) Controller() PodController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.podControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(PodGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &podController{
		GenericController: genericController,
	}

	s.client.podControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type podClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   PodController
}

func (s *podClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *podClient) Create(o *v1.Pod) (*v1.Pod, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.Pod), err
}

func (s *podClient) Get(name string, opts metav1.GetOptions) (*v1.Pod, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.Pod), err
}

func (s *podClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Pod, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.Pod), err
}

func (s *podClient) Update(o *v1.Pod) (*v1.Pod, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.Pod), err
}

func (s *podClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *podClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *podClient) List(opts metav1.ListOptions) (*PodList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*PodList), err
}

func (s *podClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *podClient) Patch(o *v1.Pod, patchType types.PatchType, data []byte, subresources ...string) (*v1.Pod, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.Pod), err
}

func (s *podClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *podClient) AddHandler(ctx context.Context, name string, sync PodHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *podClient) AddLifecycle(ctx context.Context, name string, lifecycle PodLifecycle) {
	sync := NewPodLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *podClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PodHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *podClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PodLifecycle) {
	sync := NewPodLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

type PodIndexer func(obj *v1.Pod) ([]string, error)

type PodClientCache interface {
	Get(namespace, name string) (*v1.Pod, error)
	List(namespace string, selector labels.Selector) ([]*v1.Pod, error)

	Index(name string, indexer PodIndexer)
	GetIndexed(name, key string) ([]*v1.Pod, error)
}

type PodClient interface {
	Create(*v1.Pod) (*v1.Pod, error)
	Get(namespace, name string, opts metav1.GetOptions) (*v1.Pod, error)
	Update(*v1.Pod) (*v1.Pod, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*PodList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() PodClientCache

	OnCreate(ctx context.Context, name string, sync PodChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync PodChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync PodChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() PodInterface
}

type podClientCache struct {
	client *podClient2
}

type podClient2 struct {
	iface      PodInterface
	controller PodController
}

func (n *podClient2) Interface() PodInterface {
	return n.iface
}

func (n *podClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *podClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *podClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *podClient2) Create(obj *v1.Pod) (*v1.Pod, error) {
	return n.iface.Create(obj)
}

func (n *podClient2) Get(namespace, name string, opts metav1.GetOptions) (*v1.Pod, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *podClient2) Update(obj *v1.Pod) (*v1.Pod, error) {
	return n.iface.Update(obj)
}

func (n *podClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *podClient2) List(namespace string, opts metav1.ListOptions) (*PodList, error) {
	return n.iface.List(opts)
}

func (n *podClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *podClientCache) Get(namespace, name string) (*v1.Pod, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *podClientCache) List(namespace string, selector labels.Selector) ([]*v1.Pod, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *podClient2) Cache() PodClientCache {
	n.loadController()
	return &podClientCache{
		client: n,
	}
}

func (n *podClient2) OnCreate(ctx context.Context, name string, sync PodChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &podLifecycleDelegate{create: sync})
}

func (n *podClient2) OnChange(ctx context.Context, name string, sync PodChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &podLifecycleDelegate{update: sync})
}

func (n *podClient2) OnRemove(ctx context.Context, name string, sync PodChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &podLifecycleDelegate{remove: sync})
}

func (n *podClientCache) Index(name string, indexer PodIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*v1.Pod); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *podClientCache) GetIndexed(name, key string) ([]*v1.Pod, error) {
	var result []*v1.Pod
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*v1.Pod); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *podClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type podLifecycleDelegate struct {
	create PodChangeHandlerFunc
	update PodChangeHandlerFunc
	remove PodChangeHandlerFunc
}

func (n *podLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *podLifecycleDelegate) Create(obj *v1.Pod) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *podLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *podLifecycleDelegate) Remove(obj *v1.Pod) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *podLifecycleDelegate) Updated(obj *v1.Pod) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
