package v1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ConfigMapLifecycle interface {
	Create(obj *v1.ConfigMap) (runtime.Object, error)
	Remove(obj *v1.ConfigMap) (runtime.Object, error)
	Updated(obj *v1.ConfigMap) (runtime.Object, error)
}

type configMapLifecycleAdapter struct {
	lifecycle ConfigMapLifecycle
}

func (w *configMapLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *configMapLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *configMapLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.ConfigMap))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *configMapLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.ConfigMap))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *configMapLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.ConfigMap))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewConfigMapLifecycleAdapter(name string, clusterScoped bool, client ConfigMapInterface, l ConfigMapLifecycle) ConfigMapHandlerFunc {
	adapter := &configMapLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.ConfigMap) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
