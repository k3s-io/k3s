/*
Copyright 2016 The Kubernetes Authors.

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

// Package reconciler implements interfaces that attempt to reconcile the
// desired state of the with the actual state of the world by triggering
// actions.
package reconciler

import (
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/controller/volume/attachdetach/cache"
	"k8s.io/kubernetes/pkg/controller/volume/attachdetach/metrics"
	"k8s.io/kubernetes/pkg/controller/volume/attachdetach/statusupdater"
	kevents "k8s.io/kubernetes/pkg/kubelet/events"
	"k8s.io/kubernetes/pkg/util/goroutinemap/exponentialbackoff"
	"k8s.io/kubernetes/pkg/volume/util"
	"k8s.io/kubernetes/pkg/volume/util/operationexecutor"
)

// Reconciler runs a periodic loop to reconcile the desired state of the world with
// the actual state of the world by triggering attach detach operations.
// Note: This is distinct from the Reconciler implemented by the kubelet volume
// manager. This reconciles state for the attach/detach controller. That
// reconciles state for the kubelet volume manager.
type Reconciler interface {
	// Starts running the reconciliation loop which executes periodically, checks
	// if volumes that should be attached are attached and volumes that should
	// be detached are detached. If not, it will trigger attach/detach
	// operations to rectify.
	Run(stopCh <-chan struct{})
}

// NewReconciler returns a new instance of Reconciler that waits loopPeriod
// between successive executions.
// loopPeriod is the amount of time the reconciler loop waits between
// successive executions.
// maxWaitForUnmountDuration is the max amount of time the reconciler will wait
// for the volume to be safely unmounted, after this it will detach the volume
// anyway (to handle crashed/unavailable nodes). If during this time the volume
// becomes used by a new pod, the detach request will be aborted and the timer
// cleared.
func NewReconciler(
	loopPeriod time.Duration,
	maxWaitForUnmountDuration time.Duration,
	syncDuration time.Duration,
	disableReconciliationSync bool,
	desiredStateOfWorld cache.DesiredStateOfWorld,
	actualStateOfWorld cache.ActualStateOfWorld,
	attacherDetacher operationexecutor.OperationExecutor,
	nodeStatusUpdater statusupdater.NodeStatusUpdater,
	recorder record.EventRecorder) Reconciler {
	return &reconciler{
		loopPeriod:                loopPeriod,
		maxWaitForUnmountDuration: maxWaitForUnmountDuration,
		syncDuration:              syncDuration,
		disableReconciliationSync: disableReconciliationSync,
		desiredStateOfWorld:       desiredStateOfWorld,
		actualStateOfWorld:        actualStateOfWorld,
		attacherDetacher:          attacherDetacher,
		nodeStatusUpdater:         nodeStatusUpdater,
		timeOfLastSync:            time.Now(),
		recorder:                  recorder,
	}
}

type reconciler struct {
	loopPeriod                time.Duration
	maxWaitForUnmountDuration time.Duration
	syncDuration              time.Duration
	desiredStateOfWorld       cache.DesiredStateOfWorld
	actualStateOfWorld        cache.ActualStateOfWorld
	attacherDetacher          operationexecutor.OperationExecutor
	nodeStatusUpdater         statusupdater.NodeStatusUpdater
	timeOfLastSync            time.Time
	disableReconciliationSync bool
	recorder                  record.EventRecorder
}

func (rc *reconciler) Run(stopCh <-chan struct{}) {
	wait.Until(rc.reconciliationLoopFunc(), rc.loopPeriod, stopCh)
}

// reconciliationLoopFunc this can be disabled via cli option disableReconciliation.
// It periodically checks whether the attached volumes from actual state
// are still attached to the node and update the status if they are not.
func (rc *reconciler) reconciliationLoopFunc() func() {
	return func() {

		rc.reconcile()

		if rc.disableReconciliationSync {
			klog.V(5).Info("Skipping reconciling attached volumes still attached since it is disabled via the command line.")
		} else if rc.syncDuration < time.Second {
			klog.V(5).Info("Skipping reconciling attached volumes still attached since it is set to less than one second via the command line.")
		} else if time.Since(rc.timeOfLastSync) > rc.syncDuration {
			klog.V(5).Info("Starting reconciling attached volumes still attached")
			rc.sync()
		}
	}
}

func (rc *reconciler) sync() {
	defer rc.updateSyncTime()
	rc.syncStates()
}

func (rc *reconciler) updateSyncTime() {
	rc.timeOfLastSync = time.Now()
}

func (rc *reconciler) syncStates() {
	volumesPerNode := rc.actualStateOfWorld.GetAttachedVolumesPerNode()
	rc.attacherDetacher.VerifyVolumesAreAttached(volumesPerNode, rc.actualStateOfWorld)
}

func (rc *reconciler) reconcile() {
	// Detaches are triggered before attaches so that volumes referenced by
	// pods that are rescheduled to a different node are detached first.

	// Ensure volumes that should be detached are detached.
	for _, attachedVolume := range rc.actualStateOfWorld.GetAttachedVolumes() {
		if !rc.desiredStateOfWorld.VolumeExists(
			attachedVolume.VolumeName, attachedVolume.NodeName) {

			// Check whether there already exist an operation pending, and don't even
			// try to start an operation if there is already one running.
			// This check must be done before we do any other checks, as otherwise the other checks
			// may pass while at the same time the volume leaves the pending state, resulting in
			// double detach attempts
			// The operation key format is different depending on whether the volume
			// allows multi attach across different nodes.
			if util.IsMultiAttachAllowed(attachedVolume.VolumeSpec) {
				if !rc.attacherDetacher.IsOperationSafeToRetry(attachedVolume.VolumeName, "" /* podName */, attachedVolume.NodeName, operationexecutor.DetachOperationName) {
					klog.V(10).Infof("Operation for volume %q is already running or still in exponential backoff for node %q. Can't start detach", attachedVolume.VolumeName, attachedVolume.NodeName)
					continue
				}
			} else {
				if !rc.attacherDetacher.IsOperationSafeToRetry(attachedVolume.VolumeName, "" /* podName */, "" /* nodeName */, operationexecutor.DetachOperationName) {
					klog.V(10).Infof("Operation for volume %q is already running or still in exponential backoff in the cluster. Can't start detach for %q", attachedVolume.VolumeName, attachedVolume.NodeName)
					continue
				}
			}

			// Because the detach operation updates the ActualStateOfWorld before
			// marking itself complete, it's possible for the volume to be removed
			// from the ActualStateOfWorld between the GetAttachedVolumes() check
			// and the IsOperationPending() check above.
			// Check the ActualStateOfWorld again to avoid issuing an unnecessary
			// detach.
			// See https://github.com/kubernetes/kubernetes/issues/93902
			attachState := rc.actualStateOfWorld.GetAttachState(attachedVolume.VolumeName, attachedVolume.NodeName)
			if attachState == cache.AttachStateDetached {
				if klog.V(5).Enabled() {
					klog.Infof(attachedVolume.GenerateMsgDetailed("Volume detached--skipping", ""))
				}
				continue
			}

			// Set the detach request time
			elapsedTime, err := rc.actualStateOfWorld.SetDetachRequestTime(attachedVolume.VolumeName, attachedVolume.NodeName)
			if err != nil {
				klog.Errorf("Cannot trigger detach because it fails to set detach request time with error %v", err)
				continue
			}
			// Check whether timeout has reached the maximum waiting time
			timeout := elapsedTime > rc.maxWaitForUnmountDuration
			// Check whether volume is still mounted. Skip detach if it is still mounted unless timeout
			if attachedVolume.MountedByNode && !timeout {
				klog.V(5).Infof(attachedVolume.GenerateMsgDetailed("Cannot detach volume because it is still mounted", ""))
				continue
			}

			// Before triggering volume detach, mark volume as detached and update the node status
			// If it fails to update node status, skip detach volume
			// If volume detach operation fails, the volume needs to be added back to report as attached so that node status
			// has the correct volume attachment information.
			err = rc.actualStateOfWorld.RemoveVolumeFromReportAsAttached(attachedVolume.VolumeName, attachedVolume.NodeName)
			if err != nil {
				klog.V(5).Infof("RemoveVolumeFromReportAsAttached failed while removing volume %q from node %q with: %v",
					attachedVolume.VolumeName,
					attachedVolume.NodeName,
					err)
			}

			// Update Node Status to indicate volume is no longer safe to mount.
			err = rc.nodeStatusUpdater.UpdateNodeStatuses()
			if err != nil {
				// Skip detaching this volume if unable to update node status
				klog.Errorf(attachedVolume.GenerateErrorDetailed("UpdateNodeStatuses failed while attempting to report volume as attached", err).Error())
				continue
			}

			// Trigger detach volume which requires verifying safe to detach step
			// If timeout is true, skip verifySafeToDetach check
			klog.V(5).Infof(attachedVolume.GenerateMsgDetailed("Starting attacherDetacher.DetachVolume", ""))
			verifySafeToDetach := !timeout
			err = rc.attacherDetacher.DetachVolume(attachedVolume.AttachedVolume, verifySafeToDetach, rc.actualStateOfWorld)
			if err == nil {
				if !timeout {
					klog.Infof(attachedVolume.GenerateMsgDetailed("attacherDetacher.DetachVolume started", ""))
				} else {
					metrics.RecordForcedDetachMetric()
					klog.Warningf(attachedVolume.GenerateMsgDetailed("attacherDetacher.DetachVolume started", fmt.Sprintf("This volume is not safe to detach, but maxWaitForUnmountDuration %v expired, force detaching", rc.maxWaitForUnmountDuration)))
				}
			}
			if err != nil {
				// Add volume back to ReportAsAttached if DetachVolume call failed so that node status updater will add it back to VolumeAttached list.
				// This function is also called during executing the volume detach operation in operation_generoator.
				// It is needed here too because DetachVolume call might fail before executing the actual operation in operation_executor (e.g., cannot find volume plugin etc.)
				rc.actualStateOfWorld.AddVolumeToReportAsAttached(attachedVolume.VolumeName, attachedVolume.NodeName)

				if !exponentialbackoff.IsExponentialBackoff(err) {
					// Ignore exponentialbackoff.IsExponentialBackoff errors, they are expected.
					// Log all other errors.
					klog.Errorf(attachedVolume.GenerateErrorDetailed("attacherDetacher.DetachVolume failed to start", err).Error())
				}
			}
		}
	}

	rc.attachDesiredVolumes()

	// Update Node Status
	err := rc.nodeStatusUpdater.UpdateNodeStatuses()
	if err != nil {
		klog.Warningf("UpdateNodeStatuses failed with: %v", err)
	}
}

