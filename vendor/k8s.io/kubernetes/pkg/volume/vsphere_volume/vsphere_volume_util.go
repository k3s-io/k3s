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

package vsphere_volume

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
	volumehelpers "k8s.io/cloud-provider/volume/helpers"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/volume"
	volumeutil "k8s.io/kubernetes/pkg/volume/util"
	"k8s.io/legacy-cloud-providers/vsphere"
	"k8s.io/legacy-cloud-providers/vsphere/vclib"
)

const (
	checkSleepDuration = time.Second
	diskByIDPath       = "/dev/disk/by-id/"
	diskSCSIPrefix     = "wwn-0x"
	// diskformat parameter is deprecated as of Kubernetes v1.21.0
	diskformat        = "diskformat"
	datastore         = "datastore"
	StoragePolicyName = "storagepolicyname"

	// hostfailurestotolerate parameter is deprecated as of Kubernetes v1.19.0
	HostFailuresToTolerateCapability = "hostfailurestotolerate"
	// forceprovisioning parameter is deprecated as of Kubernetes v1.19.0
	ForceProvisioningCapability = "forceprovisioning"
	// cachereservation parameter is deprecated as of Kubernetes v1.19.0
	CacheReservationCapability = "cachereservation"
	// diskstripes parameter is deprecated as of Kubernetes v1.19.0
	DiskStripesCapability = "diskstripes"
	// objectspacereservation parameter is deprecated as of Kubernetes v1.19.0
	ObjectSpaceReservationCapability = "objectspacereservation"
	// iopslimit parameter is deprecated as of Kubernetes v1.19.0
	IopsLimitCapability                 = "iopslimit"
	HostFailuresToTolerateCapabilityMin = 0
	HostFailuresToTolerateCapabilityMax = 3
	ForceProvisioningCapabilityMin      = 0
	ForceProvisioningCapabilityMax      = 1
	CacheReservationCapabilityMin       = 0
	CacheReservationCapabilityMax       = 100
	DiskStripesCapabilityMin            = 1
	DiskStripesCapabilityMax            = 12
	ObjectSpaceReservationCapabilityMin = 0
	ObjectSpaceReservationCapabilityMax = 100
	IopsLimitCapabilityMin              = 0
	// reduce number of characters in vsphere volume name. The reason for setting length smaller than 255 is because typically
	// volume name also becomes part of mount path - /var/lib/kubelet/plugins/kubernetes.io/vsphere-volume/mounts/<name>
	// and systemd has a limit of 256 chars in a unit name - https://github.com/systemd/systemd/pull/14294
	// so if we subtract the kubelet path prefix from 256, we are left with 191 characters.
	// Since datastore name is typically part of volumeName we are choosing a shorter length of 63
	// and leaving room of certain characters being escaped etc.
	// Given that volume name is typically of the form - pvc-0f13e3ad-97f8-41ab-9392-84562ef40d17.vmdk (45 chars),
	// this should still leave plenty of room for clusterName inclusion.
	maxVolumeLength = 63
)

var ErrProbeVolume = errors.New("error scanning attached volumes")

type VsphereDiskUtil struct{}

type VolumeSpec struct {
	Path              string
	Size              int
	Fstype            string
	StoragePolicyID   string
	StoragePolicyName string
	Labels            map[string]string
}

