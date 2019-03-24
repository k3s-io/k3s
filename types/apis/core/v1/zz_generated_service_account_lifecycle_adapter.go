package v1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ServiceAccountLifecycle interface {
	Create(obj *v1.ServiceAccount) (runtime.Object, error)
	Remove(obj *v1.ServiceAccount) (runtime.Object, error)
	Updated(obj *v1.ServiceAccount) (runtime.Object, error)
}

type serviceAccountLifecycleAdapter struct {
	lifecycle ServiceAccountLifecycle
}

func (w *serviceAccountLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *serviceAccountLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *serviceAccountLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.ServiceAccount))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *serviceAccountLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.ServiceAccount))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *serviceAccountLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.ServiceAccount))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewServiceAccountLifecycleAdapter(name string, clusterScoped bool, client ServiceAccountInterface, l ServiceAccountLifecycle) ServiceAccountHandlerFunc {
	adapter := &serviceAccountLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.ServiceAccount) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
