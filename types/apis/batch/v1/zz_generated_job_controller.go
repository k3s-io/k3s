package v1

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"k8s.io/api/batch/v1"
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
	JobGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Job",
	}
	JobResource = metav1.APIResource{
		Name:         "jobs",
		SingularName: "job",
		Namespaced:   true,

		Kind: JobGroupVersionKind.Kind,
	}
)

func NewJob(namespace, name string, obj v1.Job) *v1.Job {
	obj.APIVersion, obj.Kind = JobGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type JobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.Job
}

type JobHandlerFunc func(key string, obj *v1.Job) (runtime.Object, error)

type JobChangeHandlerFunc func(obj *v1.Job) (runtime.Object, error)

type JobLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Job, err error)
	Get(namespace, name string) (*v1.Job, error)
}

type JobController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() JobLister
	AddHandler(ctx context.Context, name string, handler JobHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler JobHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type JobInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.Job) (*v1.Job, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Job, error)
	Get(name string, opts metav1.GetOptions) (*v1.Job, error)
	Update(*v1.Job) (*v1.Job, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*JobList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() JobController
	AddHandler(ctx context.Context, name string, sync JobHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle JobLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync JobHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle JobLifecycle)
}

type jobLister struct {
	controller *jobController
}

func (l *jobLister) List(namespace string, selector labels.Selector) (ret []*v1.Job, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.Job))
	})
	return
}

func (l *jobLister) Get(namespace, name string) (*v1.Job, error) {
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
			Group:    JobGroupVersionKind.Group,
			Resource: "job",
		}, key)
	}
	return obj.(*v1.Job), nil
}

type jobController struct {
	controller.GenericController
}

func (c *jobController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *jobController) Lister() JobLister {
	return &jobLister{
		controller: c,
	}
}

func (c *jobController) AddHandler(ctx context.Context, name string, handler JobHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Job); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *jobController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler JobHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Job); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type jobFactory struct {
}

func (c jobFactory) Object() runtime.Object {
	return &v1.Job{}
}

func (c jobFactory) List() runtime.Object {
	return &JobList{}
}

func (s *jobClient) Controller() JobController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.jobControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(JobGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &jobController{
		GenericController: genericController,
	}

	s.client.jobControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type jobClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   JobController
}

func (s *jobClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *jobClient) Create(o *v1.Job) (*v1.Job, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.Job), err
}

func (s *jobClient) Get(name string, opts metav1.GetOptions) (*v1.Job, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.Job), err
}

func (s *jobClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Job, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.Job), err
}

func (s *jobClient) Update(o *v1.Job) (*v1.Job, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.Job), err
}

func (s *jobClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *jobClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *jobClient) List(opts metav1.ListOptions) (*JobList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*JobList), err
}

func (s *jobClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *jobClient) Patch(o *v1.Job, patchType types.PatchType, data []byte, subresources ...string) (*v1.Job, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.Job), err
}

func (s *jobClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *jobClient) AddHandler(ctx context.Context, name string, sync JobHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *jobClient) AddLifecycle(ctx context.Context, name string, lifecycle JobLifecycle) {
	sync := NewJobLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *jobClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync JobHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *jobClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle JobLifecycle) {
	sync := NewJobLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

type JobIndexer func(obj *v1.Job) ([]string, error)

type JobClientCache interface {
	Get(namespace, name string) (*v1.Job, error)
	List(namespace string, selector labels.Selector) ([]*v1.Job, error)

	Index(name string, indexer JobIndexer)
	GetIndexed(name, key string) ([]*v1.Job, error)
}

type JobClient interface {
	Create(*v1.Job) (*v1.Job, error)
	Get(namespace, name string, opts metav1.GetOptions) (*v1.Job, error)
	Update(*v1.Job) (*v1.Job, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*JobList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() JobClientCache

	OnCreate(ctx context.Context, name string, sync JobChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync JobChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync JobChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() JobInterface
}

type jobClientCache struct {
	client *jobClient2
}

type jobClient2 struct {
	iface      JobInterface
	controller JobController
}

func (n *jobClient2) Interface() JobInterface {
	return n.iface
}

func (n *jobClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *jobClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *jobClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *jobClient2) Create(obj *v1.Job) (*v1.Job, error) {
	return n.iface.Create(obj)
}

func (n *jobClient2) Get(namespace, name string, opts metav1.GetOptions) (*v1.Job, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *jobClient2) Update(obj *v1.Job) (*v1.Job, error) {
	return n.iface.Update(obj)
}

func (n *jobClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *jobClient2) List(namespace string, opts metav1.ListOptions) (*JobList, error) {
	return n.iface.List(opts)
}

func (n *jobClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *jobClientCache) Get(namespace, name string) (*v1.Job, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *jobClientCache) List(namespace string, selector labels.Selector) ([]*v1.Job, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *jobClient2) Cache() JobClientCache {
	n.loadController()
	return &jobClientCache{
		client: n,
	}
}

func (n *jobClient2) OnCreate(ctx context.Context, name string, sync JobChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &jobLifecycleDelegate{create: sync})
}

func (n *jobClient2) OnChange(ctx context.Context, name string, sync JobChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &jobLifecycleDelegate{update: sync})
}

func (n *jobClient2) OnRemove(ctx context.Context, name string, sync JobChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &jobLifecycleDelegate{remove: sync})
}

func (n *jobClientCache) Index(name string, indexer JobIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*v1.Job); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *jobClientCache) GetIndexed(name, key string) ([]*v1.Job, error) {
	var result []*v1.Job
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*v1.Job); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *jobClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type jobLifecycleDelegate struct {
	create JobChangeHandlerFunc
	update JobChangeHandlerFunc
	remove JobChangeHandlerFunc
}

func (n *jobLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *jobLifecycleDelegate) Create(obj *v1.Job) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *jobLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *jobLifecycleDelegate) Remove(obj *v1.Job) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *jobLifecycleDelegate) Updated(obj *v1.Job) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
