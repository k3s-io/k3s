package v1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type EndpointsLifecycle interface {
	Create(obj *v1.Endpoints) (runtime.Object, error)
	Remove(obj *v1.Endpoints) (runtime.Object, error)
	Updated(obj *v1.Endpoints) (runtime.Object, error)
}

type endpointsLifecycleAdapter struct {
	lifecycle EndpointsLifecycle
}

func (w *endpointsLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *endpointsLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *endpointsLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.Endpoints))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *endpointsLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.Endpoints))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *endpointsLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.Endpoints))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewEndpointsLifecycleAdapter(name string, clusterScoped bool, client EndpointsInterface, l EndpointsLifecycle) EndpointsHandlerFunc {
	adapter := &endpointsLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.Endpoints) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
