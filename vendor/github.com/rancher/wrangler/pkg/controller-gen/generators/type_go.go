package generators

import (
	"fmt"
	"io"
	"strings"

	args2 "github.com/rancher/wrangler/pkg/controller-gen/args"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/code-generator/cmd/client-gen/generators/util"
	"k8s.io/gengo/args"
	"k8s.io/gengo/generator"
	"k8s.io/gengo/namer"
	"k8s.io/gengo/types"
)

func TypeGo(gv schema.GroupVersion, name *types.Name, args *args.GeneratorArgs, customArgs *args2.CustomArgs) generator.Generator {
	return &typeGo{
		name:       name,
		gv:         gv,
		args:       args,
		customArgs: customArgs,
		DefaultGen: generator.DefaultGen{
			OptionalName: strings.ToLower(name.Name),
		},
	}
}

type typeGo struct {
	generator.DefaultGen

	name       *types.Name
	gv         schema.GroupVersion
	args       *args.GeneratorArgs
	customArgs *args2.CustomArgs
}

func (f *typeGo) Imports(*generator.Context) []string {
	group := f.customArgs.Options.Groups[f.gv.Group]

	packages := []string{
		"metav1 \"k8s.io/apimachinery/pkg/apis/meta/v1\"",
		"k8s.io/apimachinery/pkg/api/errors",
		"k8s.io/apimachinery/pkg/labels",
		"k8s.io/apimachinery/pkg/runtime",
		"k8s.io/apimachinery/pkg/runtime/schema",
		"k8s.io/apimachinery/pkg/api/equality",
		"k8s.io/apimachinery/pkg/types",
		"utilruntime \"k8s.io/apimachinery/pkg/util/runtime\"",
		"k8s.io/apimachinery/pkg/watch",
		"k8s.io/client-go/tools/cache",
		"github.com/rancher/wrangler/pkg/apply",
		"github.com/rancher/wrangler/pkg/condition",
		fmt.Sprintf("%s \"%s\"", f.gv.Version, f.name.Package),
		GenericPackage,
		fmt.Sprintf("clientset \"%s/typed/%s/%s\"", group.ClientSetPackage, groupPackageName(f.gv.Group, group.PackageName), f.gv.Version),
		fmt.Sprintf("informers \"%s/%s/%s\"", group.InformersPackage, groupPackageName(f.gv.Group, group.PackageName), f.gv.Version),
		fmt.Sprintf("listers \"%s/%s/%s\"", group.ListersPackage, groupPackageName(f.gv.Group, group.PackageName), f.gv.Version),
	}

	return packages
}

func (f *typeGo) Init(c *generator.Context, w io.Writer) error {
	sw := generator.NewSnippetWriter(w, c, "{{", "}}")

	if err := f.DefaultGen.Init(c, w); err != nil {
		return err
	}

	t := c.Universe.Type(*f.name)
	m := map[string]interface{}{
		"type":       f.name.Name,
		"lowerName":  namer.IL(f.name.Name),
		"plural":     plural.Name(t),
		"version":    f.gv.Version,
		"namespaced": !util.MustParseClientGenTags(t.SecondClosestCommentLines).NonNamespaced,
		"hasStatus":  hasStatus(t),
		"statusType": statusType(t),
	}

	sw.Do(typeBody, m)
	return sw.Error()
}

func statusType(t *types.Type) string {
	for _, m := range t.Members {
		if m.Name == "Status" {
			return m.Type.Name.Name
		}
	}
	return ""
}

func hasStatus(t *types.Type) bool {
	for _, m := range t.Members {
		if m.Name == "Status" && m.Type.Name.Package == t.Name.Package {
			return true
		}
	}
	return false
}