func (rc *reconciler) attachDesiredVolumes() {
	// Ensure volumes that should be attached are attached.
	for _, volumeToAttach := range rc.desiredStateOfWorld.GetVolumesToAttach() {
		if util.IsMultiAttachAllowed(volumeToAttach.VolumeSpec) {
			// Don't even try to start an operation if there is already one running for the given volume and node.
			if rc.attacherDetacher.IsOperationPending(volumeToAttach.VolumeName, "" /* podName */, volumeToAttach.NodeName) {
				if klog.V(10).Enabled() {
					klog.Infof("Operation for volume %q is already running for node %q. Can't start attach", volumeToAttach.VolumeName, volumeToAttach.NodeName)
				}
				continue
			}
		} else {
			// Don't even try to start an operation if there is already one running for the given volume
			if rc.attacherDetacher.IsOperationPending(volumeToAttach.VolumeName, "" /* podName */, "" /* nodeName */) {
				if klog.V(10).Enabled() {
					klog.Infof("Operation for volume %q is already running. Can't start attach for %q", volumeToAttach.VolumeName, volumeToAttach.NodeName)
				}
				continue
			}
		}

		// Because the attach operation updates the ActualStateOfWorld before
		// marking itself complete, IsOperationPending() must be checked before
		// GetAttachState() to guarantee the ActualStateOfWorld is
		// up-to-date when it's read.
		// See https://github.com/kubernetes/kubernetes/issues/93902
		attachState := rc.actualStateOfWorld.GetAttachState(volumeToAttach.VolumeName, volumeToAttach.NodeName)
		if attachState == cache.AttachStateAttached {
			// Volume/Node exists, touch it to reset detachRequestedTime
			if klog.V(5).Enabled() {
				klog.Infof(volumeToAttach.GenerateMsgDetailed("Volume attached--touching", ""))
			}
			rc.actualStateOfWorld.ResetDetachRequestTime(volumeToAttach.VolumeName, volumeToAttach.NodeName)
			continue
		}

		if !util.IsMultiAttachAllowed(volumeToAttach.VolumeSpec) {
			nodes := rc.actualStateOfWorld.GetNodesForAttachedVolume(volumeToAttach.VolumeName)
			if len(nodes) > 0 {
				if !volumeToAttach.MultiAttachErrorReported {
					rc.reportMultiAttachError(volumeToAttach, nodes)
					rc.desiredStateOfWorld.SetMultiAttachError(volumeToAttach.VolumeName, volumeToAttach.NodeName)
				}
				continue
			}
		}

		// Volume/Node doesn't exist, spawn a goroutine to attach it
		if klog.V(5).Enabled() {
			klog.Infof(volumeToAttach.GenerateMsgDetailed("Starting attacherDetacher.AttachVolume", ""))
		}
		err := rc.attacherDetacher.AttachVolume(volumeToAttach.VolumeToAttach, rc.actualStateOfWorld)
		if err == nil {
			klog.Infof(volumeToAttach.GenerateMsgDetailed("attacherDetacher.AttachVolume started", ""))
		}
		if err != nil && !exponentialbackoff.IsExponentialBackoff(err) {
			// Ignore exponentialbackoff.IsExponentialBackoff errors, they are expected.
			// Log all other errors.
			klog.Errorf(volumeToAttach.GenerateErrorDetailed("attacherDetacher.AttachVolume failed to start", err).Error())
		}
	}
}

