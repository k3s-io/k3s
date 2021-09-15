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

// This file defines the scheduling framework plugin interfaces.

package framework

import (
	"context"
	"errors"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/events"
	"k8s.io/kubernetes/pkg/scheduler/apis/config"
	"k8s.io/kubernetes/pkg/scheduler/internal/parallelize"
)

// NodeScoreList declares a list of nodes and their scores.
type NodeScoreList []NodeScore

// NodeScore is a struct with node name and score.
type NodeScore struct {
	Name  string
	Score int64
}

// PluginToNodeScores declares a map from plugin name to its NodeScoreList.
type PluginToNodeScores map[string]NodeScoreList

// NodeToStatusMap declares map from node name to its status.
type NodeToStatusMap map[string]*Status

// Code is the Status code/type which is returned from plugins.
type Code int

// These are predefined codes used in a Status.
const (
	// Success means that plugin ran correctly and found pod schedulable.
	// NOTE: A nil status is also considered as "Success".
	Success Code = iota
	// Error is used for internal plugin errors, unexpected input, etc.
	Error
	// Unschedulable is used when a plugin finds a pod unschedulable. The scheduler might attempt to
	// preempt other pods to get this pod scheduled. Use UnschedulableAndUnresolvable to make the
	// scheduler skip preemption.
	// The accompanying status message should explain why the pod is unschedulable.
	Unschedulable
	// UnschedulableAndUnresolvable is used when a plugin finds a pod unschedulable and
	// preemption would not change anything. Plugins should return Unschedulable if it is possible
	// that the pod can get scheduled with preemption.
	// The accompanying status message should explain why the pod is unschedulable.
	UnschedulableAndUnresolvable
	// Wait is used when a Permit plugin finds a pod scheduling should wait.
	Wait
	// Skip is used when a Bind plugin chooses to skip binding.
	Skip
)

// This list should be exactly the same as the codes iota defined above in the same order.
var codes = []string{"Success", "Error", "Unschedulable", "UnschedulableAndUnresolvable", "Wait", "Skip"}

// statusPrecedence defines a map from status to its precedence, larger value means higher precedent.
var statusPrecedence = map[Code]int{
	Error:                        3,
	UnschedulableAndUnresolvable: 2,
	Unschedulable:                1,
	// Any other statuses we know today, `Skip` or `Wait`, will take precedence over `Success`.
	Success: -1,
}

func (c Code) String() string {
	return codes[c]
}

const (
	// MaxNodeScore is the maximum score a Score plugin is expected to return.
	MaxNodeScore int64 = 100

	// MinNodeScore is the minimum score a Score plugin is expected to return.
	MinNodeScore int64 = 0

	// MaxTotalScore is the maximum total score.
	MaxTotalScore int64 = math.MaxInt64
)

// PodsToActivateKey is a reserved state key for stashing pods.
// If the stashed pods are present in unschedulableQ or backoffQ，they will be
// activated (i.e., moved to activeQ) in two phases:
// - end of a scheduling cycle if it succeeds (will be cleared from `PodsToActivate` if activated)
// - end of a binding cycle if it succeeds
var PodsToActivateKey StateKey = "kubernetes.io/pods-to-activate"

// PodsToActivate stores pods to be activated.
type PodsToActivate struct {
	sync.Mutex
	// Map is keyed with namespaced pod name, and valued with the pod.
	Map map[string]*v1.Pod
}

// Clone just returns the same state.
func (s *PodsToActivate) Clone() StateData {
	return s
}

// NewPodsToActivate instantiates a PodsToActivate object.
func NewPodsToActivate() *PodsToActivate {
	return &PodsToActivate{Map: make(map[string]*v1.Pod)}
}

