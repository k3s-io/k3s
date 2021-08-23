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

package azuredd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-12-01/compute"
	"k8s.io/klog/v2"
	"k8s.io/mount-utils"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/kubernetes/pkg/volume"
	"k8s.io/kubernetes/pkg/volume/util"
	"k8s.io/legacy-cloud-providers/azure"
)

type azureDiskDetacher struct {
	plugin *azureDataDiskPlugin
	cloud  *azure.Cloud
}

type azureDiskAttacher struct {
	plugin *azureDataDiskPlugin
	cloud  *azure.Cloud
}

var _ volume.Attacher = &azureDiskAttacher{}
var _ volume.Detacher = &azureDiskDetacher{}

var _ volume.DeviceMounter = &azureDiskAttacher{}
var _ volume.DeviceUnmounter = &azureDiskDetacher{}

// Attach attaches a volume.Spec to an Azure VM referenced by NodeName, returning the disk's LUN
func (a *azureDiskAttacher) Attach(spec *volume.Spec, nodeName types.NodeName) (string, error) {
	volumeSource, _, err := getVolumeSource(spec)
	if err != nil {
		klog.Warningf("failed to get azure disk spec (%v)", err)
		return "", err
	}

	diskController, err := getDiskController(a.plugin.host)
	if err != nil {
		return "", err
	}

	lun, err := diskController.GetDiskLun(volumeSource.DiskName, volumeSource.DataDiskURI, nodeName)
	if err == cloudprovider.InstanceNotFound {
		// Log error and continue with attach
		klog.Warningf(
			"Error checking if volume is already attached to current node (%q). Will continue and try attach anyway. err=%v",
			nodeName, err)
	}

	if err == nil {
		// Volume is already attached to node.
		klog.V(2).Infof("Attach operation is successful. volume %q is already attached to node %q at lun %d.", volumeSource.DiskName, nodeName, lun)
	} else {
		klog.V(2).Infof("GetDiskLun returned: %v. Initiating attaching volume %q to node %q.", err, volumeSource.DataDiskURI, nodeName)

		isManagedDisk := (*volumeSource.Kind == v1.AzureManagedDisk)
		lun, err = diskController.AttachDisk(isManagedDisk, volumeSource.DiskName, volumeSource.DataDiskURI, nodeName, compute.CachingTypes(*volumeSource.CachingMode))
		if err == nil {
			klog.V(2).Infof("Attach operation successful: volume %q attached to node %q.", volumeSource.DataDiskURI, nodeName)
		} else {
			klog.V(2).Infof("Attach volume %q to instance %q failed with %v", volumeSource.DataDiskURI, nodeName, err)
			return "", err
		}
	}

	return strconv.Itoa(int(lun)), err
}

func (a *azureDiskAttacher) VolumesAreAttached(specs []*volume.Spec, nodeName types.NodeName) (map[*volume.Spec]bool, error) {
	volumesAttachedCheck := make(map[*volume.Spec]bool)
	volumeSpecMap := make(map[string]*volume.Spec)
	volumeIDList := []string{}
	for _, spec := range specs {
		volumeSource, _, err := getVolumeSource(spec)
		if err != nil {
			klog.Errorf("azureDisk - Error getting volume (%q) source : %v", spec.Name(), err)
			continue
		}

		volumeIDList = append(volumeIDList, volumeSource.DiskName)
		volumesAttachedCheck[spec] = true
		volumeSpecMap[volumeSource.DiskName] = spec
	}

	diskController, err := getDiskController(a.plugin.host)
	if err != nil {
		return nil, err
	}
	attachedResult, err := diskController.DisksAreAttached(volumeIDList, nodeName)
	if err != nil {
		// Log error and continue with attach
		klog.Errorf(
			"azureDisk - Error checking if volumes (%v) are attached to current node (%q). err=%v",
			volumeIDList, nodeName, err)
		return volumesAttachedCheck, err
	}

	for volumeID, attached := range attachedResult {
		if !attached {
			spec := volumeSpecMap[volumeID]
			volumesAttachedCheck[spec] = false
			klog.V(2).Infof("azureDisk - VolumesAreAttached: check volume %q (specName: %q) is no longer attached", volumeID, spec.Name())
		}
	}
	return volumesAttachedCheck, nil
}

func (a *azureDiskAttacher) WaitForAttach(spec *volume.Spec, devicePath string, _ *v1.Pod, timeout time.Duration) (string, error) {
	// devicePath could be a LUN number or
	// "/dev/disk/azure/scsi1/lunx", "/dev/sdx" on Linux node
	// "/dev/diskx" on Windows node
	if strings.HasPrefix(devicePath, "/dev/") {
		return devicePath, nil
	}

	volumeSource, _, err := getVolumeSource(spec)
	if err != nil {
		return "", err
	}

	nodeName := types.NodeName(a.plugin.host.GetHostName())
	diskName := volumeSource.DiskName

	lun, err := strconv.Atoi(devicePath)
	if err != nil {
		return "", fmt.Errorf("parse %s failed with error: %v, diskName: %s, nodeName: %s", devicePath, err, diskName, nodeName)
	}

	exec := a.plugin.host.GetExec(a.plugin.GetPluginName())

	io := &osIOHandler{}
	scsiHostRescan(io, exec)

	newDevicePath := ""

	err = wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
		if newDevicePath, err = findDiskByLun(int(lun), io, exec); err != nil {
			return false, fmt.Errorf("azureDisk - WaitForAttach ticker failed node (%s) disk (%s) lun(%v) err(%s)", nodeName, diskName, lun, err)
		}

		// did we find it?
		if newDevicePath != "" {
			return true, nil
		}

		// wait until timeout
		return false, nil
	})
	if err == nil && newDevicePath == "" {
		err = fmt.Errorf("azureDisk - WaitForAttach failed within timeout node (%s) diskId:(%s) lun:(%v)", nodeName, diskName, lun)
	}

	return newDevicePath, err
}

