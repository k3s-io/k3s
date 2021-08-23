// +build !providerless

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

package gcepd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"k8s.io/klog/v2"
	"k8s.io/mount-utils"
	utilexec "k8s.io/utils/exec"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/volume"
	volumeutil "k8s.io/kubernetes/pkg/volume/util"
	"k8s.io/legacy-cloud-providers/gce"
)

type gcePersistentDiskAttacher struct {
	host     volume.VolumeHost
	gceDisks gce.Disks
}

var _ volume.Attacher = &gcePersistentDiskAttacher{}

var _ volume.DeviceMounter = &gcePersistentDiskAttacher{}

var _ volume.AttachableVolumePlugin = &gcePersistentDiskPlugin{}

var _ volume.DeviceMountableVolumePlugin = &gcePersistentDiskPlugin{}

func (plugin *gcePersistentDiskPlugin) NewAttacher() (volume.Attacher, error) {
	gceCloud, err := getCloudProvider(plugin.host.GetCloudProvider())
	if err != nil {
		return nil, err
	}

	return &gcePersistentDiskAttacher{
		host:     plugin.host,
		gceDisks: gceCloud,
	}, nil
}

func (plugin *gcePersistentDiskPlugin) NewDeviceMounter() (volume.DeviceMounter, error) {
	return plugin.NewAttacher()
}

func (plugin *gcePersistentDiskPlugin) GetDeviceMountRefs(deviceMountPath string) ([]string, error) {
	mounter := plugin.host.GetMounter(plugin.GetPluginName())
	return mounter.GetMountRefs(deviceMountPath)
}

// Attach checks with the GCE cloud provider if the specified volume is already
// attached to the node with the specified Name.
// If the volume is attached, it succeeds (returns nil).
// If it is not, Attach issues a call to the GCE cloud provider to attach it.
// Callers are responsible for retrying on failure.
// Callers are responsible for thread safety between concurrent attach and
// detach operations.
func (attacher *gcePersistentDiskAttacher) Attach(spec *volume.Spec, nodeName types.NodeName) (string, error) {
	volumeSource, readOnly, err := getVolumeSource(spec)
	if err != nil {
		return "", err
	}

	pdName := volumeSource.PDName

	attached, err := attacher.gceDisks.DiskIsAttached(pdName, nodeName)
	if err != nil {
		// Log error and continue with attach
		klog.Errorf(
			"Error checking if PD (%q) is already attached to current node (%q). Will continue and try attach anyway. err=%v",
			pdName, nodeName, err)
	}

	if err == nil && attached {
		// Volume is already attached to node.
		klog.Infof("Attach operation is successful. PD %q is already attached to node %q.", pdName, nodeName)
	} else {
		if err := attacher.gceDisks.AttachDisk(pdName, nodeName, readOnly, isRegionalPD(spec)); err != nil {
			klog.Errorf("Error attaching PD %q to node %q: %+v", pdName, nodeName, err)
			return "", err
		}
	}

	return filepath.Join(diskByIDPath, diskGooglePrefix+pdName), nil
}

func (attacher *gcePersistentDiskAttacher) VolumesAreAttached(specs []*volume.Spec, nodeName types.NodeName) (map[*volume.Spec]bool, error) {
	volumesAttachedCheck := make(map[*volume.Spec]bool)
	volumePdNameMap := make(map[string]*volume.Spec)
	pdNameList := []string{}
	for _, spec := range specs {
		volumeSource, _, err := getVolumeSource(spec)
		// If error is occurred, skip this volume and move to the next one
		if err != nil {
			klog.Errorf("Error getting volume (%q) source : %v", spec.Name(), err)
			continue
		}
		pdNameList = append(pdNameList, volumeSource.PDName)
		volumesAttachedCheck[spec] = true
		volumePdNameMap[volumeSource.PDName] = spec
	}
	attachedResult, err := attacher.gceDisks.DisksAreAttached(pdNameList, nodeName)
	if err != nil {
		// Log error and continue with attach
		klog.Errorf(
			"Error checking if PDs (%v) are already attached to current node (%q). err=%v",
			pdNameList, nodeName, err)
		return volumesAttachedCheck, err
	}

	for pdName, attached := range attachedResult {
		if !attached {
			spec := volumePdNameMap[pdName]
			volumesAttachedCheck[spec] = false
			klog.V(2).Infof("VolumesAreAttached: check volume %q (specName: %q) is no longer attached", pdName, spec.Name())
		}
	}
	return volumesAttachedCheck, nil
}

