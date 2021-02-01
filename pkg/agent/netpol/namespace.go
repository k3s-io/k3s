// Apache License v2.0 (copyright Cloud Native Labs & Rancher Labs)
// - modified from https://github.com/cloudnativelabs/kube-router/blob/ee9f6d890d10609284098229fa1e283ab5d83b93/pkg/controllers/netpol/namespace.go

// +build !windows

package netpol

import (
	"reflect"

	api "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	glog "k8s.io/klog"
)

func (npc *NetworkPolicyController) newNamespaceEventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			npc.handleNamespaceAdd(obj.(*api.Namespace))
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			npc.handleNamespaceUpdate(oldObj.(*api.Namespace), newObj.(*api.Namespace))
		},
		DeleteFunc: func(obj interface{}) {
			switch obj := obj.(type) {
			case *api.Namespace:
				npc.handleNamespaceDelete(obj)
				return
			case cache.DeletedFinalStateUnknown:
				if namespace, ok := obj.Obj.(*api.Namespace); ok {
					npc.handleNamespaceDelete(namespace)
					return
				}
			default:
				glog.Errorf("unexpected object type: %v", obj)
			}
		},
	}
}

func (npc *NetworkPolicyController) handleNamespaceAdd(obj *api.Namespace) {
	if obj.Labels == nil {
		return
	}
	glog.V(2).Infof("Received update for namespace: %s", obj.Name)

	npc.RequestFullSync()
}

func (npc *NetworkPolicyController) handleNamespaceUpdate(oldObj, newObj *api.Namespace) {
	if reflect.DeepEqual(oldObj.Labels, newObj.Labels) {
		return
	}
	glog.V(2).Infof("Received update for namespace: %s", newObj.Name)

	npc.RequestFullSync()
}

func (npc *NetworkPolicyController) handleNamespaceDelete(obj *api.Namespace) {
	if obj.Labels == nil {
		return
	}
	glog.V(2).Infof("Received namespace: %s delete event", obj.Name)

	npc.RequestFullSync()
}
