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
		"k8s.io/apimachinery/pkg/labels",
		"k8s.io/apimachinery/pkg/runtime",
		"k8s.io/apimachinery/pkg/runtime/schema",
		"k8s.io/apimachinery/pkg/types",
		"utilruntime \"k8s.io/apimachinery/pkg/util/runtime\"",
		"k8s.io/apimachinery/pkg/watch",
		"k8s.io/client-go/tools/cache",
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
	}

	sw.Do(string(typeBody), m)
	return sw.Error()
}

func hasStatus(t *types.Type) bool {
	for _, m := range t.Members {
		if m.Name == "Status" {
			return true
		}
	}
	return false
}

var typeBody = `
type {{.type}}Handler func(string, *{{.version}}.{{.type}}) (*{{.version}}.{{.type}}, error)

type {{.type}}Controller interface {
	{{.type}}Client

	OnChange(ctx context.Context, name string, sync {{.type}}Handler)
	OnRemove(ctx context.Context, name string, sync {{.type}}Handler)
	Enqueue({{ if .namespaced}}namespace, {{end}}name string)

	Cache() {{.type}}Cache

	Informer() cache.SharedIndexInformer
	GroupVersionKind() schema.GroupVersionKind

	AddGenericHandler(ctx context.Context, name string, handler generic.Handler)
	AddGenericRemoveHandler(ctx context.Context, name string, handler generic.Handler)
	Updater() generic.Updater
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

func Update{{.type}}OnChange(updater generic.Updater, handler {{.type}}Handler) {{.type}}Handler {
	return func(key string, obj *{{.version}}.{{.type}}) (*{{.version}}.{{.type}}, error) {
		if obj == nil {
			return handler(key, nil)
		}

		copyObj := obj.DeepCopy()
		newObj, err := handler(key, copyObj)
		if newObj != nil {
			copyObj = newObj
		}
		if obj.ResourceVersion == copyObj.ResourceVersion && !equality.Semantic.DeepEqual(obj, copyObj) {
			newObj, err := updater(copyObj)
			if newObj != nil && err == nil {
				copyObj = newObj.(*{{.version}}.{{.type}})
			}
		}

		return copyObj, err
	}
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
	return c.clientGetter.{{.plural}}({{ if .namespaced}}obj.Namespace{{end}}).Create(obj)
}

func (c *{{.lowerName}}Controller) Update(obj *{{.version}}.{{.type}}) (*{{.version}}.{{.type}}, error) {
	return c.clientGetter.{{.plural}}({{ if .namespaced}}obj.Namespace{{end}}).Update(obj)
}

{{ if .hasStatus -}}
func (c *{{.lowerName}}Controller) UpdateStatus(obj *{{.version}}.{{.type}}) (*{{.version}}.{{.type}}, error) {
	return c.clientGetter.{{.plural}}({{ if .namespaced}}obj.Namespace{{end}}).UpdateStatus(obj)
}
{{- end }}

func (c *{{.lowerName}}Controller) Delete({{ if .namespaced}}namespace, {{end}}name string, options *metav1.DeleteOptions) error {
	return c.clientGetter.{{.plural}}({{ if .namespaced}}namespace{{end}}).Delete(name, options)
}

func (c *{{.lowerName}}Controller) Get({{ if .namespaced}}namespace, {{end}}name string, options metav1.GetOptions) (*{{.version}}.{{.type}}, error) {
	return c.clientGetter.{{.plural}}({{ if .namespaced}}namespace{{end}}).Get(name, options)
}

func (c *{{.lowerName}}Controller) List({{ if .namespaced}}namespace string, {{end}}opts metav1.ListOptions) (*{{.version}}.{{.type}}List, error) {
	return c.clientGetter.{{.plural}}({{ if .namespaced}}namespace{{end}}).List(opts)
}

func (c *{{.lowerName}}Controller) Watch({{ if .namespaced}}namespace string, {{end}}opts metav1.ListOptions) (watch.Interface, error) {
	return c.clientGetter.{{.plural}}({{ if .namespaced}}namespace{{end}}).Watch(opts)
}

func (c *{{.lowerName}}Controller) Patch({{ if .namespaced}}namespace, {{end}}name string, pt types.PatchType, data []byte, subresources ...string) (result *{{.version}}.{{.type}}, err error) {
	return c.clientGetter.{{.plural}}({{ if .namespaced}}namespace{{end}}).Patch(name, pt, data, subresources...)
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
	for _, obj := range objs {
		result = append(result, obj.(*{{.version}}.{{.type}}))
	}
	return result, nil
}
`
