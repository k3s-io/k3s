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

package features

import (
	apiextensionsfeatures "k8s.io/apiextensions-apiserver/pkg/features"
	genericfeatures "k8s.io/apiserver/pkg/features"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

const (
	// Every feature gate should add method here following this template:
	//
	// // owner: @username
	// // alpha: v1.X
	// MyFeature utilfeature.Feature = "MyFeature"

	// owner: @tallclair
	// beta: v1.4
	AppArmor utilfeature.Feature = "AppArmor"

	// owner: @jiayingz
	// beta: v1.10
	//
	// Enables support for Device Plugins
	DevicePlugins utilfeature.Feature = "DevicePlugins"

	// owner: @msau42
	// alpha: v1.7
	//
	// A new volume type that supports local disks on a node.
	PersistentLocalVolumes utilfeature.Feature = "PersistentLocalVolumes"

	// owner: @jinxu
	// beta: v1.10
	//
	// New local storage types to support local storage capacity isolation
	LocalStorageCapacityIsolation utilfeature.Feature = "LocalStorageCapacityIsolation"

	// owner: @gnufied
	// alpha: v1.8
	// Ability to Expand persistent volumes
	ExpandPersistentVolumes utilfeature.Feature = "ExpandPersistentVolumes"

	// owner: @jsafrane
	// beta: v1.10
	//
	// Enable mount propagation of volumes.
	MountPropagation utilfeature.Feature = "MountPropagation"

	// owner: @ConnorDoyle
	// alpha: v1.8
	//
	// Alternative container-level CPU affinity policies.
	CPUManager utilfeature.Feature = "CPUManager"

	// owner: @derekwaynecarr
	// beta: v1.10
	//
	// Enable pods to consume pre-allocated huge pages of varying page sizes
	HugePages utilfeature.Feature = "HugePages"

	// owner: @msau42
	// alpha: v1.9
	//
	// Extend the default scheduler to be aware of PV topology and handle PV binding
	// Before moving to beta, resolve Kubernetes issue #56180
	VolumeScheduling utilfeature.Feature = "VolumeScheduling"

	// owner: @vladimirvivien
	// alpha: v1.9
	//
	// Enable mount/attachment of Container Storage Interface (CSI) backed PVs
	CSIPersistentVolume utilfeature.Feature = "CSIPersistentVolume"

	// owner @MrHohn
	// beta: v1.10
	//
	// Support configurable pod DNS parameters.
	CustomPodDNS utilfeature.Feature = "CustomPodDNS"

	// owner: @pospispa
	// beta: v1.10
	//
	// Postpone deletion of a PV or a PVC when they are being used
	StorageObjectInUseProtection utilfeature.Feature = "StorageObjectInUseProtection"

	// owner: @m1093782566
	// beta: v1.9
	//
	// Implement IPVS-based in-cluster service load balancing
	SupportIPVSProxyMode utilfeature.Feature = "SupportIPVSProxyMode"

	// owner: @verult
	// beta: v1.10
	//
	// Enables the regional PD feature on GCE.
	GCERegionalPersistentDisk utilfeature.Feature = "GCERegionalPersistentDisk"

	// owner: @saad-ali
	// ga
	//
	// Allow mounting a subpath of a volume in a container
	// Do not remove this feature gate even though it's GA
	VolumeSubpath utilfeature.Feature = "VolumeSubpath"
)

func init() {
	utilfeature.DefaultFeatureGate.Add(defaultKubernetesFeatureGates)
}

// defaultKubernetesFeatureGates consists of all known Kubernetes-specific feature keys.
// To add a new feature, define a key for it above and add it here. The features will be
// available throughout Kubernetes binaries.
var defaultKubernetesFeatureGates = map[utilfeature.Feature]utilfeature.FeatureSpec{
	AppArmor:                      {Default: true, PreRelease: utilfeature.Beta},
	DevicePlugins:                 {Default: true, PreRelease: utilfeature.Beta},
	PersistentLocalVolumes:        {Default: true, PreRelease: utilfeature.Beta},
	LocalStorageCapacityIsolation: {Default: true, PreRelease: utilfeature.Beta},
	HugePages:                     {Default: true, PreRelease: utilfeature.Beta},
	MountPropagation:              {Default: true, PreRelease: utilfeature.Beta},
	CPUManager:                    {Default: true, PreRelease: utilfeature.Beta},
	VolumeScheduling:              {Default: true, PreRelease: utilfeature.Beta},
	CSIPersistentVolume:           {Default: true, PreRelease: utilfeature.Beta},
	CustomPodDNS:                  {Default: true, PreRelease: utilfeature.Beta},
	StorageObjectInUseProtection:  {Default: true, PreRelease: utilfeature.Beta},
	SupportIPVSProxyMode:          {Default: true, PreRelease: utilfeature.Beta},
	GCERegionalPersistentDisk:     {Default: true, PreRelease: utilfeature.Beta},
	VolumeSubpath:                 {Default: true, PreRelease: utilfeature.GA},

	// inherited features from generic apiserver, relisted here to get a conflict if it is changed
	// unintentionally on either side:
	genericfeatures.StreamingProxyRedirects: {Default: true, PreRelease: utilfeature.Beta},
	genericfeatures.AdvancedAuditing:        {Default: true, PreRelease: utilfeature.Beta},
	genericfeatures.APIListChunking:         {Default: true, PreRelease: utilfeature.Beta},

	// inherited features from apiextensions-apiserver, relisted here to get a conflict if it is changed
	// unintentionally on either side:
	apiextensionsfeatures.CustomResourceValidation: {Default: true, PreRelease: utilfeature.Beta},
}