var typeBody = `
type {{.type}}Handler func(string, *{{.version}}.{{.type}}) (*{{.version}}.{{.type}}, error)

type {{.type}}Controller interface {
    generic.ControllerMeta
	{{.type}}Client

	OnChange(ctx context.Context, name string, sync {{.type}}Handler)
	OnRemove(ctx context.Context, name string, sync {{.type}}Handler)
	Enqueue({{ if .namespaced}}namespace, {{end}}name string)
	EnqueueAfter({{ if .namespaced}}namespace, {{end}}name string, duration time.Duration)

	Cache() {{.type}}Cache
}

type {{.type}}Client interface {
	Create(*{{.version}}.{{.type}}) (*{{.version}}.{{.type}}, error)
	Update(*{{.version}}.{{.type}}) (*{{.version}}.{{.type}}, error)
{{ if .hasStatus -}}
	UpdateStatus(*{{.version}}.{{.type}}) (*{{.version}}.{{.type}}, error)
{{- end }}
	Delete({{ if .namespaced}}namespace, {{end}}name string, options *metav1.DeleteOptions) error
	Get({{ if .namespaced}}namespace, {{end}}name string, options metav1.GetOptions) (*{{.version}}.{{.type}}, error)
	List({{ if .namespaced}}namespace string, {{end}}opts metav1.ListOptions) (*{{.version}}.{{.type}}List, error)
	Watch({{ if .namespaced}}namespace string, {{end}}opts metav1.ListOptions) (watch.Interface, error)
	Patch({{ if .namespaced}}namespace, {{end}}name string, pt types.PatchType, data []byte, subresources ...string) (result *{{.version}}.{{.type}}, err error)
}

type {{.type}}Cache interface {
	Get({{ if .namespaced}}namespace, {{end}}name string) (*{{.version}}.{{.type}}, error)
	List({{ if .namespaced}}namespace string, {{end}}selector labels.Selector) ([]*{{.version}}.{{.type}}, error)

	AddIndexer(indexName string, indexer {{.type}}Indexer)
	GetByIndex(indexName, key string) ([]*{{.version}}.{{.type}}, error)
}

type {{.type}}Indexer func(obj *{{.version}}.{{.type}}) ([]string, error)

type {{.lowerName}}Controller struct {
	controllerManager *generic.ControllerManager
	clientGetter      clientset.{{.plural}}Getter
	informer          informers.{{.type}}Informer
	gvk               schema.GroupVersionKind
}

func New{{.type}}Controller(gvk schema.GroupVersionKind, controllerManager *generic.ControllerManager, clientGetter clientset.{{.plural}}Getter, informer informers.{{.type}}Informer) {{.type}}Controller {
	return &{{.lowerName}}Controller{
		controllerManager: controllerManager,
		clientGetter:      clientGetter,
		informer:          informer,
		gvk:               gvk,
	}
}

func From{{.type}}HandlerToHandler(sync {{.type}}Handler) generic.Handler {
	return func(key string, obj runtime.Object) (ret runtime.Object, err error) {
		var v *{{.version}}.{{.type}}
		if obj == nil {
			v, err = sync(key, nil)
		} else {
			v, err = sync(key, obj.(*{{.version}}.{{.type}}))
		}
		if v == nil {
			return nil, err
		}
		return v, err
	}
}

func (c *{{.lowerName}}Controller) Updater() generic.Updater {
	return func(obj runtime.Object) (runtime.Object, error) {
		newObj, err := c.Update(obj.(*{{.version}}.{{.type}}))
		if newObj == nil {
			return nil, err
		}
		return newObj, err
	}
}

func Update{{.type}}DeepCopyOnChange(client {{.type}}Client, obj *{{.version}}.{{.type}}, handler func(obj *{{.version}}.{{.type}}) (*{{.version}}.{{.type}}, error)) (*{{.version}}.{{.type}}, error) {
	if obj == nil {
		return obj, nil
	}

	copyObj := obj.DeepCopy()
	newObj, err := handler(copyObj)
	if newObj != nil {
		copyObj = newObj
	}
	if obj.ResourceVersion == copyObj.ResourceVersion && !equality.Semantic.DeepEqual(obj, copyObj) {
		return client.Update(copyObj)
	}

	return copyObj, err
}

func (c *{{.lowerName}}Controller) AddGenericHandler(ctx context.Context, name string, handler generic.Handler) {
	c.controllerManager.AddHandler(ctx, c.gvk, c.informer.Informer(), name, handler)
}

func (c *{{.lowerName}}Controller) AddGenericRemoveHandler(ctx context.Context, name string, handler generic.Handler) {
	removeHandler := generic.NewRemoveHandler(name, c.Updater(), handler)
	c.controllerManager.AddHandler(ctx, c.gvk, c.informer.Informer(), name, removeHandler)
}

func (c *{{.lowerName}}Controller) OnChange(ctx context.Context, name string, sync {{.type}}Handler) {
	c.AddGenericHandler(ctx, name, From{{.type}}HandlerToHandler(sync))
}

func (c *{{.lowerName}}Controller) OnRemove(ctx context.Context, name string, sync {{.type}}Handler) {
	removeHandler := generic.NewRemoveHandler(name, c.Updater(), From{{.type}}HandlerToHandler(sync))
	c.AddGenericHandler(ctx, name, removeHandler)
}

func (c *{{.lowerName}}Controller) Enqueue({{ if .namespaced}}namespace, {{end}}name string) {
	c.controllerManager.Enqueue(c.gvk, c.informer.Informer(), {{ if .namespaced }}namespace, {{else}}"", {{end}}name)
}

func (c *{{.lowerName}}Controller) EnqueueAfter({{ if .namespaced}}namespace, {{end}}name string, duration time.Duration) {
	c.controllerManager.EnqueueAfter(c.gvk, c.informer.Informer(), {{ if .namespaced }}namespace, {{else}}"", {{end}}name, duration)
}

func (c *{{.lowerName}}Controller) Informer() cache.SharedIndexInformer {
	return c.informer.Informer()
}

func (c *{{.lowerName}}Controller) GroupVersionKind() schema.GroupVersionKind {
	return c.gvk
}

func (c *{{.lowerName}}Controller) Cache() {{.type}}Cache {
	return &{{.lowerName}}Cache{
		lister:  c.informer.Lister(),
		indexer: c.informer.Informer().GetIndexer(),
	}
}

func (c *{{.lowerName}}Controller) Create(obj *{{.version}}.{{.type}}) (*{{.version}}.{{.type}}, error) {
	return c.clientGetter.{{.plural}}({{ if .namespaced}}obj.Namespace{{end}}).Create(context.TODO(), obj, metav1.CreateOptions{})
}

func (c *{{.lowerName}}Controller) Update(obj *{{.version}}.{{.type}}) (*{{.version}}.{{.type}}, error) {
	return c.clientGetter.{{.plural}}({{ if .namespaced}}obj.Namespace{{end}}).Update(context.TODO(), obj, metav1.UpdateOptions{})
}

{{ if .hasStatus -}}
func (c *{{.lowerName}}Controller) UpdateStatus(obj *{{.version}}.{{.type}}) (*{{.version}}.{{.type}}, error) {
	return c.clientGetter.{{.plural}}({{ if .namespaced}}obj.Namespace{{end}}).UpdateStatus(context.TODO(), obj, metav1.UpdateOptions{})
}
{{- end }}

func (c *{{.lowerName}}Controller) Delete({{ if .namespaced}}namespace, {{end}}name string, options *metav1.DeleteOptions) error {
	if options == nil {
		options = &metav1.DeleteOptions{}
	}
	return c.clientGetter.{{.plural}}({{ if .namespaced}}namespace{{end}}).Delete(context.TODO(), name, *options)
}

func (c *{{.lowerName}}Controller) Get({{ if .namespaced}}namespace, {{end}}name string, options metav1.GetOptions) (*{{.version}}.{{.type}}, error) {
	return c.clientGetter.{{.plural}}({{ if .namespaced}}namespace{{end}}).Get(context.TODO(), name, options)
}

func (c *{{.lowerName}}Controller) List({{ if .namespaced}}namespace string, {{end}}opts metav1.ListOptions) (*{{.version}}.{{.type}}List, error) {
	return c.clientGetter.{{.plural}}({{ if .namespaced}}namespace{{end}}).List(context.TODO(), opts)
}

func (c *{{.lowerName}}Controller) Watch({{ if .namespaced}}namespace string, {{end}}opts metav1.ListOptions) (watch.Interface, error) {
	return c.clientGetter.{{.plural}}({{ if .namespaced}}namespace{{end}}).Watch(context.TODO(), opts)
}

func (c *{{.lowerName}}Controller) Patch({{ if .namespaced}}namespace, {{end}}name string, pt types.PatchType, data []byte, subresources ...string) (result *{{.version}}.{{.type}}, err error) {
	return c.clientGetter.{{.plural}}({{ if .namespaced}}namespace{{end}}).Patch(context.TODO(), name, pt, data, metav1.PatchOptions{}, subresources...)
}

type {{.lowerName}}Cache struct {
	lister  listers.{{.type}}Lister
	indexer cache.Indexer
}

func (c *{{.lowerName}}Cache) Get({{ if .namespaced}}namespace, {{end}}name string) (*{{.version}}.{{.type}}, error) {
	return c.lister.{{ if .namespaced}}{{.plural}}(namespace).{{end}}Get(name)
}

func (c *{{.lowerName}}Cache) List({{ if .namespaced}}namespace string, {{end}}selector labels.Selector) ([]*{{.version}}.{{.type}}, error) {
	return c.lister.{{ if .namespaced}}{{.plural}}(namespace).{{end}}List(selector)
}

func (c *{{.lowerName}}Cache) AddIndexer(indexName string, indexer {{.type}}Indexer) {
	utilruntime.Must(c.indexer.AddIndexers(map[string]cache.IndexFunc{
		indexName: func(obj interface{}) (strings []string, e error) {
			return indexer(obj.(*{{.version}}.{{.type}}))
		},
	}))
}

func (c *{{.lowerName}}Cache) GetByIndex(indexName, key string) (result []*{{.version}}.{{.type}}, err error) {
	objs, err := c.indexer.ByIndex(indexName, key)
	if err != nil {
		return nil, err
	}
	result = make([]*{{.version}}.{{.type}}, 0, len(objs))
	for _, obj := range objs {
		result = append(result, obj.(*{{.version}}.{{.type}}))
	}
	return result, nil
}

{{ if .hasStatus -}}
type {{.type}}StatusHandler func(obj *{{.version}}.{{.type}}, status {{.version}}.{{.statusType}}) ({{.version}}.{{.statusType}}, error)

type {{.type}}GeneratingHandler func(obj *{{.version}}.{{.type}}, status {{.version}}.{{.statusType}}) ([]runtime.Object, {{.version}}.{{.statusType}}, error)

func Register{{.type}}StatusHandler(ctx context.Context, controller {{.type}}Controller, condition condition.Cond, name string, handler {{.type}}StatusHandler) {
	statusHandler := &{{.lowerName}}StatusHandler{
		client:    controller,
		condition: condition,
		handler:   handler,
	}
	controller.AddGenericHandler(ctx, name, From{{.type}}HandlerToHandler(statusHandler.sync))
}

func Register{{.type}}GeneratingHandler(ctx context.Context, controller {{.type}}Controller, apply apply.Apply,
	condition condition.Cond, name string, handler {{.type}}GeneratingHandler, opts *generic.GeneratingHandlerOptions) {
	statusHandler := &{{.lowerName}}GeneratingHandler{
		{{.type}}GeneratingHandler: handler,
		apply:                            apply,
		name:                             name,
		gvk:                              controller.GroupVersionKind(),
	}
	if opts != nil {
		statusHandler.opts = *opts
	}
	controller.OnChange(ctx, name, statusHandler.Remove)
	Register{{.type}}StatusHandler(ctx, controller, condition, name, statusHandler.Handle)
}

type {{.lowerName}}StatusHandler struct {
	client    {{.type}}Client
	condition condition.Cond
	handler   {{.type}}StatusHandler
}

func (a *{{.lowerName}}StatusHandler) sync(key string, obj *{{.version}}.{{.type}}) (*{{.version}}.{{.type}}, error) {
	if obj == nil {
		return obj, nil
	}

	origStatus := obj.Status.DeepCopy()
	obj = obj.DeepCopy()
	newStatus, err := a.handler(obj, obj.Status)
	if err != nil {
		// Revert to old status on error
		newStatus = *origStatus.DeepCopy()
	}

	if a.condition != "" {
		if errors.IsConflict(err) {
			a.condition.SetError(&newStatus, "", nil)
		} else {
			a.condition.SetError(&newStatus, "", err)
		}
	}
	if !equality.Semantic.DeepEqual(origStatus, &newStatus) {
		var newErr error
		obj.Status = newStatus
		obj, newErr = a.client.UpdateStatus(obj)
		if err == nil {
			err = newErr
		}
	}
	return obj, err
}

type {{.lowerName}}GeneratingHandler struct {
	{{.type}}GeneratingHandler
	apply apply.Apply
	opts  generic.GeneratingHandlerOptions
	gvk   schema.GroupVersionKind
	name  string
}

func (a *{{.lowerName}}GeneratingHandler) Remove(key string, obj *{{.version}}.{{.type}}) (*{{.version}}.{{.type}}, error) {
	if obj != nil {
		return obj, nil
	}

	obj = &{{.version}}.{{.type}}{}
	obj.Namespace, obj.Name = kv.RSplit(key, "/")
	obj.SetGroupVersionKind(a.gvk)

	return nil, generic.ConfigureApplyForObject(a.apply, obj, &a.opts).
		WithOwner(obj).
		WithSetID(a.name).
		ApplyObjects()
}

func (a *{{.lowerName}}GeneratingHandler) Handle(obj *{{.version}}.{{.type}}, status {{.version}}.{{.statusType}}) ({{.version}}.{{.statusType}}, error) {
	objs, newStatus, err := a.{{.type}}GeneratingHandler(obj, status)
	if err != nil {
		return newStatus, err
	}

	return newStatus, generic.ConfigureApplyForObject(a.apply, obj, &a.opts).
		WithOwner(obj).
		WithSetID(a.name).
		ApplyObjects(objs...)
}
{{- end }}
`