func (attacher *gcePersistentDiskAttacher) BulkVerifyVolumes(volumesByNode map[types.NodeName][]*volume.Spec) (map[types.NodeName]map[*volume.Spec]bool, error) {
	volumesAttachedCheck := make(map[types.NodeName]map[*volume.Spec]bool)
	diskNamesByNode := make(map[types.NodeName][]string)
	volumeSpecToDiskName := make(map[*volume.Spec]string)

	for nodeName, volumeSpecs := range volumesByNode {
		diskNames := []string{}
		for _, spec := range volumeSpecs {
			volumeSource, _, err := getVolumeSource(spec)
			if err != nil {
				klog.Errorf("Error getting volume (%q) source : %v", spec.Name(), err)
				continue
			}
			diskNames = append(diskNames, volumeSource.PDName)
			volumeSpecToDiskName[spec] = volumeSource.PDName
		}
		diskNamesByNode[nodeName] = diskNames
	}

	attachedDisksByNode, err := attacher.gceDisks.BulkDisksAreAttached(diskNamesByNode)
	if err != nil {
		return nil, err
	}

	for nodeName, volumeSpecs := range volumesByNode {
		volumesAreAttachedToNode := make(map[*volume.Spec]bool)
		for _, spec := range volumeSpecs {
			diskName := volumeSpecToDiskName[spec]
			volumesAreAttachedToNode[spec] = attachedDisksByNode[nodeName][diskName]
		}
		volumesAttachedCheck[nodeName] = volumesAreAttachedToNode
	}
	return volumesAttachedCheck, nil
}

// search Windows disk number by LUN
func getDiskID(pdName string, exec utilexec.Interface) (string, error) {
	// TODO: replace Get-GcePdName with native windows support of Get-Disk, see issue #74674
	cmd := `Get-GcePdName | select Name, DeviceId | ConvertTo-Json`
	output, err := exec.Command("powershell", "/c", cmd).CombinedOutput()
	if err != nil {
		klog.Errorf("Get-GcePdName failed, error: %v, output: %q", err, string(output))
		err = errors.New(err.Error() + " " + string(output))
		return "", err
	}

	var data []map[string]interface{}
	if err = json.Unmarshal(output, &data); err != nil {
		klog.Errorf("Get-Disk output is not a json array, output: %q", string(output))
		return "", err
	}

	for _, pd := range data {
		if jsonName, ok := pd["Name"]; ok {
			if name, ok := jsonName.(string); ok {
				if name == pdName {
					klog.Infof("found the disk %q", name)
					if diskNum, ok := pd["DeviceId"]; ok {
						switch v := diskNum.(type) {
						case int:
							return strconv.Itoa(v), nil
						case float64:
							return strconv.Itoa(int(v)), nil
						case string:
							return v, nil
						default:
							// diskNum isn't one of the types above
							klog.Warningf("Disk %q found, but disknumber (%q) is not in one of the recongnized type", name, diskNum)
						}
					}
				}
			}
		}
	}

	return "", fmt.Errorf("could not found disk number for disk %q", pdName)
}

func (attacher *gcePersistentDiskAttacher) WaitForAttach(spec *volume.Spec, devicePath string, _ *v1.Pod, timeout time.Duration) (string, error) {
	ticker := time.NewTicker(checkSleepDuration)
	defer ticker.Stop()
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	volumeSource, _, err := getVolumeSource(spec)
	if err != nil {
		return "", err
	}

	pdName := volumeSource.PDName

	if runtime.GOOS == "windows" {
		exec := attacher.host.GetExec(gcePersistentDiskPluginName)
		id, err := getDiskID(pdName, exec)
		if err != nil {
			klog.Errorf("WaitForAttach (windows) failed with error %s", err)
		}
		return id, err
	}

	partition := ""
	if volumeSource.Partition != 0 {
		partition = strconv.Itoa(int(volumeSource.Partition))
	}

	sdBefore, err := filepath.Glob(diskSDPattern)
	if err != nil {
		klog.Errorf("Error filepath.Glob(\"%s\"): %v\r\n", diskSDPattern, err)
	}
	sdBeforeSet := sets.NewString(sdBefore...)

	devicePaths := getDiskByIDPaths(pdName, partition)
	for {
		select {
		case <-ticker.C:
			klog.V(5).Infof("Checking GCE PD %q is attached.", pdName)
			path, err := verifyDevicePath(devicePaths, sdBeforeSet, pdName)
			if err != nil {
				// Log error, if any, and continue checking periodically. See issue #11321
				klog.Errorf("Error verifying GCE PD (%q) is attached: %v", pdName, err)
			} else if path != "" {
				// A device path has successfully been created for the PD
				klog.Infof("Successfully found attached GCE PD %q.", pdName)
				return path, nil
			} else {
				klog.V(4).Infof("could not verify GCE PD (%q) is attached, device path does not exist", pdName)
			}
		case <-timer.C:
			return "", fmt.Errorf("could not find attached GCE PD %q. Timeout waiting for mount paths to be created", pdName)
		}
	}
}