// CreateVolume creates a vSphere volume.
func (util *VsphereDiskUtil) CreateVolume(v *vsphereVolumeProvisioner, selectedNode *v1.Node, selectedZone []string) (volSpec *VolumeSpec, err error) {
	var fstype string
	cloud, err := getCloudProvider(v.plugin.host.GetCloudProvider())
	if err != nil {
		return nil, err
	}

	capacity := v.options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	// vSphere works with KiB, but its minimum allocation unit is 1 MiB
	volSizeMiB, err := volumehelpers.RoundUpToMiBInt(capacity)
	if err != nil {
		return nil, err
	}
	volSizeKiB := volSizeMiB * 1024
	name := volumeutil.GenerateVolumeName(v.options.ClusterName, v.options.PVName, maxVolumeLength)
	volumeOptions := &vclib.VolumeOptions{
		CapacityKB: volSizeKiB,
		Tags:       *v.options.CloudTags,
		Name:       name,
	}

	volumeOptions.Zone = selectedZone
	volumeOptions.SelectedNode = selectedNode
	// Apply Parameters (case-insensitive). We leave validation of
	// the values to the cloud provider.
	for parameter, value := range v.options.Parameters {
		switch strings.ToLower(parameter) {
		case diskformat:
			volumeOptions.DiskFormat = value
		case datastore:
			volumeOptions.Datastore = value
		case volume.VolumeParameterFSType:
			fstype = value
			klog.V(4).Infof("Setting fstype as %q", fstype)
		case StoragePolicyName:
			volumeOptions.StoragePolicyName = value
			klog.V(4).Infof("Setting StoragePolicyName as %q", volumeOptions.StoragePolicyName)
		case HostFailuresToTolerateCapability, ForceProvisioningCapability,
			CacheReservationCapability, DiskStripesCapability,
			ObjectSpaceReservationCapability, IopsLimitCapability:
			capabilityData, err := validateVSANCapability(strings.ToLower(parameter), value)
			if err != nil {
				return nil, err
			}
			volumeOptions.VSANStorageProfileData += capabilityData
		default:
			return nil, fmt.Errorf("invalid option %q for volume plugin %s", parameter, v.plugin.GetPluginName())
		}
	}

	if volumeOptions.VSANStorageProfileData != "" {
		if volumeOptions.StoragePolicyName != "" {
			return nil, fmt.Errorf("cannot specify storage policy capabilities along with storage policy name. Please specify only one")
		}
		volumeOptions.VSANStorageProfileData = "(" + volumeOptions.VSANStorageProfileData + ")"
	}
	klog.V(4).Infof("VSANStorageProfileData in vsphere volume %q", volumeOptions.VSANStorageProfileData)
	// TODO: implement PVC.Selector parsing
	if v.options.PVC.Spec.Selector != nil {
		return nil, fmt.Errorf("claim.Spec.Selector is not supported for dynamic provisioning on vSphere")
	}

	vmDiskPath, err := cloud.CreateVolume(volumeOptions)
	if err != nil {
		return nil, err
	}
	labels, err := cloud.GetVolumeLabels(vmDiskPath)
	if err != nil {
		return nil, err
	}
	volSpec = &VolumeSpec{
		Path:              vmDiskPath,
		Size:              volSizeKiB,
		Fstype:            fstype,
		StoragePolicyName: volumeOptions.StoragePolicyName,
		StoragePolicyID:   volumeOptions.StoragePolicyID,
		Labels:            labels,
	}
	klog.V(2).Infof("Successfully created vsphere volume %s", name)
	return volSpec, nil
}

// DeleteVolume deletes a vSphere volume.
func (util *VsphereDiskUtil) DeleteVolume(vd *vsphereVolumeDeleter) error {
	cloud, err := getCloudProvider(vd.plugin.host.GetCloudProvider())
	if err != nil {
		return err
	}

	if err = cloud.DeleteVolume(vd.volPath); err != nil {
		klog.V(2).Infof("Error deleting vsphere volume %s: %v", vd.volPath, err)
		return err
	}
	klog.V(2).Infof("Successfully deleted vsphere volume %s", vd.volPath)
	return nil
}

func getVolPathfromVolumeName(deviceMountPath string) string {
	// Assumption: No file or folder is named starting with '[' in datastore
	volPath := deviceMountPath[strings.LastIndex(deviceMountPath, "["):]
	// space between datastore and vmdk name in volumePath is encoded as '\040' when returned by GetMountRefs().
	// volumePath eg: "[local] xxx.vmdk" provided to attach/mount
	// replacing \040 with space to match the actual volumePath
	return strings.Replace(volPath, "\\040", " ", -1)
}

