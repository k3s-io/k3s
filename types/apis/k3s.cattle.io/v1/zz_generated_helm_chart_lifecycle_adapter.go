package v1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type HelmChartLifecycle interface {
	Create(obj *HelmChart) (runtime.Object, error)
	Remove(obj *HelmChart) (runtime.Object, error)
	Updated(obj *HelmChart) (runtime.Object, error)
}

type helmChartLifecycleAdapter struct {
	lifecycle HelmChartLifecycle
}

func (w *helmChartLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *helmChartLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *helmChartLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*HelmChart))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *helmChartLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*HelmChart))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *helmChartLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*HelmChart))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewHelmChartLifecycleAdapter(name string, clusterScoped bool, client HelmChartInterface, l HelmChartLifecycle) HelmChartHandlerFunc {
	adapter := &helmChartLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *HelmChart) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