// Status indicates the result of running a plugin. It consists of a code, a
// message, (optionally) an error and an plugin name it fails by. When the status
// code is not `Success`, the reasons should explain why.
// NOTE: A nil Status is also considered as Success.
type Status struct {
	code    Code
	reasons []string
	err     error
	// failedPlugin is an optional field that records the plugin name a Pod failed by.
	// It's set by the framework when code is Error, Unschedulable or UnschedulableAndUnresolvable.
	failedPlugin string
}

// Code returns code of the Status.
func (s *Status) Code() Code {
	if s == nil {
		return Success
	}
	return s.code
}

// Message returns a concatenated message on reasons of the Status.
func (s *Status) Message() string {
	if s == nil {
		return ""
	}
	return strings.Join(s.reasons, ", ")
}

// SetFailedPlugin sets the given plugin name to s.failedPlugin.
func (s *Status) SetFailedPlugin(plugin string) {
	s.failedPlugin = plugin
}

// WithFailedPlugin sets the given plugin name to s.failedPlugin,
// and returns the given status object.
func (s *Status) WithFailedPlugin(plugin string) *Status {
	s.SetFailedPlugin(plugin)
	return s
}

// FailedPlugin returns the failed plugin name.
func (s *Status) FailedPlugin() string {
	return s.failedPlugin
}

// Reasons returns reasons of the Status.
func (s *Status) Reasons() []string {
	return s.reasons
}

// AppendReason appends given reason to the Status.
func (s *Status) AppendReason(reason string) {
	s.reasons = append(s.reasons, reason)
}

// IsSuccess returns true if and only if "Status" is nil or Code is "Success".
func (s *Status) IsSuccess() bool {
	return s.Code() == Success
}

// IsUnschedulable returns true if "Status" is Unschedulable (Unschedulable or UnschedulableAndUnresolvable).
func (s *Status) IsUnschedulable() bool {
	code := s.Code()
	return code == Unschedulable || code == UnschedulableAndUnresolvable
}

// AsError returns nil if the status is a success; otherwise returns an "error" object
// with a concatenated message on reasons of the Status.
func (s *Status) AsError() error {
	if s.IsSuccess() {
		return nil
	}
	if s.err != nil {
		return s.err
	}
	return errors.New(s.Message())
}

// Equal checks equality of two statuses. This is useful for testing with
// cmp.Equal.
func (s *Status) Equal(x *Status) bool {
	if s == nil || x == nil {
		return s.IsSuccess() && x.IsSuccess()
	}
	if s.code != x.code {
		return false
	}
	if s.code == Error {
		return cmp.Equal(s.err, x.err, cmpopts.EquateErrors())
	}
	return cmp.Equal(s.reasons, x.reasons)
}

// NewStatus makes a Status out of the given arguments and returns its pointer.
func NewStatus(code Code, reasons ...string) *Status {
	s := &Status{
		code:    code,
		reasons: reasons,
	}
	if code == Error {
		s.err = errors.New(s.Message())
	}
	return s
}

// AsStatus wraps an error in a Status.
func AsStatus(err error) *Status {
	return &Status{
		code:    Error,
		reasons: []string{err.Error()},
		err:     err,
	}
}

// PluginToStatus maps plugin name to status. Currently used to identify which Filter plugin
// returned which status.
type PluginToStatus map[string]*Status

// Merge merges the statuses in the map into one. The resulting status code have the following
// precedence: Error, UnschedulableAndUnresolvable, Unschedulable.
func (p PluginToStatus) Merge() *Status {
	if len(p) == 0 {
		return nil
	}

	finalStatus := NewStatus(Success)
	for _, s := range p {
		if s.Code() == Error {
			finalStatus.err = s.AsError()
		}
		if statusPrecedence[s.Code()] > statusPrecedence[finalStatus.code] {
			finalStatus.code = s.Code()
			// Same as code, we keep the most relevant failedPlugin in the returned Status.
			finalStatus.failedPlugin = s.FailedPlugin()
		}

		for _, r := range s.reasons {
			finalStatus.AppendReason(r)
		}
	}

	return finalStatus
}