func (attacher *gcePersistentDiskAttacher) GetDeviceMountPath(
	spec *volume.Spec) (string, error) {
	volumeSource, _, err := getVolumeSource(spec)
	if err != nil {
		return "", err
	}

	return makeGlobalPDName(attacher.host, volumeSource.PDName), nil
}

func (attacher *gcePersistentDiskAttacher) MountDevice(spec *volume.Spec, devicePath string, deviceMountPath string, _ volume.DeviceMounterArgs) error {
	// Only mount the PD globally once.
	mounter := attacher.host.GetMounter(gcePersistentDiskPluginName)
	notMnt, err := mounter.IsLikelyNotMountPoint(deviceMountPath)
	if err != nil {
		if os.IsNotExist(err) {
			dir := deviceMountPath
			if runtime.GOOS == "windows" {
				// in windows, as we use mklink, only need to MkdirAll for parent directory
				dir = filepath.Dir(deviceMountPath)
			}
			if err := os.MkdirAll(dir, 0750); err != nil {
				return fmt.Errorf("MountDevice:CreateDirectory failed with %s", err)
			}
			notMnt = true
		} else {
			return err
		}
	}

	volumeSource, readOnly, err := getVolumeSource(spec)
	if err != nil {
		return err
	}

	options := []string{}
	if readOnly {
		options = append(options, "ro")
	}
	if notMnt {
		diskMounter := volumeutil.NewSafeFormatAndMountFromHost(gcePersistentDiskPluginName, attacher.host)
		mountOptions := volumeutil.MountOptionFromSpec(spec, options...)
		err = diskMounter.FormatAndMount(devicePath, deviceMountPath, volumeSource.FSType, mountOptions)
		if err != nil {
			os.Remove(deviceMountPath)
			return err
		}
		klog.V(4).Infof("formatting spec %v devicePath %v deviceMountPath %v fs %v with options %+v", spec.Name(), devicePath, deviceMountPath, volumeSource.FSType, options)
	}
	return nil
}

type gcePersistentDiskDetacher struct {
	host     volume.VolumeHost
	gceDisks gce.Disks
}

var _ volume.Detacher = &gcePersistentDiskDetacher{}

var _ volume.DeviceUnmounter = &gcePersistentDiskDetacher{}

func (plugin *gcePersistentDiskPlugin) NewDetacher() (volume.Detacher, error) {
	gceCloud, err := getCloudProvider(plugin.host.GetCloudProvider())
	if err != nil {
		return nil, err
	}

	return &gcePersistentDiskDetacher{
		host:     plugin.host,
		gceDisks: gceCloud,
	}, nil
}

func (plugin *gcePersistentDiskPlugin) NewDeviceUnmounter() (volume.DeviceUnmounter, error) {
	return plugin.NewDetacher()
}

// Detach checks with the GCE cloud provider if the specified volume is already
// attached to the specified node. If the volume is not attached, it succeeds
// (returns nil). If it is attached, Detach issues a call to the GCE cloud
// provider to attach it.
// Callers are responsible for retrying on failure.
// Callers are responsible for thread safety between concurrent attach and detach
// operations.
func (detacher *gcePersistentDiskDetacher) Detach(volumeName string, nodeName types.NodeName) error {
	pdName := path.Base(volumeName)

	attached, err := detacher.gceDisks.DiskIsAttached(pdName, nodeName)
	if err != nil {
		// Log error and continue with detach
		klog.Errorf(
			"Error checking if PD (%q) is already attached to current node (%q). Will continue and try detach anyway. err=%v",
			pdName, nodeName, err)
	}

	if err == nil && !attached {
		// Volume is not attached to node. Success!
		klog.Infof("Detach operation is successful. PD %q was not attached to node %q.", pdName, nodeName)
		return nil
	}

	if err = detacher.gceDisks.DetachDisk(pdName, nodeName); err != nil {
		klog.Errorf("Error detaching PD %q from node %q: %v", pdName, nodeName, err)
		return err
	}

	return nil
}

func (detacher *gcePersistentDiskDetacher) UnmountDevice(deviceMountPath string) error {
	if runtime.GOOS == "windows" {
		// Flush data cache for windows because it does not do so automatically during unmount device
		exec := detacher.host.GetExec(gcePersistentDiskPluginName)
		err := volumeutil.WriteVolumeCache(deviceMountPath, exec)
		if err != nil {
			return err
		}
	}
	return mount.CleanupMountPoint(deviceMountPath, detacher.host.GetMounter(gcePersistentDiskPluginName), false)
}

func (plugin *gcePersistentDiskPlugin) CanAttach(spec *volume.Spec) (bool, error) {
	return true, nil
}

func (plugin *gcePersistentDiskPlugin) CanDeviceMount(spec *volume.Spec) (bool, error) {
	return true, nil
}
