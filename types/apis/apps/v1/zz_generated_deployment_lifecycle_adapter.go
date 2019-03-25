package v1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type DeploymentLifecycle interface {
	Create(obj *v1.Deployment) (runtime.Object, error)
	Remove(obj *v1.Deployment) (runtime.Object, error)
	Updated(obj *v1.Deployment) (runtime.Object, error)
}

type deploymentLifecycleAdapter struct {
	lifecycle DeploymentLifecycle
}

func (w *deploymentLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *deploymentLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *deploymentLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.Deployment))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *deploymentLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.Deployment))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *deploymentLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.Deployment))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewDeploymentLifecycleAdapter(name string, clusterScoped bool, client DeploymentInterface, l DeploymentLifecycle) DeploymentHandlerFunc {
	adapter := &deploymentLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.Deployment) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
