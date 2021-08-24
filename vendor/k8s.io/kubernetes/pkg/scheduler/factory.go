/*
Copyright 2014 The Kubernetes Authors.

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

package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	schedulerapi "k8s.io/kubernetes/pkg/scheduler/apis/config"
	"k8s.io/kubernetes/pkg/scheduler/apis/config/v1beta2"
	"k8s.io/kubernetes/pkg/scheduler/apis/config/validation"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	frameworkplugins "k8s.io/kubernetes/pkg/scheduler/framework/plugins"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/defaultbinder"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/defaultpreemption"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/noderesources"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/queuesort"
	frameworkruntime "k8s.io/kubernetes/pkg/scheduler/framework/runtime"
	internalcache "k8s.io/kubernetes/pkg/scheduler/internal/cache"
	cachedebugger "k8s.io/kubernetes/pkg/scheduler/internal/cache/debugger"
	internalqueue "k8s.io/kubernetes/pkg/scheduler/internal/queue"
	"k8s.io/kubernetes/pkg/scheduler/profile"
)

// Binder knows how to write a binding.
type Binder interface {
	Bind(binding *v1.Binding) error
}

// Configurator defines I/O, caching, and other functionality needed to
// construct a new scheduler.
type Configurator struct {
	client     clientset.Interface
	kubeConfig *restclient.Config

	recorderFactory profile.RecorderFactory

	informerFactory informers.SharedInformerFactory

	// Close this to stop all reflectors
	StopEverything <-chan struct{}

	schedulerCache internalcache.Cache

	componentConfigVersion string

	// Always check all predicates even if the middle of one predicate fails.
	alwaysCheckAllPredicates bool

	// percentageOfNodesToScore specifies percentage of all nodes to score in each scheduling cycle.
	percentageOfNodesToScore int32

	podInitialBackoffSeconds int64

	podMaxBackoffSeconds int64

	profiles          []schedulerapi.KubeSchedulerProfile
	registry          frameworkruntime.Registry
	nodeInfoSnapshot  *internalcache.Snapshot
	extenders         []schedulerapi.Extender
	frameworkCapturer FrameworkCapturer
	parallellism      int32
	// A "cluster event" -> "plugin names" map.
	clusterEventMap map[framework.ClusterEvent]sets.String
}

// create a scheduler from a set of registered plugins.
func (c *Configurator) create() (*Scheduler, error) {
	var extenders []framework.Extender
	var ignoredExtendedResources []string
	if len(c.extenders) != 0 {
		var ignorableExtenders []framework.Extender
		for ii := range c.extenders {
			klog.V(2).InfoS("Creating extender", "extender", c.extenders[ii])
			extender, err := NewHTTPExtender(&c.extenders[ii])
			if err != nil {
				return nil, err
			}
			if !extender.IsIgnorable() {
				extenders = append(extenders, extender)
			} else {
				ignorableExtenders = append(ignorableExtenders, extender)
			}
			for _, r := range c.extenders[ii].ManagedResources {
				if r.IgnoredByScheduler {
					ignoredExtendedResources = append(ignoredExtendedResources, r.Name)
				}
			}
		}
		// place ignorable extenders to the tail of extenders
		extenders = append(extenders, ignorableExtenders...)
	}

	// If there are any extended resources found from the Extenders, append them to the pluginConfig for each profile.
	// This should only have an effect on ComponentConfig, where it is possible to configure Extenders and
	// plugin args (and in which case the extender ignored resources take precedence).
	// For earlier versions, using both policy and custom plugin config is disallowed, so this should be the only
	// plugin config for this plugin.
	if len(ignoredExtendedResources) > 0 {
		for i := range c.profiles {
			prof := &c.profiles[i]
			var found = false
			for k := range prof.PluginConfig {
				if prof.PluginConfig[k].Name == noderesources.FitName {
					// Update the existing args
					pc := &prof.PluginConfig[k]
					args, ok := pc.Args.(*schedulerapi.NodeResourcesFitArgs)
					if !ok {
						return nil, fmt.Errorf("want args to be of type NodeResourcesFitArgs, got %T", pc.Args)
					}
					args.IgnoredResources = ignoredExtendedResources
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("can't find NodeResourcesFitArgs in plugin config")
			}
		}
	}

	// The nominator will be passed all the way to framework instantiation.
	nominator := internalqueue.NewPodNominator(c.informerFactory.Core().V1().Pods().Lister())
	profiles, err := profile.NewMap(c.profiles, c.registry, c.recorderFactory,
		frameworkruntime.WithComponentConfigVersion(c.componentConfigVersion),
		frameworkruntime.WithClientSet(c.client),
		frameworkruntime.WithKubeConfig(c.kubeConfig),
		frameworkruntime.WithInformerFactory(c.informerFactory),
		frameworkruntime.WithSnapshotSharedLister(c.nodeInfoSnapshot),
		frameworkruntime.WithRunAllFilters(c.alwaysCheckAllPredicates),
		frameworkruntime.WithPodNominator(nominator),
		frameworkruntime.WithCaptureProfile(frameworkruntime.CaptureProfile(c.frameworkCapturer)),
		frameworkruntime.WithClusterEventMap(c.clusterEventMap),
		frameworkruntime.WithParallelism(int(c.parallellism)),
		frameworkruntime.WithExtenders(extenders),
	)
	if err != nil {
		return nil, fmt.Errorf("initializing profiles: %v", err)
	}
	if len(profiles) == 0 {
		return nil, errors.New("at least one profile is required")
	}
	// Profiles are required to have equivalent queue sort plugins.
	lessFn := profiles[c.profiles[0].SchedulerName].QueueSortFunc()
	podQueue := internalqueue.NewSchedulingQueue(
		lessFn,
		c.informerFactory,
		internalqueue.WithPodInitialBackoffDuration(time.Duration(c.podInitialBackoffSeconds)*time.Second),
		internalqueue.WithPodMaxBackoffDuration(time.Duration(c.podMaxBackoffSeconds)*time.Second),
		internalqueue.WithPodNominator(nominator),
		internalqueue.WithClusterEventMap(c.clusterEventMap),
	)

	// Setup cache debugger.
	debugger := cachedebugger.New(
		c.informerFactory.Core().V1().Nodes().Lister(),
		c.informerFactory.Core().V1().Pods().Lister(),
		c.schedulerCache,
		podQueue,
	)
	debugger.ListenForSignal(c.StopEverything)

	algo := NewGenericScheduler(
		c.schedulerCache,
		c.nodeInfoSnapshot,
		c.percentageOfNodesToScore,
	)

	return &Scheduler{
		SchedulerCache:  c.schedulerCache,
		Algorithm:       algo,
		Extenders:       extenders,
		Profiles:        profiles,
		NextPod:         internalqueue.MakeNextPodFunc(podQueue),
		Error:           MakeDefaultErrorFunc(c.client, c.informerFactory.Core().V1().Pods().Lister(), podQueue, c.schedulerCache),
		StopEverything:  c.StopEverything,
		SchedulingQueue: podQueue,
	}, nil
}

// createFromPolicy creates a scheduler from the legacy policy file.
func (c *Configurator) createFromPolicy(policy schedulerapi.Policy) (*Scheduler, error) {
	lr := frameworkplugins.NewLegacyRegistry()
	args := &frameworkplugins.ConfigProducerArgs{}

	klog.V(2).InfoS("Creating scheduler from configuration", "policy", policy)

	// validate the policy configuration
	if err := validation.ValidatePolicy(policy); err != nil {
		return nil, err
	}

	// If profiles is already set, it means the user is using both CC and policy config, error out
	// since these configs are no longer merged and they should not be used simultaneously.
	if c.profiles != nil {
		return nil, fmt.Errorf("profiles and policy config both set, this should not happen")
	}

	predicateKeys := sets.NewString()
	if policy.Predicates == nil {
		predicateKeys = lr.DefaultPredicates
	} else {
		for _, predicate := range policy.Predicates {
			klog.V(2).InfoS("Registering predicate", "predicate", predicate.Name)
			predicateName, err := lr.ProcessPredicatePolicy(predicate, args)
			if err != nil {
				return nil, err
			}
			predicateKeys.Insert(predicateName)
		}
	}

	priorityKeys := make(map[string]int64)
	if policy.Priorities == nil {
		klog.V(2).InfoS("Using default priorities")
		priorityKeys = lr.DefaultPriorities
	} else {
		for _, priority := range policy.Priorities {
			if priority.Name == frameworkplugins.EqualPriority {
				klog.V(2).InfoS("Skip registering priority", "priority", priority.Name)
				continue
			}
			klog.V(2).InfoS("Registering priority", "priority", priority.Name)
			priorityName, err := lr.ProcessPriorityPolicy(priority, args)
			if err != nil {
				return nil, err
			}
			priorityKeys[priorityName] = priority.Weight
		}
	}

	// HardPodAffinitySymmetricWeight in the policy config takes precedence over
	// CLI configuration.
	if policy.HardPodAffinitySymmetricWeight != 0 {
		args.InterPodAffinityArgs = &schedulerapi.InterPodAffinityArgs{
			HardPodAffinityWeight: policy.HardPodAffinitySymmetricWeight,
		}
	}

	// When AlwaysCheckAllPredicates is set to true, scheduler checks all the configured
	// predicates even after one or more of them fails.
	if policy.AlwaysCheckAllPredicates {
		c.alwaysCheckAllPredicates = policy.AlwaysCheckAllPredicates
	}

	klog.V(2).InfoS("Creating scheduler", "predicates", predicateKeys, "priorities", priorityKeys)

	// Combine all framework configurations. If this results in any duplication, framework
	// instantiation should fail.

	// "PrioritySort", "DefaultPreemption" and "DefaultBinder" were neither predicates nor priorities
	// before. We add them by default.
	plugins := schedulerapi.Plugins{
		QueueSort: schedulerapi.PluginSet{
			Enabled: []schedulerapi.Plugin{{Name: queuesort.Name}},
		},
		PostFilter: schedulerapi.PluginSet{
			Enabled: []schedulerapi.Plugin{{Name: defaultpreemption.Name}},
		},
		Bind: schedulerapi.PluginSet{
			Enabled: []schedulerapi.Plugin{{Name: defaultbinder.Name}},
		},
	}
	var pluginConfig []schedulerapi.PluginConfig
	var err error
	if plugins, pluginConfig, err = lr.AppendPredicateConfigs(predicateKeys, args, plugins, pluginConfig); err != nil {
		return nil, err
	}
	if plugins, pluginConfig, err = lr.AppendPriorityConfigs(priorityKeys, args, plugins, pluginConfig); err != nil {
		return nil, err
	}
	if pluginConfig, err = dedupPluginConfigs(pluginConfig); err != nil {
		return nil, err
	}

	c.profiles = []schedulerapi.KubeSchedulerProfile{
		{
			SchedulerName: v1.DefaultSchedulerName,
			Plugins:       &plugins,
			PluginConfig:  pluginConfig,
		},
	}

	if err := defaultPluginConfigArgs(&c.profiles[0]); err != nil {
		return nil, err
	}

	return c.create()
}

func defaultPluginConfigArgs(prof *schedulerapi.KubeSchedulerProfile) error {
	scheme := v1beta2.GetPluginArgConversionScheme()
	existingConfigs := sets.NewString()
	for j := range prof.PluginConfig {
		existingConfigs.Insert(prof.PluginConfig[j].Name)
		// For existing plugin configs, we don't apply any defaulting, the assumption
		// is that the legacy registry does it already.
	}

	// Append default configs for plugins that didn't have one explicitly set.
	for _, name := range prof.Plugins.Names() {
		if existingConfigs.Has(name) {
			continue
		}
		gvk := v1beta2.SchemeGroupVersion.WithKind(name + "Args")
		args, err := scheme.New(gvk)
		if err != nil {
			if runtime.IsNotRegisteredError(err) {
				// This plugin is out-of-tree or doesn't require configuration.
				continue
			}
			return err
		}
		scheme.Default(args)
		internalArgs, err := scheme.ConvertToVersion(args, schedulerapi.SchemeGroupVersion)
		if err != nil {
			return fmt.Errorf("converting %q into internal type: %w", gvk.Kind, err)
		}
		prof.PluginConfig = append(prof.PluginConfig, schedulerapi.PluginConfig{
			Name: name,
			Args: internalArgs,
		})
	}

	return nil
}

// dedupPluginConfigs removes duplicates from pluginConfig, ensuring that,
// if a plugin name is repeated, the arguments are the same.
func dedupPluginConfigs(pc []schedulerapi.PluginConfig) ([]schedulerapi.PluginConfig, error) {
	args := make(map[string]runtime.Object)
	result := make([]schedulerapi.PluginConfig, 0, len(pc))
	for _, c := range pc {
		if v, found := args[c.Name]; !found {
			result = append(result, c)
			args[c.Name] = c.Args
		} else if !cmp.Equal(v, c.Args) {
			// This should be unreachable.
			return nil, fmt.Errorf("inconsistent configuration produced for plugin %s", c.Name)
		}
	}
	return result, nil
}

// MakeDefaultErrorFunc construct a function to handle pod scheduler error
func MakeDefaultErrorFunc(client clientset.Interface, podLister corelisters.PodLister, podQueue internalqueue.SchedulingQueue, schedulerCache internalcache.Cache) func(*framework.QueuedPodInfo, error) {
	return func(podInfo *framework.QueuedPodInfo, err error) {
		pod := podInfo.Pod
		if err == ErrNoNodesAvailable {
			klog.V(2).InfoS("Unable to schedule pod; no nodes are registered to the cluster; waiting", "pod", klog.KObj(pod))
		} else if fitError, ok := err.(*framework.FitError); ok {
			// Inject UnschedulablePlugins to PodInfo, which will be used later for moving Pods between queues efficiently.
			podInfo.UnschedulablePlugins = fitError.Diagnosis.UnschedulablePlugins
			klog.V(2).InfoS("Unable to schedule pod; no fit; waiting", "pod", klog.KObj(pod), "err", err)
		} else if apierrors.IsNotFound(err) {
			klog.V(2).InfoS("Unable to schedule pod, possibly due to node not found; waiting", "pod", klog.KObj(pod), "err", err)
			if errStatus, ok := err.(apierrors.APIStatus); ok && errStatus.Status().Details.Kind == "node" {
				nodeName := errStatus.Status().Details.Name
				// when node is not found, We do not remove the node right away. Trying again to get
				// the node and if the node is still not found, then remove it from the scheduler cache.
				_, err := client.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
				if err != nil && apierrors.IsNotFound(err) {
					node := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}}
					if err := schedulerCache.RemoveNode(&node); err != nil {
						klog.V(4).InfoS("Node is not found; failed to remove it from the cache", "node", node.Name)
					}
				}
			}
		} else {
			klog.ErrorS(err, "Error scheduling pod; retrying", "pod", klog.KObj(pod))
		}

		// Check if the Pod exists in informer cache.
		cachedPod, err := podLister.Pods(pod.Namespace).Get(pod.Name)
		if err != nil {
			klog.InfoS("Pod doesn't exist in informer cache", "pod", klog.KObj(pod), "err", err)
			return
		}

		// In the case of extender, the pod may have been bound successfully, but timed out returning its response to the scheduler.
		// It could result in the live version to carry .spec.nodeName, and that's inconsistent with the internal-queued version.
		if len(cachedPod.Spec.NodeName) != 0 {
			klog.InfoS("Pod has been assigned to node. Abort adding it back to queue.", "pod", klog.KObj(pod), "node", cachedPod.Spec.NodeName)
			return
		}

		// As <cachedPod> is from SharedInformer, we need to do a DeepCopy() here.
		podInfo.PodInfo = framework.NewPodInfo(cachedPod.DeepCopy())
		if err := podQueue.AddUnschedulableIfNotPresent(podInfo, podQueue.SchedulingCycle()); err != nil {
			klog.ErrorS(err, "Error occurred")
		}
	}
}
