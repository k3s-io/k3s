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
	HelmChartGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "HelmChart",
	}
	HelmChartResource = metav1.APIResource{
		Name:         "helmcharts",
		SingularName: "helmchart",
		Namespaced:   true,

		Kind: HelmChartGroupVersionKind.Kind,
	}
)

func NewHelmChart(namespace, name string, obj HelmChart) *HelmChart {
	obj.APIVersion, obj.Kind = HelmChartGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type HelmChartList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HelmChart
}

type HelmChartHandlerFunc func(key string, obj *HelmChart) (runtime.Object, error)

type HelmChartChangeHandlerFunc func(obj *HelmChart) (runtime.Object, error)

type HelmChartLister interface {
	List(namespace string, selector labels.Selector) (ret []*HelmChart, err error)
	Get(namespace, name string) (*HelmChart, error)
}

type HelmChartController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() HelmChartLister
	AddHandler(ctx context.Context, name string, handler HelmChartHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler HelmChartHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type HelmChartInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*HelmChart) (*HelmChart, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*HelmChart, error)
	Get(name string, opts metav1.GetOptions) (*HelmChart, error)
	Update(*HelmChart) (*HelmChart, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*HelmChartList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() HelmChartController
	AddHandler(ctx context.Context, name string, sync HelmChartHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle HelmChartLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync HelmChartHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle HelmChartLifecycle)
}

type helmChartLister struct {
	controller *helmChartController
}

func (l *helmChartLister) List(namespace string, selector labels.Selector) (ret []*HelmChart, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*HelmChart))
	})
	return
}

func (l *helmChartLister) Get(namespace, name string) (*HelmChart, error) {
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
			Group:    HelmChartGroupVersionKind.Group,
			Resource: "helmChart",
		}, key)
	}
	return obj.(*HelmChart), nil
}

type helmChartController struct {
	controller.GenericController
}

func (c *helmChartController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *helmChartController) Lister() HelmChartLister {
	return &helmChartLister{
		controller: c,
	}
}

func (c *helmChartController) AddHandler(ctx context.Context, name string, handler HelmChartHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*HelmChart); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *helmChartController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler HelmChartHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*HelmChart); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type helmChartFactory struct {
}

func (c helmChartFactory) Object() runtime.Object {
	return &HelmChart{}
}

func (c helmChartFactory) List() runtime.Object {
	return &HelmChartList{}
}

func (s *helmChartClient) Controller() HelmChartController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.helmChartControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(HelmChartGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &helmChartController{
		GenericController: genericController,
	}

	s.client.helmChartControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type helmChartClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   HelmChartController
}

func (s *helmChartClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *helmChartClient) Create(o *HelmChart) (*HelmChart, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*HelmChart), err
}

func (s *helmChartClient) Get(name string, opts metav1.GetOptions) (*HelmChart, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*HelmChart), err
}

func (s *helmChartClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*HelmChart, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*HelmChart), err
}

func (s *helmChartClient) Update(o *HelmChart) (*HelmChart, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*HelmChart), err
}

func (s *helmChartClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *helmChartClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *helmChartClient) List(opts metav1.ListOptions) (*HelmChartList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*HelmChartList), err
}

func (s *helmChartClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *helmChartClient) Patch(o *HelmChart, patchType types.PatchType, data []byte, subresources ...string) (*HelmChart, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*HelmChart), err
}

func (s *helmChartClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *helmChartClient) AddHandler(ctx context.Context, name string, sync HelmChartHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *helmChartClient) AddLifecycle(ctx context.Context, name string, lifecycle HelmChartLifecycle) {
	sync := NewHelmChartLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *helmChartClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync HelmChartHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *helmChartClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle HelmChartLifecycle) {
	sync := NewHelmChartLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

type HelmChartIndexer func(obj *HelmChart) ([]string, error)

type HelmChartClientCache interface {
	Get(namespace, name string) (*HelmChart, error)
	List(namespace string, selector labels.Selector) ([]*HelmChart, error)

	Index(name string, indexer HelmChartIndexer)
	GetIndexed(name, key string) ([]*HelmChart, error)
}

type HelmChartClient interface {
	Create(*HelmChart) (*HelmChart, error)
	Get(namespace, name string, opts metav1.GetOptions) (*HelmChart, error)
	Update(*HelmChart) (*HelmChart, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*HelmChartList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() HelmChartClientCache

	OnCreate(ctx context.Context, name string, sync HelmChartChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync HelmChartChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync HelmChartChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() HelmChartInterface
}

type helmChartClientCache struct {
	client *helmChartClient2
}

type helmChartClient2 struct {
	iface      HelmChartInterface
	controller HelmChartController
}

func (n *helmChartClient2) Interface() HelmChartInterface {
	return n.iface
}

func (n *helmChartClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *helmChartClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *helmChartClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *helmChartClient2) Create(obj *HelmChart) (*HelmChart, error) {
	return n.iface.Create(obj)
}

func (n *helmChartClient2) Get(namespace, name string, opts metav1.GetOptions) (*HelmChart, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *helmChartClient2) Update(obj *HelmChart) (*HelmChart, error) {
	return n.iface.Update(obj)
}

func (n *helmChartClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *helmChartClient2) List(namespace string, opts metav1.ListOptions) (*HelmChartList, error) {
	return n.iface.List(opts)
}

func (n *helmChartClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *helmChartClientCache) Get(namespace, name string) (*HelmChart, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *helmChartClientCache) List(namespace string, selector labels.Selector) ([]*HelmChart, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *helmChartClient2) Cache() HelmChartClientCache {
	n.loadController()
	return &helmChartClientCache{
		client: n,
	}
}

func (n *helmChartClient2) OnCreate(ctx context.Context, name string, sync HelmChartChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &helmChartLifecycleDelegate{create: sync})
}

func (n *helmChartClient2) OnChange(ctx context.Context, name string, sync HelmChartChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &helmChartLifecycleDelegate{update: sync})
}

func (n *helmChartClient2) OnRemove(ctx context.Context, name string, sync HelmChartChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &helmChartLifecycleDelegate{remove: sync})
}

func (n *helmChartClientCache) Index(name string, indexer HelmChartIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*HelmChart); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *helmChartClientCache) GetIndexed(name, key string) ([]*HelmChart, error) {
	var result []*HelmChart
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*HelmChart); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *helmChartClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type helmChartLifecycleDelegate struct {
	create HelmChartChangeHandlerFunc
	update HelmChartChangeHandlerFunc
	remove HelmChartChangeHandlerFunc
}

func (n *helmChartLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *helmChartLifecycleDelegate) Create(obj *HelmChart) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *helmChartLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *helmChartLifecycleDelegate) Remove(obj *HelmChart) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *helmChartLifecycleDelegate) Updated(obj *HelmChart) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