// reportMultiAttachError sends events and logs situation that a volume that
// should be attached to a node is already attached to different node(s).
func (rc *reconciler) reportMultiAttachError(volumeToAttach cache.VolumeToAttach, nodes []types.NodeName) {
	// Filter out the current node from list of nodes where the volume is
	// attached.
	// Some methods need []string, some other needs []NodeName, collect both.
	// In theory, these arrays should have always only one element - the
	// controller does not allow more than one attachment. But use array just
	// in case...
	otherNodes := []types.NodeName{}
	otherNodesStr := []string{}
	for _, node := range nodes {
		if node != volumeToAttach.NodeName {
			otherNodes = append(otherNodes, node)
			otherNodesStr = append(otherNodesStr, string(node))
		}
	}

	// Get list of pods that use the volume on the other nodes.
	pods := rc.desiredStateOfWorld.GetVolumePodsOnNodes(otherNodes, volumeToAttach.VolumeName)

	if len(pods) == 0 {
		// We did not find any pods that requests the volume. The pod must have been deleted already.
		simpleMsg, _ := volumeToAttach.GenerateMsg("Multi-Attach error", "Volume is already exclusively attached to one node and can't be attached to another")
		for _, pod := range volumeToAttach.ScheduledPods {
			rc.recorder.Eventf(pod, v1.EventTypeWarning, kevents.FailedAttachVolume, simpleMsg)
		}
		// Log detailed message to system admin
		nodeList := strings.Join(otherNodesStr, ", ")
		detailedMsg := volumeToAttach.GenerateMsgDetailed("Multi-Attach error", fmt.Sprintf("Volume is already exclusively attached to node %s and can't be attached to another", nodeList))
		klog.Warning(detailedMsg)
		return
	}

	// There are pods that require the volume and run on another node. Typically
	// it's user error, e.g. a ReplicaSet uses a PVC and has >1 replicas. Let
	// the user know what pods are blocking the volume.
	for _, scheduledPod := range volumeToAttach.ScheduledPods {
		// Each scheduledPod must get a custom message. They can run in
		// different namespaces and user of a namespace should not see names of
		// pods in other namespaces.
		localPodNames := []string{} // Names of pods in scheduledPods's namespace
		otherPods := 0              // Count of pods in other namespaces
		for _, pod := range pods {
			if pod.Namespace == scheduledPod.Namespace {
				localPodNames = append(localPodNames, pod.Name)
			} else {
				otherPods++
			}
		}

		var msg string
		if len(localPodNames) > 0 {
			msg = fmt.Sprintf("Volume is already used by pod(s) %s", strings.Join(localPodNames, ", "))
			if otherPods > 0 {
				msg = fmt.Sprintf("%s and %d pod(s) in different namespaces", msg, otherPods)
			}
		} else {
			// No local pods, there are pods only in different namespaces.
			msg = fmt.Sprintf("Volume is already used by %d pod(s) in different namespaces", otherPods)
		}
		simpleMsg, _ := volumeToAttach.GenerateMsg("Multi-Attach error", msg)
		rc.recorder.Eventf(scheduledPod, v1.EventTypeWarning, kevents.FailedAttachVolume, simpleMsg)
	}

	// Log all pods for system admin
	podNames := []string{}
	for _, pod := range pods {
		podNames = append(podNames, pod.Namespace+"/"+pod.Name)
	}
	detailedMsg := volumeToAttach.GenerateMsgDetailed("Multi-Attach error", fmt.Sprintf("Volume is already used by pods %s on node %s", strings.Join(podNames, ", "), strings.Join(otherNodesStr, ", ")))
	klog.Warningf(detailedMsg)
}
