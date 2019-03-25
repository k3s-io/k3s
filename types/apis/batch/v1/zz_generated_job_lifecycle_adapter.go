package v1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type JobLifecycle interface {
	Create(obj *v1.Job) (runtime.Object, error)
	Remove(obj *v1.Job) (runtime.Object, error)
	Updated(obj *v1.Job) (runtime.Object, error)
}

type jobLifecycleAdapter struct {
	lifecycle JobLifecycle
}

func (w *jobLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *jobLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *jobLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.Job))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *jobLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.Job))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *jobLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.Job))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewJobLifecycleAdapter(name string, clusterScoped bool, client JobInterface, l JobLifecycle) JobHandlerFunc {
	adapter := &jobLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.Job) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
