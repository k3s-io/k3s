package v1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type NodeLifecycle interface {
	Create(obj *v1.Node) (runtime.Object, error)
	Remove(obj *v1.Node) (runtime.Object, error)
	Updated(obj *v1.Node) (runtime.Object, error)
}

type nodeLifecycleAdapter struct {
	lifecycle NodeLifecycle
}

func (w *nodeLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *nodeLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *nodeLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.Node))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *nodeLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.Node))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *nodeLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.Node))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewNodeLifecycleAdapter(name string, clusterScoped bool, client NodeInterface, l NodeLifecycle) NodeHandlerFunc {
	adapter := &nodeLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.Node) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
