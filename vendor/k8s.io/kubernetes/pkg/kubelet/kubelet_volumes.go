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

package kubelet

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"syscall"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/util/removeall"
	"k8s.io/kubernetes/pkg/volume"
	volumetypes "k8s.io/kubernetes/pkg/volume/util/types"
)

// ListVolumesForPod returns a map of the mounted volumes for the given pod.
// The key in the map is the OuterVolumeSpecName (i.e. pod.Spec.Volumes[x].Name)
func (kl *Kubelet) ListVolumesForPod(podUID types.UID) (map[string]volume.Volume, bool) {
	volumesToReturn := make(map[string]volume.Volume)
	podVolumes := kl.volumeManager.GetMountedVolumesForPod(
		volumetypes.UniquePodName(podUID))
	for outerVolumeSpecName, volume := range podVolumes {
		// TODO: volume.Mounter could be nil if volume object is recovered
		// from reconciler's sync state process. PR 33616 will fix this problem
		// to create Mounter object when recovering volume state.
		if volume.Mounter == nil {
			continue
		}
		volumesToReturn[outerVolumeSpecName] = volume.Mounter
	}

	return volumesToReturn, len(volumesToReturn) > 0
}

// podVolumesExist checks with the volume manager and returns true any of the
// pods for the specified volume are mounted.
func (kl *Kubelet) podVolumesExist(podUID types.UID) bool {
	if mountedVolumes :=
		kl.volumeManager.GetMountedVolumesForPod(
			volumetypes.UniquePodName(podUID)); len(mountedVolumes) > 0 {
		return true
	}
	// TODO: This checks pod volume paths and whether they are mounted. If checking returns error, podVolumesExist will return true
	// which means we consider volumes might exist and requires further checking.
	// There are some volume plugins such as flexvolume might not have mounts. See issue #61229
	volumePaths, err := kl.getMountedVolumePathListFromDisk(podUID)
	if err != nil {
		klog.Errorf("pod %q found, but error %v occurred during checking mounted volumes from disk", podUID, err)
		return true
	}
	if len(volumePaths) > 0 {
		klog.V(4).Infof("pod %q found, but volumes are still mounted on disk %v", podUID, volumePaths)
		return true
	}

	return false
}

// newVolumeMounterFromPlugins attempts to find a plugin by volume spec, pod
// and volume options and then creates a Mounter.
// Returns a valid mounter or an error.
func (kl *Kubelet) newVolumeMounterFromPlugins(spec *volume.Spec, pod *v1.Pod, opts volume.VolumeOptions) (volume.Mounter, error) {
	plugin, err := kl.volumePluginMgr.FindPluginBySpec(spec)
	if err != nil {
		return nil, fmt.Errorf("can't use volume plugins for %s: %v", spec.Name(), err)
	}
	physicalMounter, err := plugin.NewMounter(spec, pod, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate mounter for volume: %s using plugin: %s with a root cause: %v", spec.Name(), plugin.GetPluginName(), err)
	}
	klog.V(10).Infof("Using volume plugin %q to mount %s", plugin.GetPluginName(), spec.Name())
	return physicalMounter, nil
}

// removeOrphanedPodVolumeDirs attempts to remove the pod volumes directory and
// its subdirectories. There should be no files left under normal conditions
// when this is called, so it effectively does a recursive rmdir instead of
// RemoveAll to ensure it only removes directories and not regular files.
func (kl *Kubelet) removeOrphanedPodVolumeDirs(uid types.UID) []error {
	orphanVolumeErrors := []error{}

	// If there are still volume directories, attempt to rmdir them
	volumePaths, err := kl.getPodVolumePathListFromDisk(uid)
	if err != nil {
		orphanVolumeErrors = append(orphanVolumeErrors, fmt.Errorf("orphaned pod %q found, but error %v occurred during reading volume dir from disk", uid, err))
		return orphanVolumeErrors
	}
	if len(volumePaths) > 0 {
		for _, volumePath := range volumePaths {
			if err := syscall.Rmdir(volumePath); err != nil {
				orphanVolumeErrors = append(orphanVolumeErrors, fmt.Errorf("orphaned pod %q found, but failed to rmdir() volume at path %v: %v", uid, volumePath, err))
			} else {
				klog.Warningf("Cleaned up orphaned volume from pod %q at %s", uid, volumePath)
			}
		}
	}

	// If there are any volume-subpaths, attempt to rmdir them
	subpathVolumePaths, err := kl.getPodVolumeSubpathListFromDisk(uid)
	if err != nil {
		orphanVolumeErrors = append(orphanVolumeErrors, fmt.Errorf("orphaned pod %q found, but error %v occurred during reading of volume-subpaths dir from disk", uid, err))
		return orphanVolumeErrors
	}
	if len(subpathVolumePaths) > 0 {
		for _, subpathVolumePath := range subpathVolumePaths {
			if err := syscall.Rmdir(subpathVolumePath); err != nil {
				orphanVolumeErrors = append(orphanVolumeErrors, fmt.Errorf("orphaned pod %q found, but failed to rmdir() subpath at path %v: %v", uid, subpathVolumePath, err))
			} else {
				klog.Warningf("Cleaned up orphaned volume subpath from pod %q at %s", uid, subpathVolumePath)
			}
		}
	}

	// Remove any remaining subdirectories along with the volumes directory itself.
	// Fail if any regular files are encountered.
	podVolDir := kl.getPodVolumesDir(uid)
	if err := removeall.RemoveDirsOneFilesystem(kl.mounter, podVolDir); err != nil {
		orphanVolumeErrors = append(orphanVolumeErrors, fmt.Errorf("orphaned pod %q found, but error %v occurred when trying to remove the volumes dir", uid, err))
	} else {
		klog.Warningf("Cleaned up orphaned pod volumes dir from pod %q at %s", uid, podVolDir)
	}

	return orphanVolumeErrors
}