func getCloudProvider(cloud cloudprovider.Interface) (*vsphere.VSphere, error) {
	if cloud == nil {
		klog.Errorf("Cloud provider not initialized properly")
		return nil, errors.New("cloud provider not initialized properly")
	}

	vs, ok := cloud.(*vsphere.VSphere)
	if !ok || vs == nil {
		return nil, errors.New("invalid cloud provider: expected vSphere")
	}
	return vs, nil
}

// Validate the capability requirement for the user specified policy attributes.
func validateVSANCapability(capabilityName string, capabilityValue string) (string, error) {
	var capabilityData string
	capabilityIntVal, ok := verifyCapabilityValueIsInteger(capabilityValue)
	if !ok {
		return "", fmt.Errorf("invalid value for %s. The capabilityValue: %s must be a valid integer value", capabilityName, capabilityValue)
	}
	switch strings.ToLower(capabilityName) {
	case HostFailuresToTolerateCapability:
		if capabilityIntVal >= HostFailuresToTolerateCapabilityMin && capabilityIntVal <= HostFailuresToTolerateCapabilityMax {
			capabilityData = " (\"hostFailuresToTolerate\" i" + capabilityValue + ")"
		} else {
			return "", fmt.Errorf(`invalid value for hostFailuresToTolerate.
				The default value is %d, minimum value is %d and maximum value is %d`,
				1, HostFailuresToTolerateCapabilityMin, HostFailuresToTolerateCapabilityMax)
		}
	case ForceProvisioningCapability:
		if capabilityIntVal >= ForceProvisioningCapabilityMin && capabilityIntVal <= ForceProvisioningCapabilityMax {
			capabilityData = " (\"forceProvisioning\" i" + capabilityValue + ")"
		} else {
			return "", fmt.Errorf(`invalid value for forceProvisioning.
				The value can be either %d or %d`,
				ForceProvisioningCapabilityMin, ForceProvisioningCapabilityMax)
		}
	case CacheReservationCapability:
		if capabilityIntVal >= CacheReservationCapabilityMin && capabilityIntVal <= CacheReservationCapabilityMax {
			capabilityData = " (\"cacheReservation\" i" + strconv.Itoa(capabilityIntVal*10000) + ")"
		} else {
			return "", fmt.Errorf(`invalid value for cacheReservation.
				The minimum percentage is %d and maximum percentage is %d`,
				CacheReservationCapabilityMin, CacheReservationCapabilityMax)
		}
	case DiskStripesCapability:
		if capabilityIntVal >= DiskStripesCapabilityMin && capabilityIntVal <= DiskStripesCapabilityMax {
			capabilityData = " (\"stripeWidth\" i" + capabilityValue + ")"
		} else {
			return "", fmt.Errorf(`invalid value for diskStripes.
				The minimum value is %d and maximum value is %d`,
				DiskStripesCapabilityMin, DiskStripesCapabilityMax)
		}
	case ObjectSpaceReservationCapability:
		if capabilityIntVal >= ObjectSpaceReservationCapabilityMin && capabilityIntVal <= ObjectSpaceReservationCapabilityMax {
			capabilityData = " (\"proportionalCapacity\" i" + capabilityValue + ")"
		} else {
			return "", fmt.Errorf(`invalid value for ObjectSpaceReservation.
				The minimum percentage is %d and maximum percentage is %d`,
				ObjectSpaceReservationCapabilityMin, ObjectSpaceReservationCapabilityMax)
		}
	case IopsLimitCapability:
		if capabilityIntVal >= IopsLimitCapabilityMin {
			capabilityData = " (\"iopsLimit\" i" + capabilityValue + ")"
		} else {
			return "", fmt.Errorf(`invalid value for iopsLimit.
				The value should be greater than %d`, IopsLimitCapabilityMin)
		}
	}
	return capabilityData, nil
}

// Verify if the capability value is of type integer.
func verifyCapabilityValueIsInteger(capabilityValue string) (int, bool) {
	i, err := strconv.Atoi(capabilityValue)
	if err != nil {
		return -1, false
	}
	return i, true
}
