package v1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type ListenerConfigLifecycle interface {
	Create(obj *ListenerConfig) (runtime.Object, error)
	Remove(obj *ListenerConfig) (runtime.Object, error)
	Updated(obj *ListenerConfig) (runtime.Object, error)
}

type listenerConfigLifecycleAdapter struct {
	lifecycle ListenerConfigLifecycle
}

func (w *listenerConfigLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *listenerConfigLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *listenerConfigLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*ListenerConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *listenerConfigLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ListenerConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *listenerConfigLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ListenerConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewListenerConfigLifecycleAdapter(name string, clusterScoped bool, client ListenerConfigInterface, l ListenerConfigLifecycle) ListenerConfigHandlerFunc {
	adapter := &listenerConfigLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ListenerConfig) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
