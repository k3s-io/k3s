/*
Copyright 2021 The Kubernetes Authors.

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

package names

const (
	PrioritySort                    = "PrioritySort"
	DefaultBinder                   = "DefaultBinder"
	DefaultPreemption               = "DefaultPreemption"
	ImageLocality                   = "ImageLocality"
	InterPodAffinity                = "InterPodAffinity"
	NodeAffinity                    = "NodeAffinity"
	NodeLabel                       = "NodeLabel"
	NodeName                        = "NodeName"
	NodePorts                       = "NodePorts"
	NodePreferAvoidPods             = "NodePreferAvoidPods"
	NodeResourcesBalancedAllocation = "NodeResourcesBalancedAllocation"
	NodeResourcesFit                = "NodeResourcesFit"
	NodeResourcesLeastAllocated     = "NodeResourcesLeastAllocated"
	NodeResourcesMostAllocated      = "NodeResourcesMostAllocated"
	RequestedToCapacityRatio        = "RequestedToCapacityRatio"
	NodeUnschedulable               = "NodeUnschedulable"
	NodeVolumeLimits                = "NodeVolumeLimits"
	AzureDiskLimits                 = "AzureDiskLimits"
	CinderLimits                    = "CinderLimits"
	EBSLimits                       = "EBSLimits"
	GCEPDLimits                     = "GCEPDLimits"
	PodTopologySpread               = "PodTopologySpread"
	SelectorSpread                  = "SelectorSpread"
	ServiceAffinity                 = "ServiceAffinity"
	TaintToleration                 = "TaintToleration"
	VolumeBinding                   = "VolumeBinding"
	VolumeRestrictions              = "VolumeRestrictions"
	VolumeZone                      = "VolumeZone"
)
