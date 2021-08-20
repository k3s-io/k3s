/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package podtopologyspread

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/kubernetes/pkg/scheduler/apis/config"
	"k8s.io/kubernetes/pkg/scheduler/apis/config/validation"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/names"
	"k8s.io/kubernetes/pkg/scheduler/internal/parallelize"
)

const (
	// ErrReasonConstraintsNotMatch is used for PodTopologySpread filter error.
	ErrReasonConstraintsNotMatch = "node(s) didn't match pod topology spread constraints"
	// ErrReasonNodeLabelNotMatch is used when the node doesn't hold the required label.
	ErrReasonNodeLabelNotMatch = ErrReasonConstraintsNotMatch + " (missing required label)"
)

var systemDefaultConstraints = []v1.TopologySpreadConstraint{
	{
		TopologyKey:       v1.LabelHostname,
		WhenUnsatisfiable: v1.ScheduleAnyway,
		MaxSkew:           3,
	},
	{
		TopologyKey:       v1.LabelTopologyZone,
		WhenUnsatisfiable: v1.ScheduleAnyway,
		MaxSkew:           5,
	},
}

// PodTopologySpread is a plugin that ensures pod's topologySpreadConstraints is satisfied.
type PodTopologySpread struct {
	parallelizer       parallelize.Parallelizer
	defaultConstraints []v1.TopologySpreadConstraint
	sharedLister       framework.SharedLister
	services           corelisters.ServiceLister
	replicationCtrls   corelisters.ReplicationControllerLister
	replicaSets        appslisters.ReplicaSetLister
	statefulSets       appslisters.StatefulSetLister
}

var _ framework.PreFilterPlugin = &PodTopologySpread{}
var _ framework.FilterPlugin = &PodTopologySpread{}
var _ framework.PreScorePlugin = &PodTopologySpread{}
var _ framework.ScorePlugin = &PodTopologySpread{}
var _ framework.EnqueueExtensions = &PodTopologySpread{}

const (
	// Name is the name of the plugin used in the plugin registry and configurations.
	Name = names.PodTopologySpread
)

// Name returns name of the plugin. It is used in logs, etc.
func (pl *PodTopologySpread) Name() string {
	return Name
}

// New initializes a new plugin and returns it.
func New(plArgs runtime.Object, h framework.Handle) (framework.Plugin, error) {
	if h.SnapshotSharedLister() == nil {
		return nil, fmt.Errorf("SnapshotSharedlister is nil")
	}
	args, err := getArgs(plArgs)
	if err != nil {
		return nil, err
	}
	if err := validation.ValidatePodTopologySpreadArgs(nil, &args); err != nil {
		return nil, err
	}
	pl := &PodTopologySpread{
		parallelizer:       h.Parallelizer(),
		sharedLister:       h.SnapshotSharedLister(),
		defaultConstraints: args.DefaultConstraints,
	}
	if args.DefaultingType == config.SystemDefaulting {
		pl.defaultConstraints = systemDefaultConstraints
	}
	if len(pl.defaultConstraints) != 0 {
		if h.SharedInformerFactory() == nil {
			return nil, fmt.Errorf("SharedInformerFactory is nil")
		}
		pl.setListers(h.SharedInformerFactory())
	}
	return pl, nil
}

func getArgs(obj runtime.Object) (config.PodTopologySpreadArgs, error) {
	ptr, ok := obj.(*config.PodTopologySpreadArgs)
	if !ok {
		return config.PodTopologySpreadArgs{}, fmt.Errorf("want args to be of type PodTopologySpreadArgs, got %T", obj)
	}
	return *ptr, nil
}

func (pl *PodTopologySpread) setListers(factory informers.SharedInformerFactory) {
	pl.services = factory.Core().V1().Services().Lister()
	pl.replicationCtrls = factory.Core().V1().ReplicationControllers().Lister()
	pl.replicaSets = factory.Apps().V1().ReplicaSets().Lister()
	pl.statefulSets = factory.Apps().V1().StatefulSets().Lister()
}

// EventsToRegister returns the possible events that may make a Pod
// failed by this plugin schedulable.
func (pl *PodTopologySpread) EventsToRegister() []framework.ClusterEvent {
	return []framework.ClusterEvent{
		// All ActionType includes the following events:
		// - Add. An unschedulable Pod may fail due to violating topology spread constraints,
		// adding an assigned Pod may make it schedulable.
		// - Update. Updating on an existing Pod's labels (e.g., removal) may make
		// an unschedulable Pod schedulable.
		// - Delete. An unschedulable Pod may fail due to violating an existing Pod's topology spread constraints,
		// deleting an existing Pod may make it schedulable.
		{Resource: framework.Pod, ActionType: framework.All},
		// Node add|delete|updateLabel maybe lead an topology key changed,
		// and make these pod in scheduling schedulable or unschedulable.
		{Resource: framework.Node, ActionType: framework.Add | framework.Delete | framework.UpdateNodeLabel},
	}
}
