package generator

var controllerTemplate = `package {{.schema.Version.Version}}

import (
	"context"

	{{.importPackage}}
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/controller"
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
	{{.schema.CodeName}}GroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "{{.schema.CodeName}}",
	}
	{{.schema.CodeName}}Resource = metav1.APIResource{
		Name:         "{{.schema.PluralName | toLower}}",
		SingularName: "{{.schema.ID | toLower}}",
{{- if eq .schema.Scope "namespace" }}
		Namespaced:   true,
{{ else }}
		Namespaced:   false,
{{- end }}
		Kind:         {{.schema.CodeName}}GroupVersionKind.Kind,
	}
)

func New{{.schema.CodeName}}(namespace, name string, obj {{.prefix}}{{.schema.CodeName}}) *{{.prefix}}{{.schema.CodeName}} {
	obj.APIVersion, obj.Kind = {{.schema.CodeName}}GroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type {{.schema.CodeName}}List struct {
	metav1.TypeMeta   %BACK%json:",inline"%BACK%
	metav1.ListMeta   %BACK%json:"metadata,omitempty"%BACK%
	Items             []{{.prefix}}{{.schema.CodeName}}
}

type {{.schema.CodeName}}HandlerFunc func(key string, obj *{{.prefix}}{{.schema.CodeName}}) (runtime.Object, error)

type {{.schema.CodeName}}ChangeHandlerFunc func(obj *{{.prefix}}{{.schema.CodeName}}) (runtime.Object, error)

type {{.schema.CodeName}}Lister interface {
	List(namespace string, selector labels.Selector) (ret []*{{.prefix}}{{.schema.CodeName}}, err error)
	Get(namespace, name string) (*{{.prefix}}{{.schema.CodeName}}, error)
}

type {{.schema.CodeName}}Controller interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() {{.schema.CodeName}}Lister
	AddHandler(ctx context.Context, name string, handler {{.schema.CodeName}}HandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler {{.schema.CodeName}}HandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type {{.schema.CodeName}}Interface interface {
    ObjectClient() *objectclient.ObjectClient
	Create(*{{.prefix}}{{.schema.CodeName}}) (*{{.prefix}}{{.schema.CodeName}}, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*{{.prefix}}{{.schema.CodeName}}, error)
	Get(name string, opts metav1.GetOptions) (*{{.prefix}}{{.schema.CodeName}}, error)
	Update(*{{.prefix}}{{.schema.CodeName}}) (*{{.prefix}}{{.schema.CodeName}}, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*{{.schema.CodeName}}List, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() {{.schema.CodeName}}Controller
	AddHandler(ctx context.Context, name string, sync {{.schema.CodeName}}HandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle {{.schema.CodeName}}Lifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync {{.schema.CodeName}}HandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle {{.schema.CodeName}}Lifecycle)
}

type {{.schema.ID}}Lister struct {
	controller *{{.schema.ID}}Controller
}

func (l *{{.schema.ID}}Lister) List(namespace string, selector labels.Selector) (ret []*{{.prefix}}{{.schema.CodeName}}, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*{{.prefix}}{{.schema.CodeName}}))
	})
	return
}

func (l *{{.schema.ID}}Lister) Get(namespace, name string) (*{{.prefix}}{{.schema.CodeName}}, error) {
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
			Group: {{.schema.CodeName}}GroupVersionKind.Group,
			Resource: "{{.schema.ID}}",
		}, key)
	}
	return obj.(*{{.prefix}}{{.schema.CodeName}}), nil
}

type {{.schema.ID}}Controller struct {
	controller.GenericController
}

func (c *{{.schema.ID}}Controller) Generic() controller.GenericController {
	return c.GenericController
}

func (c *{{.schema.ID}}Controller) Lister() {{.schema.CodeName}}Lister {
	return &{{.schema.ID}}Lister{
		controller: c,
	}
}


func (c *{{.schema.ID}}Controller) AddHandler(ctx context.Context, name string, handler {{.schema.CodeName}}HandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*{{.prefix}}{{.schema.CodeName}}); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *{{.schema.ID}}Controller) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler {{.schema.CodeName}}HandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*{{.prefix}}{{.schema.CodeName}}); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type {{.schema.ID}}Factory struct {
}

func (c {{.schema.ID}}Factory) Object() runtime.Object {
	return &{{.prefix}}{{.schema.CodeName}}{}
}

func (c {{.schema.ID}}Factory) List() runtime.Object {
	return &{{.schema.CodeName}}List{}
}

func (s *{{.schema.ID}}Client) Controller() {{.schema.CodeName}}Controller {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.{{.schema.ID}}Controllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController({{.schema.CodeName}}GroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &{{.schema.ID}}Controller{
		GenericController: genericController,
	}

	s.client.{{.schema.ID}}Controllers[s.ns] = c
    s.client.starters = append(s.client.starters, c)

	return c
}

type {{.schema.ID}}Client struct {
	client *Client
	ns string
	objectClient *objectclient.ObjectClient
	controller   {{.schema.CodeName}}Controller
}

func (s *{{.schema.ID}}Client) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *{{.schema.ID}}Client) Create(o *{{.prefix}}{{.schema.CodeName}}) (*{{.prefix}}{{.schema.CodeName}}, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*{{.prefix}}{{.schema.CodeName}}), err
}

func (s *{{.schema.ID}}Client) Get(name string, opts metav1.GetOptions) (*{{.prefix}}{{.schema.CodeName}}, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*{{.prefix}}{{.schema.CodeName}}), err
}

func (s *{{.schema.ID}}Client) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*{{.prefix}}{{.schema.CodeName}}, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*{{.prefix}}{{.schema.CodeName}}), err
}

func (s *{{.schema.ID}}Client) Update(o *{{.prefix}}{{.schema.CodeName}}) (*{{.prefix}}{{.schema.CodeName}}, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*{{.prefix}}{{.schema.CodeName}}), err
}

func (s *{{.schema.ID}}Client) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *{{.schema.ID}}Client) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *{{.schema.ID}}Client) List(opts metav1.ListOptions) (*{{.schema.CodeName}}List, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*{{.schema.CodeName}}List), err
}

func (s *{{.schema.ID}}Client) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *{{.schema.ID}}Client) Patch(o *{{.prefix}}{{.schema.CodeName}}, patchType types.PatchType, data []byte, subresources ...string) (*{{.prefix}}{{.schema.CodeName}}, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*{{.prefix}}{{.schema.CodeName}}), err
}

func (s *{{.schema.ID}}Client) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *{{.schema.ID}}Client) AddHandler(ctx context.Context, name string, sync {{.schema.CodeName}}HandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *{{.schema.ID}}Client) AddLifecycle(ctx context.Context, name string, lifecycle {{.schema.CodeName}}Lifecycle) {
	sync := New{{.schema.CodeName}}LifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *{{.schema.ID}}Client) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync {{.schema.CodeName}}HandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *{{.schema.ID}}Client) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle {{.schema.CodeName}}Lifecycle) {
	sync := New{{.schema.CodeName}}LifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

type {{.schema.CodeName}}Indexer func(obj *{{.prefix}}{{.schema.CodeName}}) ([]string, error)

type {{.schema.CodeName}}ClientCache interface {
	Get(namespace, name string) (*{{.prefix}}{{.schema.CodeName}}, error)
	List(namespace string, selector labels.Selector) ([]*{{.prefix}}{{.schema.CodeName}}, error)

	Index(name string, indexer {{.schema.CodeName}}Indexer)
	GetIndexed(name, key string) ([]*{{.prefix}}{{.schema.CodeName}}, error)
}

type {{.schema.CodeName}}Client interface {
	Create(*{{.prefix}}{{.schema.CodeName}}) (*{{.prefix}}{{.schema.CodeName}}, error)
	Get(namespace, name string, opts metav1.GetOptions) (*{{.prefix}}{{.schema.CodeName}}, error)
	Update(*{{.prefix}}{{.schema.CodeName}}) (*{{.prefix}}{{.schema.CodeName}}, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*{{.schema.CodeName}}List, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() {{.schema.CodeName}}ClientCache

	OnCreate(ctx context.Context, name string, sync {{.schema.CodeName}}ChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync {{.schema.CodeName}}ChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync {{.schema.CodeName}}ChangeHandlerFunc)
    Enqueue(namespace, name string)

	Generic() controller.GenericController
    ObjectClient() *objectclient.ObjectClient
	Interface() {{.schema.CodeName}}Interface
}

type {{.schema.ID}}ClientCache struct {
	client *{{.schema.ID}}Client2
}

type {{.schema.ID}}Client2 struct {
	iface      {{.schema.CodeName}}Interface
	controller {{.schema.CodeName}}Controller
}

func (n *{{.schema.ID}}Client2) Interface() {{.schema.CodeName}}Interface {
	return n.iface
}

func (n *{{.schema.ID}}Client2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *{{.schema.ID}}Client2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *{{.schema.ID}}Client2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *{{.schema.ID}}Client2) Create(obj *{{.prefix}}{{.schema.CodeName}}) (*{{.prefix}}{{.schema.CodeName}}, error) {
	return n.iface.Create(obj)
}

func (n *{{.schema.ID}}Client2) Get(namespace, name string, opts metav1.GetOptions) (*{{.prefix}}{{.schema.CodeName}}, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *{{.schema.ID}}Client2) Update(obj *{{.prefix}}{{.schema.CodeName}}) (*{{.prefix}}{{.schema.CodeName}}, error) {
	return n.iface.Update(obj)
}

func (n *{{.schema.ID}}Client2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *{{.schema.ID}}Client2) List(namespace string, opts metav1.ListOptions) (*{{.schema.CodeName}}List, error) {
	return n.iface.List(opts)
}

func (n *{{.schema.ID}}Client2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *{{.schema.ID}}ClientCache) Get(namespace, name string) (*{{.prefix}}{{.schema.CodeName}}, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *{{.schema.ID}}ClientCache) List(namespace string, selector labels.Selector) ([]*{{.prefix}}{{.schema.CodeName}}, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *{{.schema.ID}}Client2) Cache() {{.schema.CodeName}}ClientCache {
	n.loadController()
	return &{{.schema.ID}}ClientCache{
		client: n,
	}
}

func (n *{{.schema.ID}}Client2) OnCreate(ctx context.Context, name string, sync {{.schema.CodeName}}ChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &{{.schema.ID}}LifecycleDelegate{create: sync})
}

func (n *{{.schema.ID}}Client2) OnChange(ctx context.Context, name string, sync {{.schema.CodeName}}ChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &{{.schema.ID}}LifecycleDelegate{update: sync})
}

func (n *{{.schema.ID}}Client2) OnRemove(ctx context.Context, name string, sync {{.schema.CodeName}}ChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &{{.schema.ID}}LifecycleDelegate{remove: sync})
}

func (n *{{.schema.ID}}ClientCache) Index(name string, indexer {{.schema.CodeName}}Indexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*{{.prefix}}{{.schema.CodeName}}); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *{{.schema.ID}}ClientCache) GetIndexed(name, key string) ([]*{{.prefix}}{{.schema.CodeName}}, error) {
	var result []*{{.prefix}}{{.schema.CodeName}}
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*{{.prefix}}{{.schema.CodeName}}); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *{{.schema.ID}}Client2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type {{.schema.ID}}LifecycleDelegate struct {
	create {{.schema.CodeName}}ChangeHandlerFunc
	update {{.schema.CodeName}}ChangeHandlerFunc
	remove {{.schema.CodeName}}ChangeHandlerFunc
}

func (n *{{.schema.ID}}LifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *{{.schema.ID}}LifecycleDelegate) Create(obj *{{.prefix}}{{.schema.CodeName}}) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *{{.schema.ID}}LifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *{{.schema.ID}}LifecycleDelegate) Remove(obj *{{.prefix}}{{.schema.CodeName}}) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *{{.schema.ID}}LifecycleDelegate) Updated(obj *{{.prefix}}{{.schema.CodeName}}) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
`
