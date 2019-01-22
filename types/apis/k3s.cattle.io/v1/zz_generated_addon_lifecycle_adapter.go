package v1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type AddonLifecycle interface {
	Create(obj *Addon) (runtime.Object, error)
	Remove(obj *Addon) (runtime.Object, error)
	Updated(obj *Addon) (runtime.Object, error)
}

type addonLifecycleAdapter struct {
	lifecycle AddonLifecycle
}

func (w *addonLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *addonLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *addonLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*Addon))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *addonLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*Addon))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *addonLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*Addon))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewAddonLifecycleAdapter(name string, clusterScoped bool, client AddonInterface, l AddonLifecycle) AddonHandlerFunc {
	adapter := &addonLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *Addon) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