// WaitingPod represents a pod currently waiting in the permit phase.
type WaitingPod interface {
	// GetPod returns a reference to the waiting pod.
	GetPod() *v1.Pod
	// GetPendingPlugins returns a list of pending Permit plugin's name.
	GetPendingPlugins() []string
	// Allow declares the waiting pod is allowed to be scheduled by the plugin named as "pluginName".
	// If this is the last remaining plugin to allow, then a success signal is delivered
	// to unblock the pod.
	Allow(pluginName string)
	// Reject declares the waiting pod unschedulable.
	Reject(pluginName, msg string)
}

// Plugin is the parent type for all the scheduling framework plugins.
type Plugin interface {
	Name() string
}

// LessFunc is the function to sort pod info
type LessFunc func(podInfo1, podInfo2 *QueuedPodInfo) bool

// QueueSortPlugin is an interface that must be implemented by "QueueSort" plugins.
// These plugins are used to sort pods in the scheduling queue. Only one queue sort
// plugin may be enabled at a time.
type QueueSortPlugin interface {
	Plugin
	// Less are used to sort pods in the scheduling queue.
	Less(*QueuedPodInfo, *QueuedPodInfo) bool
}

// EnqueueExtensions is an optional interface that plugins can implement to efficiently
// move unschedulable Pods in internal scheduling queues.
type EnqueueExtensions interface {
	// EventsToRegister returns a series of interested events that
	// will be registered when instantiating the internal scheduling queue.
	// Note: the returned list needs to be static (not depend on configuration parameters);
	// otherwise it would lead to undefined behavior.
	EventsToRegister() []ClusterEvent
}

// PreFilterExtensions is an interface that is included in plugins that allow specifying
// callbacks to make incremental updates to its supposedly pre-calculated
// state.
type PreFilterExtensions interface {
	// AddPod is called by the framework while trying to evaluate the impact
	// of adding podToAdd to the node while scheduling podToSchedule.
	AddPod(ctx context.Context, state *CycleState, podToSchedule *v1.Pod, podInfoToAdd *PodInfo, nodeInfo *NodeInfo) *Status
	// RemovePod is called by the framework while trying to evaluate the impact
	// of removing podToRemove from the node while scheduling podToSchedule.
	RemovePod(ctx context.Context, state *CycleState, podToSchedule *v1.Pod, podInfoToRemove *PodInfo, nodeInfo *NodeInfo) *Status
}

// PreFilterPlugin is an interface that must be implemented by "PreFilter" plugins.
// These plugins are called at the beginning of the scheduling cycle.
type PreFilterPlugin interface {
	Plugin
	// PreFilter is called at the beginning of the scheduling cycle. All PreFilter
	// plugins must return success or the pod will be rejected.
	PreFilter(ctx context.Context, state *CycleState, p *v1.Pod) *Status
	// PreFilterExtensions returns a PreFilterExtensions interface if the plugin implements one,
	// or nil if it does not. A Pre-filter plugin can provide extensions to incrementally
	// modify its pre-processed info. The framework guarantees that the extensions
	// AddPod/RemovePod will only be called after PreFilter, possibly on a cloned
	// CycleState, and may call those functions more than once before calling
	// Filter again on a specific node.
	PreFilterExtensions() PreFilterExtensions
}