// cleanupOrphanedPodDirs removes the volumes of pods that should not be
// running and that have no containers running.  Note that we roll up logs here since it runs in the main loop.
func (kl *Kubelet) cleanupOrphanedPodDirs(pods []*v1.Pod, runningPods []*kubecontainer.Pod) error {
	allPods := sets.NewString()
	for _, pod := range pods {
		allPods.Insert(string(pod.UID))
	}
	for _, pod := range runningPods {
		allPods.Insert(string(pod.ID))
	}

	found, err := kl.listPodsFromDisk()
	if err != nil {
		return err
	}

	orphanRemovalErrors := []error{}
	orphanVolumeErrors := []error{}

	for _, uid := range found {
		if allPods.Has(string(uid)) {
			continue
		}
		// If volumes have not been unmounted/detached, do not delete directory.
		// Doing so may result in corruption of data.
		// TODO: getMountedVolumePathListFromDisk() call may be redundant with
		// kl.getPodVolumePathListFromDisk(). Can this be cleaned up?
		if podVolumesExist := kl.podVolumesExist(uid); podVolumesExist {
			klog.V(3).Infof("Orphaned pod %q found, but volumes are not cleaned up", uid)
			continue
		}

		// Attempt to remove the pod volumes directory and its subdirs
		podVolumeErrors := kl.removeOrphanedPodVolumeDirs(uid)
		if len(podVolumeErrors) > 0 {
			orphanVolumeErrors = append(orphanVolumeErrors, podVolumeErrors...)
			// Not all volumes were removed, so don't clean up the pod directory yet. It is likely
			// that there are still mountpoints or files left which could cause removal of the pod
			// directory to fail below.
			// Errors for all removal operations have already been recorded, so don't add another
			// one here.
			continue
		}

		// Call RemoveAllOneFilesystem for remaining subdirs under the pod directory
		podDir := kl.getPodDir(uid)
		podSubdirs, err := ioutil.ReadDir(podDir)
		if err != nil {
			klog.Errorf("Could not read directory %q; err: %v", podDir, err)
			orphanRemovalErrors = append(orphanRemovalErrors, fmt.Errorf("orphaned pod %q found, but error %v occurred during reading the pod dir from disk", uid, err))
			continue
		}
		for _, podSubdir := range podSubdirs {
			podSubdirName := podSubdir.Name()
			podSubdirPath := filepath.Join(podDir, podSubdirName)
			// Never attempt RemoveAllOneFilesystem on the volumes directory,
			// as this could lead to data loss in some situations. The volumes
			// directory should have been removed by removeOrphanedPodVolumeDirs.
			if podSubdirName == "volumes" {
				err := fmt.Errorf("volumes subdir was found after it was removed")
				klog.Errorf("Orphaned pod %q found, but failed to remove volumes subdir %q; err: %v", uid, podSubdirPath, err)
				continue
			}
			if err := removeall.RemoveAllOneFilesystem(kl.mounter, podSubdirPath); err != nil {
				klog.Errorf("Failed to remove orphaned pod %q subdir %q; err: %v", uid, podSubdirPath, err)
				orphanRemovalErrors = append(orphanRemovalErrors, fmt.Errorf("orphaned pod %q found, but error %v occurred when trying to remove subdir %q", uid, err, podSubdirPath))
			}
		}

		// Rmdir the pod dir, which should be empty if everything above was successful
		klog.V(3).Infof("Orphaned pod %q found, removing", uid)
		if err := syscall.Rmdir(podDir); err != nil {
			klog.Errorf("Failed to remove orphaned pod %q dir; err: %v", uid, err)
			orphanRemovalErrors = append(orphanRemovalErrors, fmt.Errorf("orphaned pod %q found, but error %v occurred when trying to remove the pod directory", uid, err))
		}
	}

	logSpew := func(errs []error) {
		if len(errs) > 0 {
			klog.Errorf("%v : There were a total of %v errors similar to this. Turn up verbosity to see them.", errs[0], len(errs))
			for _, err := range errs {
				klog.V(5).Infof("Orphan pod: %v", err)
			}
		}
	}
	logSpew(orphanVolumeErrors)
	logSpew(orphanRemovalErrors)
	return utilerrors.NewAggregate(orphanRemovalErrors)
}
