package v1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type PodLifecycle interface {
	Create(obj *v1.Pod) (runtime.Object, error)
	Remove(obj *v1.Pod) (runtime.Object, error)
	Updated(obj *v1.Pod) (runtime.Object, error)
}

type podLifecycleAdapter struct {
	lifecycle PodLifecycle
}

func (w *podLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *podLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *podLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.Pod))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *podLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.Pod))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *podLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.Pod))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewPodLifecycleAdapter(name string, clusterScoped bool, client PodInterface, l PodLifecycle) PodHandlerFunc {
	adapter := &podLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.Pod) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