// FilterPlugin is an interface for Filter plugins. These plugins are called at the
// filter extension point for filtering out hosts that cannot run a pod.
// This concept used to be called 'predicate' in the original scheduler.
// These plugins should return "Success", "Unschedulable" or "Error" in Status.code.
// However, the scheduler accepts other valid codes as well.
// Anything other than "Success" will lead to exclusion of the given host from
// running the pod.
type FilterPlugin interface {
	Plugin
	// Filter is called by the scheduling framework.
	// All FilterPlugins should return "Success" to declare that
	// the given node fits the pod. If Filter doesn't return "Success",
	// it will return "Unschedulable", "UnschedulableAndUnresolvable" or "Error".
	// For the node being evaluated, Filter plugins should look at the passed
	// nodeInfo reference for this particular node's information (e.g., pods
	// considered to be running on the node) instead of looking it up in the
	// NodeInfoSnapshot because we don't guarantee that they will be the same.
	// For example, during preemption, we may pass a copy of the original
	// nodeInfo object that has some pods removed from it to evaluate the
	// possibility of preempting them to schedule the target pod.
	Filter(ctx context.Context, state *CycleState, pod *v1.Pod, nodeInfo *NodeInfo) *Status
}

// PostFilterPlugin is an interface for "PostFilter" plugins. These plugins are called
// after a pod cannot be scheduled.
type PostFilterPlugin interface {
	Plugin
	// PostFilter is called by the scheduling framework.
	// A PostFilter plugin should return one of the following statuses:
	// - Unschedulable: the plugin gets executed successfully but the pod cannot be made schedulable.
	// - Success: the plugin gets executed successfully and the pod can be made schedulable.
	// - Error: the plugin aborts due to some internal error.
	//
	// Informational plugins should be configured ahead of other ones, and always return Unschedulable status.
	// Optionally, a non-nil PostFilterResult may be returned along with a Success status. For example,
	// a preemption plugin may choose to return nominatedNodeName, so that framework can reuse that to update the
	// preemptor pod's .spec.status.nominatedNodeName field.
	PostFilter(ctx context.Context, state *CycleState, pod *v1.Pod, filteredNodeStatusMap NodeToStatusMap) (*PostFilterResult, *Status)
}

// PreScorePlugin is an interface for "PreScore" plugin. PreScore is an
// informational extension point. Plugins will be called with a list of nodes
// that passed the filtering phase. A plugin may use this data to update internal
// state or to generate logs/metrics.
type PreScorePlugin interface {
	Plugin
	// PreScore is called by the scheduling framework after a list of nodes
	// passed the filtering phase. All prescore plugins must return success or
	// the pod will be rejected
	PreScore(ctx context.Context, state *CycleState, pod *v1.Pod, nodes []*v1.Node) *Status
}

// ScoreExtensions is an interface for Score extended functionality.
type ScoreExtensions interface {
	// NormalizeScore is called for all node scores produced by the same plugin's "Score"
	// method. A successful run of NormalizeScore will update the scores list and return
	// a success status.
	NormalizeScore(ctx context.Context, state *CycleState, p *v1.Pod, scores NodeScoreList) *Status
}

// ScorePlugin is an interface that must be implemented by "Score" plugins to rank
// nodes that passed the filtering phase.
type ScorePlugin interface {
	Plugin
	// Score is called on each filtered node. It must return success and an integer
	// indicating the rank of the node. All scoring plugins must return success or
	// the pod will be rejected.
	Score(ctx context.Context, state *CycleState, p *v1.Pod, nodeName string) (int64, *Status)

	// ScoreExtensions returns a ScoreExtensions interface if it implements one, or nil if does not.
	ScoreExtensions() ScoreExtensions
}

// ReservePlugin is an interface for plugins with Reserve and Unreserve
// methods. These are meant to update the state of the plugin. This concept
// used to be called 'assume' in the original scheduler. These plugins should
// return only Success or Error in Status.code. However, the scheduler accepts
// other valid codes as well. Anything other than Success will lead to
// rejection of the pod.
type ReservePlugin interface {
	Plugin
	// Reserve is called by the scheduling framework when the scheduler cache is
	// updated. If this method returns a failed Status, the scheduler will call
	// the Unreserve method for all enabled ReservePlugins.
	Reserve(ctx context.Context, state *CycleState, p *v1.Pod, nodeName string) *Status
	// Unreserve is called by the scheduling framework when a reserved pod was
	// rejected, an error occurred during reservation of subsequent plugins, or
	// in a later phase. The Unreserve method implementation must be idempotent
	// and may be called by the scheduler even if the corresponding Reserve
	// method for the same plugin was not called.
	Unreserve(ctx context.Context, state *CycleState, p *v1.Pod, nodeName string)
}