// to avoid name conflicts (similar *.vhd name)
// we use hash diskUri and we use it as device mount target.
// this is generalized for both managed and blob disks
// we also prefix the hash with m/b based on disk kind
func (a *azureDiskAttacher) GetDeviceMountPath(spec *volume.Spec) (string, error) {
	volumeSource, _, err := getVolumeSource(spec)
	if err != nil {
		return "", err
	}

	if volumeSource.Kind == nil { // this spec was constructed from info on the node
		pdPath := filepath.Join(a.plugin.host.GetPluginDir(azureDataDiskPluginName), util.MountsInGlobalPDPath, volumeSource.DataDiskURI)
		return pdPath, nil
	}

	isManagedDisk := (*volumeSource.Kind == v1.AzureManagedDisk)
	return makeGlobalPDPath(a.plugin.host, volumeSource.DataDiskURI, isManagedDisk)
}

func (a *azureDiskAttacher) MountDevice(spec *volume.Spec, devicePath string, deviceMountPath string, _ volume.DeviceMounterArgs) error {
	mounter := a.plugin.host.GetMounter(azureDataDiskPluginName)
	notMnt, err := mounter.IsLikelyNotMountPoint(deviceMountPath)

	if err != nil {
		if os.IsNotExist(err) {
			dir := deviceMountPath
			if runtime.GOOS == "windows" {
				// in windows, as we use mklink, only need to MkdirAll for parent directory
				dir = filepath.Dir(deviceMountPath)
			}
			if err := os.MkdirAll(dir, 0750); err != nil {
				return fmt.Errorf("azureDisk - mountDevice:CreateDirectory failed with %s", err)
			}
			notMnt = true
		} else {
			return fmt.Errorf("azureDisk - mountDevice:IsLikelyNotMountPoint failed with %s", err)
		}
	}

	if !notMnt {
		// testing original mount point, make sure the mount link is valid
		if _, err := (&osIOHandler{}).ReadDir(deviceMountPath); err != nil {
			// mount link is invalid, now unmount and remount later
			klog.Warningf("azureDisk - ReadDir %s failed with %v, unmount this directory", deviceMountPath, err)
			if err := mounter.Unmount(deviceMountPath); err != nil {
				klog.Errorf("azureDisk - Unmount deviceMountPath %s failed with %v", deviceMountPath, err)
				return err
			}
			notMnt = true
		}
	}

	volumeSource, _, err := getVolumeSource(spec)
	if err != nil {
		return err
	}

	options := []string{}
	if notMnt {
		diskMounter := util.NewSafeFormatAndMountFromHost(azureDataDiskPluginName, a.plugin.host)
		mountOptions := util.MountOptionFromSpec(spec, options...)
		if runtime.GOOS == "windows" {
			// only parse devicePath on Windows node
			diskNum, err := getDiskNum(devicePath)
			if err != nil {
				return err
			}
			devicePath = diskNum
		}
		err = diskMounter.FormatAndMount(devicePath, deviceMountPath, *volumeSource.FSType, mountOptions)
		if err != nil {
			if cleanErr := os.Remove(deviceMountPath); cleanErr != nil {
				return fmt.Errorf("azureDisk - mountDevice:FormatAndMount failed with %s and clean up failed with :%v", err, cleanErr)
			}
			return fmt.Errorf("azureDisk - mountDevice:FormatAndMount failed with %s", err)
		}
	}
	return nil
}

// Detach detaches disk from Azure VM.
func (d *azureDiskDetacher) Detach(diskURI string, nodeName types.NodeName) error {
	if diskURI == "" {
		return fmt.Errorf("invalid disk to detach: %q", diskURI)
	}

	diskController, err := getDiskController(d.plugin.host)
	if err != nil {
		return err
	}

	err = diskController.DetachDisk("", diskURI, nodeName)
	if err != nil {
		klog.Errorf("failed to detach azure disk %q, err %v", diskURI, err)
	}

	klog.V(2).Infof("azureDisk - disk:%s was detached from node:%v", diskURI, nodeName)
	return err
}

// UnmountDevice unmounts the volume on the node
func (d *azureDiskDetacher) UnmountDevice(deviceMountPath string) error {
	if runtime.GOOS == "windows" {
		// Flush data cache for windows because it does not do so automatically during unmount device
		exec := d.plugin.host.GetExec(d.plugin.GetPluginName())
		err := util.WriteVolumeCache(deviceMountPath, exec)
		if err != nil {
			return err
		}
	}

	err := mount.CleanupMountPoint(deviceMountPath, d.plugin.host.GetMounter(d.plugin.GetPluginName()), false)
	if err == nil {
		klog.V(2).Infof("azureDisk - Device %s was unmounted", deviceMountPath)
	} else {
		klog.Warningf("azureDisk - Device %s failed to unmount with error: %s", deviceMountPath, err.Error())
	}
	return err
}
