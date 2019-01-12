package generator

var lifecycleTemplate = `package {{.schema.Version.Version}}

import (
	{{.importPackage}}
	"k8s.io/apimachinery/pkg/runtime"
	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/lifecycle"
)

type {{.schema.CodeName}}Lifecycle interface {
	Create(obj *{{.prefix}}{{.schema.CodeName}}) (runtime.Object, error)
	Remove(obj *{{.prefix}}{{.schema.CodeName}}) (runtime.Object, error)
	Updated(obj *{{.prefix}}{{.schema.CodeName}}) (runtime.Object, error)
}

type {{.schema.ID}}LifecycleAdapter struct {
	lifecycle {{.schema.CodeName}}Lifecycle
}

func (w *{{.schema.ID}}LifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *{{.schema.ID}}LifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *{{.schema.ID}}LifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*{{.prefix}}{{.schema.CodeName}}))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *{{.schema.ID}}LifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*{{.prefix}}{{.schema.CodeName}}))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *{{.schema.ID}}LifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*{{.prefix}}{{.schema.CodeName}}))
	if o == nil {
		return nil, err
	}
	return o, err
}

func New{{.schema.CodeName}}LifecycleAdapter(name string, clusterScoped bool, client {{.schema.CodeName}}Interface, l {{.schema.CodeName}}Lifecycle) {{.schema.CodeName}}HandlerFunc {
	adapter := &{{.schema.ID}}LifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *{{.prefix}}{{.schema.CodeName}}) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
`