// PreBindPlugin is an interface that must be implemented by "PreBind" plugins.
// These plugins are called before a pod being scheduled.
type PreBindPlugin interface {
	Plugin
	// PreBind is called before binding a pod. All prebind plugins must return
	// success or the pod will be rejected and won't be sent for binding.
	PreBind(ctx context.Context, state *CycleState, p *v1.Pod, nodeName string) *Status
}

// PostBindPlugin is an interface that must be implemented by "PostBind" plugins.
// These plugins are called after a pod is successfully bound to a node.
type PostBindPlugin interface {
	Plugin
	// PostBind is called after a pod is successfully bound. These plugins are
	// informational. A common application of this extension point is for cleaning
	// up. If a plugin needs to clean-up its state after a pod is scheduled and
	// bound, PostBind is the extension point that it should register.
	PostBind(ctx context.Context, state *CycleState, p *v1.Pod, nodeName string)
}

// PermitPlugin is an interface that must be implemented by "Permit" plugins.
// These plugins are called before a pod is bound to a node.
type PermitPlugin interface {
	Plugin
	// Permit is called before binding a pod (and before prebind plugins). Permit
	// plugins are used to prevent or delay the binding of a Pod. A permit plugin
	// must return success or wait with timeout duration, or the pod will be rejected.
	// The pod will also be rejected if the wait timeout or the pod is rejected while
	// waiting. Note that if the plugin returns "wait", the framework will wait only
	// after running the remaining plugins given that no other plugin rejects the pod.
	Permit(ctx context.Context, state *CycleState, p *v1.Pod, nodeName string) (*Status, time.Duration)
}

// BindPlugin is an interface that must be implemented by "Bind" plugins. Bind
// plugins are used to bind a pod to a Node.
type BindPlugin interface {
	Plugin
	// Bind plugins will not be called until all pre-bind plugins have completed. Each
	// bind plugin is called in the configured order. A bind plugin may choose whether
	// or not to handle the given Pod. If a bind plugin chooses to handle a Pod, the
	// remaining bind plugins are skipped. When a bind plugin does not handle a pod,
	// it must return Skip in its Status code. If a bind plugin returns an Error, the
	// pod is rejected and will not be bound.
	Bind(ctx context.Context, state *CycleState, p *v1.Pod, nodeName string) *Status
}

