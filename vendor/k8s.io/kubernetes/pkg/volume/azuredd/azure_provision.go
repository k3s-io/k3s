// +build !providerless

/*
Copyright 2017 The Kubernetes Authors.

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
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2019-06-01/storage"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	volumehelpers "k8s.io/cloud-provider/volume/helpers"
	"k8s.io/kubernetes/pkg/volume"
	"k8s.io/kubernetes/pkg/volume/util"
	"k8s.io/legacy-cloud-providers/azure"
)

type azureDiskProvisioner struct {
	plugin  *azureDataDiskPlugin
	options volume.VolumeOptions
}

type azureDiskDeleter struct {
	*dataDisk
	spec   *volume.Spec
	plugin *azureDataDiskPlugin
}

var _ volume.Provisioner = &azureDiskProvisioner{}
var _ volume.Deleter = &azureDiskDeleter{}

func (d *azureDiskDeleter) GetPath() string {
	return getPath(d.podUID, d.dataDisk.diskName, d.plugin.host)
}

func (d *azureDiskDeleter) Delete() error {
	volumeSource, _, err := getVolumeSource(d.spec)
	if err != nil {
		return err
	}

	diskController, err := getDiskController(d.plugin.host)
	if err != nil {
		return err
	}

	managed := (*volumeSource.Kind == v1.AzureManagedDisk)

	if managed {
		return diskController.DeleteManagedDisk(volumeSource.DataDiskURI)
	}

	return diskController.DeleteBlobDisk(volumeSource.DataDiskURI)
}

// parseZoned parsed 'zoned' for storage class. If zoned is not specified (empty string),
// then it defaults to true for managed disks.
func parseZoned(zonedString string, kind v1.AzureDataDiskKind) (bool, error) {
	if zonedString == "" {
		return kind == v1.AzureManagedDisk, nil
	}

	zoned, err := strconv.ParseBool(zonedString)
	if err != nil {
		return false, fmt.Errorf("failed to parse 'zoned': %v", err)
	}

	if zoned && kind != v1.AzureManagedDisk {
		return false, fmt.Errorf("zoned is only supported by managed disks")
	}

	return zoned, nil
}

func (p *azureDiskProvisioner) Provision(selectedNode *v1.Node, allowedTopologies []v1.TopologySelectorTerm) (*v1.PersistentVolume, error) {
	// perform static validation first
	if p.options.PVC.Spec.Selector != nil {
		return nil, fmt.Errorf("azureDisk - claim.Spec.Selector is not supported for dynamic provisioning on Azure disk")
	}

	var (
		location, account          string
		storageAccountType, fsType string
		cachingMode                v1.AzureDataDiskCachingMode
		strKind                    string
		err                        error
		resourceGroup              string

		zoned                    bool
		zonePresent              bool
		zonesPresent             bool
		strZoned                 string
		availabilityZone         string
		availabilityZones        sets.String
		selectedAvailabilityZone string
		writeAcceleratorEnabled  string

		diskIopsReadWrite   string
		diskMbpsReadWrite   string
		diskEncryptionSetID string
		customTags          string

		maxShares int
	)
	// maxLength = 79 - (4 for ".vhd") = 75
	name := util.GenerateVolumeName(p.options.ClusterName, p.options.PVName, 75)
	capacity := p.options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	requestGiB, err := volumehelpers.RoundUpToGiBInt(capacity)
	if err != nil {
		return nil, err
	}

	for k, v := range p.options.Parameters {
		switch strings.ToLower(k) {
		case "skuname":
			storageAccountType = v
		case "location":
			location = v
		case "storageaccount":
			account = v
		case "storageaccounttype":
			storageAccountType = v
		case "kind":
			strKind = v
		case "cachingmode":
			cachingMode = v1.AzureDataDiskCachingMode(v)
		case volume.VolumeParameterFSType:
			fsType = strings.ToLower(v)
		case "resourcegroup":
			resourceGroup = v
		case "zone":
			zonePresent = true
			availabilityZone = v
		case "zones":
			zonesPresent = true
			availabilityZones, err = volumehelpers.ZonesToSet(v)
			if err != nil {
				return nil, fmt.Errorf("error parsing zones %s, must be strings separated by commas: %v", v, err)
			}
		case "zoned":
			strZoned = v
		case "diskiopsreadwrite":
			diskIopsReadWrite = v
		case "diskmbpsreadwrite":
			diskMbpsReadWrite = v
		case "diskencryptionsetid":
			diskEncryptionSetID = v
		case "tags":
			customTags = v
		case azure.WriteAcceleratorEnabled:
			writeAcceleratorEnabled = v
		case "maxshares":
			maxShares, err = strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("parse %s failed with error: %v", v, err)
			}
			if maxShares < 1 {
				return nil, fmt.Errorf("parse %s returned with invalid value: %d", v, maxShares)
			}
		default:
			return nil, fmt.Errorf("AzureDisk - invalid option %s in storage class", k)
		}
	}

	supportedModes := p.plugin.GetAccessModes()
	if maxShares < 2 {
		// only do AccessModes validation when maxShares < 2
		if !util.ContainsAllAccessModes(p.plugin.GetAccessModes(), p.options.PVC.Spec.AccessModes) {
			return nil, fmt.Errorf("invalid AccessModes %v: only AccessModes %v are supported with maxShares(%d) < 2", p.options.PVC.Spec.AccessModes, p.plugin.GetAccessModes(), maxShares)
		}

		if len(p.options.PVC.Spec.AccessModes) > 1 {
			return nil, fmt.Errorf("AzureDisk - multiple access modes are not supported on AzureDisk plugin with maxShares(%d) < 2", maxShares)
		}

		if len(p.options.PVC.Spec.AccessModes) == 1 {
			if p.options.PVC.Spec.AccessModes[0] != supportedModes[0] {
				return nil, fmt.Errorf("AzureDisk - mode %s is not supported by AzureDisk plugin (supported mode is %s) with maxShares(%d) < 2", p.options.PVC.Spec.AccessModes[0], supportedModes, maxShares)
			}
		}
	} else {
		supportedModes = []v1.PersistentVolumeAccessMode{
			v1.ReadWriteOnce,
			v1.ReadOnlyMany,
			v1.ReadWriteMany,
		}
	}

	// normalize values
	skuName, err := normalizeStorageAccountType(storageAccountType)
	if err != nil {
		return nil, err
	}

	kind, err := normalizeKind(strFirstLetterToUpper(strKind))
	if err != nil {
		return nil, err
	}

	zoned, err = parseZoned(strZoned, kind)
	if err != nil {
		return nil, err
	}

	if kind != v1.AzureManagedDisk {
		if resourceGroup != "" {
			return nil, errors.New("StorageClass option 'resourceGroup' can be used only for managed disks")
		}

		if zoned {
			return nil, errors.New("StorageClass option 'zoned' parameter is only supported for managed disks")
		}
	}

	if !zoned && (zonePresent || zonesPresent || len(allowedTopologies) > 0) {
		return nil, fmt.Errorf("zone, zones and allowedTopologies StorageClass parameters must be used together with zoned parameter")
	}

	if cachingMode, err = normalizeCachingMode(cachingMode); err != nil {
		return nil, err
	}

	diskController, err := getDiskController(p.plugin.host)
	if err != nil {
		return nil, err
	}

	// Select zone for managed disks based on zone, zones and allowedTopologies.
	if zoned {
		activeZones, err := diskController.GetActiveZones()
		if err != nil {
			return nil, fmt.Errorf("error querying active zones: %v", err)
		}

		if availabilityZone != "" || availabilityZones.Len() != 0 || activeZones.Len() != 0 || len(allowedTopologies) != 0 {
			selectedAvailabilityZone, err = volumehelpers.SelectZoneForVolume(zonePresent, zonesPresent, availabilityZone, availabilityZones, activeZones, selectedNode, allowedTopologies, p.options.PVC.Name)
			if err != nil {
				return nil, err
			}
		}
	}

	// create disk
	diskURI := ""
	labels := map[string]string{}
	if kind == v1.AzureManagedDisk {
		tags, err := azure.ConvertTagsToMap(customTags)
		if err != nil {
			return nil, err
		}
		if p.options.CloudTags != nil {
			for k, v := range *(p.options.CloudTags) {
				tags[k] = v
			}
		}
		if strings.EqualFold(writeAcceleratorEnabled, "true") {
			tags[azure.WriteAcceleratorEnabled] = "true"
		}

		volumeOptions := &azure.ManagedDiskOptions{
			DiskName:            name,
			StorageAccountType:  skuName,
			ResourceGroup:       resourceGroup,
			PVCName:             p.options.PVC.Name,
			SizeGB:              requestGiB,
			Tags:                tags,
			AvailabilityZone:    selectedAvailabilityZone,
			DiskIOPSReadWrite:   diskIopsReadWrite,
			DiskMBpsReadWrite:   diskMbpsReadWrite,
			DiskEncryptionSetID: diskEncryptionSetID,
			MaxShares:           int32(maxShares),
		}
		diskURI, err = diskController.CreateManagedDisk(volumeOptions)
		if err != nil {
			return nil, err
		}
		labels, err = diskController.GetAzureDiskLabels(diskURI)
		if err != nil {
			return nil, err
		}
	} else { // Attention: blob disk feature is deprecated
		if kind == v1.AzureDedicatedBlobDisk {
			_, diskURI, _, err = diskController.CreateVolume(name, account, storageAccountType, location, requestGiB)
			if err != nil {
				return nil, err
			}
		} else {
			diskURI, err = diskController.CreateBlobDisk(name, storage.SkuName(skuName), requestGiB)
			if err != nil {
				return nil, err
			}
		}
	}

	volumeMode := p.options.PVC.Spec.VolumeMode
	if volumeMode != nil && *volumeMode == v1.PersistentVolumeBlock {
		// Block volumes should not have any FSType
		fsType = ""
	}

	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:   p.options.PVName,
			Labels: labels,
			Annotations: map[string]string{
				"volumehelper.VolumeDynamicallyCreatedByKey": "azure-disk-dynamic-provisioner",
			},
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: p.options.PersistentVolumeReclaimPolicy,
			AccessModes:                   supportedModes,
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): resource.MustParse(fmt.Sprintf("%dGi", requestGiB)),
			},
			VolumeMode: volumeMode,
			PersistentVolumeSource: v1.PersistentVolumeSource{
				AzureDisk: &v1.AzureDiskVolumeSource{
					CachingMode: &cachingMode,
					DiskName:    name,
					DataDiskURI: diskURI,
					Kind:        &kind,
					FSType:      &fsType,
				},
			},
			MountOptions: p.options.MountOptions,
		},
	}

	nodeSelectorTerms := make([]v1.NodeSelectorTerm, 0)

	if zoned {
		// Set node affinity labels based on availability zone labels.
		if len(labels) > 0 {
			requirements := make([]v1.NodeSelectorRequirement, 0)
			for k, v := range labels {
				requirements = append(requirements, v1.NodeSelectorRequirement{Key: k, Operator: v1.NodeSelectorOpIn, Values: []string{v}})
			}

			nodeSelectorTerms = append(nodeSelectorTerms, v1.NodeSelectorTerm{
				MatchExpressions: requirements,
			})
		}
	} else {
		// Set node affinity labels based on topology.
		// This is required because unzoned AzureDisk can't be attached to zoned nodes.
		// There are at most 3 Availability Zones per supported Azure region.
		// Refer https://docs.microsoft.com/en-us/azure/virtual-machines/windows/manage-availability.
		for i := 0; i < 3; i++ {
			requirements := []v1.NodeSelectorRequirement{
				{
					Key:      v1.LabelTopologyRegion,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{diskController.GetLocation()},
				},
				{
					Key:      v1.LabelTopologyZone,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{strconv.Itoa(i)},
				},
			}
			nodeSelectorTerms = append(nodeSelectorTerms, v1.NodeSelectorTerm{
				MatchExpressions: requirements,
			})
		}
	}

	if len(nodeSelectorTerms) > 0 {
		pv.Spec.NodeAffinity = &v1.VolumeNodeAffinity{
			Required: &v1.NodeSelector{
				NodeSelectorTerms: nodeSelectorTerms,
			},
		}
	}

	return pv, nil
}
