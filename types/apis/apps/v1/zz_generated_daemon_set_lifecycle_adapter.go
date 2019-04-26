package v1

import (
	"github.com/rancher/norman/lifecycle"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type DaemonSetLifecycle interface {
	Create(obj *v1.DaemonSet) (runtime.Object, error)
	Remove(obj *v1.DaemonSet) (runtime.Object, error)
	Updated(obj *v1.DaemonSet) (runtime.Object, error)
}

type daemonSetLifecycleAdapter struct {
	lifecycle DaemonSetLifecycle
}

func (w *daemonSetLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *daemonSetLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *daemonSetLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.DaemonSet))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *daemonSetLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.DaemonSet))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *daemonSetLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.DaemonSet))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewDaemonSetLifecycleAdapter(name string, clusterScoped bool, client DaemonSetInterface, l DaemonSetLifecycle) DaemonSetHandlerFunc {
	adapter := &daemonSetLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.DaemonSet) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