// Framework manages the set of plugins in use by the scheduling framework.
// Configured plugins are called at specified points in a scheduling context.
type Framework interface {
	Handle
	// QueueSortFunc returns the function to sort pods in scheduling queue
	QueueSortFunc() LessFunc

	// RunPreFilterPlugins runs the set of configured PreFilter plugins. It returns
	// *Status and its code is set to non-success if any of the plugins returns
	// anything but Success. If a non-success status is returned, then the scheduling
	// cycle is aborted.
	RunPreFilterPlugins(ctx context.Context, state *CycleState, pod *v1.Pod) *Status

	// RunPostFilterPlugins runs the set of configured PostFilter plugins.
	// PostFilter plugins can either be informational, in which case should be configured
	// to execute first and return Unschedulable status, or ones that try to change the
	// cluster state to make the pod potentially schedulable in a future scheduling cycle.
	RunPostFilterPlugins(ctx context.Context, state *CycleState, pod *v1.Pod, filteredNodeStatusMap NodeToStatusMap) (*PostFilterResult, *Status)

	// RunPreBindPlugins runs the set of configured PreBind plugins. It returns
	// *Status and its code is set to non-success if any of the plugins returns
	// anything but Success. If the Status code is "Unschedulable", it is
	// considered as a scheduling check failure, otherwise, it is considered as an
	// internal error. In either case the pod is not going to be bound.
	RunPreBindPlugins(ctx context.Context, state *CycleState, pod *v1.Pod, nodeName string) *Status

	// RunPostBindPlugins runs the set of configured PostBind plugins.
	RunPostBindPlugins(ctx context.Context, state *CycleState, pod *v1.Pod, nodeName string)

	// RunReservePluginsReserve runs the Reserve method of the set of
	// configured Reserve plugins. If any of these calls returns an error, it
	// does not continue running the remaining ones and returns the error. In
	// such case, pod will not be scheduled.
	RunReservePluginsReserve(ctx context.Context, state *CycleState, pod *v1.Pod, nodeName string) *Status

	// RunReservePluginsUnreserve runs the Unreserve method of the set of
	// configured Reserve plugins.
	RunReservePluginsUnreserve(ctx context.Context, state *CycleState, pod *v1.Pod, nodeName string)

	// RunPermitPlugins runs the set of configured Permit plugins. If any of these
	// plugins returns a status other than "Success" or "Wait", it does not continue
	// running the remaining plugins and returns an error. Otherwise, if any of the
	// plugins returns "Wait", then this function will create and add waiting pod
	// to a map of currently waiting pods and return status with "Wait" code.
	// Pod will remain waiting pod for the minimum duration returned by the Permit plugins.
	RunPermitPlugins(ctx context.Context, state *CycleState, pod *v1.Pod, nodeName string) *Status

	// WaitOnPermit will block, if the pod is a waiting pod, until the waiting pod is rejected or allowed.
	WaitOnPermit(ctx context.Context, pod *v1.Pod) *Status

	// RunBindPlugins runs the set of configured Bind plugins. A Bind plugin may choose
	// whether or not to handle the given Pod. If a Bind plugin chooses to skip the
	// binding, it should return code=5("skip") status. Otherwise, it should return "Error"
	// or "Success". If none of the plugins handled binding, RunBindPlugins returns
	// code=5("skip") status.
	RunBindPlugins(ctx context.Context, state *CycleState, pod *v1.Pod, nodeName string) *Status

	// HasFilterPlugins returns true if at least one Filter plugin is defined.
	HasFilterPlugins() bool

	// HasPostFilterPlugins returns true if at least one PostFilter plugin is defined.
	HasPostFilterPlugins() bool

	// HasScorePlugins returns true if at least one Score plugin is defined.
	HasScorePlugins() bool

	// ListPlugins returns a map of extension point name to list of configured Plugins.
	ListPlugins() *config.Plugins

	// ProfileName returns the profile name associated to this framework.
	ProfileName() string
}

// Handle provides data and some tools that plugins can use. It is
// passed to the plugin factories at the time of plugin initialization. Plugins
// must store and use this handle to call framework functions.
type Handle interface {
	// PodNominator abstracts operations to maintain nominated Pods.
	PodNominator
	// PluginsRunner abstracts operations to run some plugins.
	PluginsRunner
	// SnapshotSharedLister returns listers from the latest NodeInfo Snapshot. The snapshot
	// is taken at the beginning of a scheduling cycle and remains unchanged until
	// a pod finishes "Permit" point. There is no guarantee that the information
	// remains unchanged in the binding phase of scheduling, so plugins in the binding
	// cycle (pre-bind/bind/post-bind/un-reserve plugin) should not use it,
	// otherwise a concurrent read/write error might occur, they should use scheduler
	// cache instead.
	SnapshotSharedLister() SharedLister

	// IterateOverWaitingPods acquires a read lock and iterates over the WaitingPods map.
	IterateOverWaitingPods(callback func(WaitingPod))

	// GetWaitingPod returns a waiting pod given its UID.
	GetWaitingPod(uid types.UID) WaitingPod

	// RejectWaitingPod rejects a waiting pod given its UID.
	RejectWaitingPod(uid types.UID)

	// ClientSet returns a kubernetes clientSet.
	ClientSet() clientset.Interface

	// KubeConfig returns the raw kube config.
	KubeConfig() *restclient.Config

	// EventRecorder returns an event recorder.
	EventRecorder() events.EventRecorder

	SharedInformerFactory() informers.SharedInformerFactory

	// RunFilterPluginsWithNominatedPods runs the set of configured filter plugins for nominated pod on the given node.
	RunFilterPluginsWithNominatedPods(ctx context.Context, state *CycleState, pod *v1.Pod, info *NodeInfo) *Status

	// Extenders returns registered scheduler extenders.
	Extenders() []Extender

	// Parallelizer returns a parallelizer holding parallelism for scheduler.
	Parallelizer() parallelize.Parallelizer
}

// PostFilterResult wraps needed info for scheduler framework to act upon PostFilter phase.
type PostFilterResult struct {
	NominatedNodeName string
}

// PodNominator abstracts operations to maintain nominated Pods.
type PodNominator interface {
	// AddNominatedPod adds the given pod to the nominator or
	// updates it if it already exists.
	AddNominatedPod(pod *PodInfo, nodeName string)
	// DeleteNominatedPodIfExists deletes nominatedPod from internal cache. It's a no-op if it doesn't exist.
	DeleteNominatedPodIfExists(pod *v1.Pod)
	// UpdateNominatedPod updates the <oldPod> with <newPod>.
	UpdateNominatedPod(oldPod *v1.Pod, newPodInfo *PodInfo)
	// NominatedPodsForNode returns nominatedPods on the given node.
	NominatedPodsForNode(nodeName string) []*PodInfo
}

// PluginsRunner abstracts operations to run some plugins.
// This is used by preemption PostFilter plugins when evaluating the feasibility of
// scheduling the pod on nodes when certain running pods get evicted.
type PluginsRunner interface {
	// RunPreScorePlugins runs the set of configured PreScore plugins. If any
	// of these plugins returns any status other than "Success", the given pod is rejected.
	RunPreScorePlugins(context.Context, *CycleState, *v1.Pod, []*v1.Node) *Status
	// RunScorePlugins runs the set of configured Score plugins. It returns a map that
	// stores for each Score plugin name the corresponding NodeScoreList(s).
	// It also returns *Status, which is set to non-success if any of the plugins returns
	// a non-success status.
	RunScorePlugins(context.Context, *CycleState, *v1.Pod, []*v1.Node) (PluginToNodeScores, *Status)
	// RunFilterPlugins runs the set of configured Filter plugins for pod on
	// the given node. Note that for the node being evaluated, the passed nodeInfo
	// reference could be different from the one in NodeInfoSnapshot map (e.g., pods
	// considered to be running on the node could be different). For example, during
	// preemption, we may pass a copy of the original nodeInfo object that has some pods
	// removed from it to evaluate the possibility of preempting them to
	// schedule the target pod.
	RunFilterPlugins(context.Context, *CycleState, *v1.Pod, *NodeInfo) PluginToStatus
	// RunPreFilterExtensionAddPod calls the AddPod interface for the set of configured
	// PreFilter plugins. It returns directly if any of the plugins return any
	// status other than Success.
	RunPreFilterExtensionAddPod(ctx context.Context, state *CycleState, podToSchedule *v1.Pod, podInfoToAdd *PodInfo, nodeInfo *NodeInfo) *Status
	// RunPreFilterExtensionRemovePod calls the RemovePod interface for the set of configured
	// PreFilter plugins. It returns directly if any of the plugins return any
	// status other than Success.
	RunPreFilterExtensionRemovePod(ctx context.Context, state *CycleState, podToSchedule *v1.Pod, podInfoToRemove *PodInfo, nodeInfo *NodeInfo) *Status
}
