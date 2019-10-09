// +build linux

/*
   Copyright The containerd Authors.

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

package v1

import "github.com/containerd/cgroups"

type (
	// Metrics alias
	Metrics = cgroups.Metrics
	// BlkIOEntry alias
	BlkIOEntry = cgroups.BlkIOEntry
	// MemoryStat alias
	MemoryStat = cgroups.MemoryStat
	// CPUStat alias
	CPUStat = cgroups.CPUStat
	// CPUUsage alias
	CPUUsage = cgroups.CPUUsage
	// BlkIOStat alias
	BlkIOStat = cgroups.BlkIOStat
	// PidsStat alias
	PidsStat = cgroups.PidsStat
	// RdmaStat alias
	RdmaStat = cgroups.RdmaStat
	// RdmaEntry alias
	RdmaEntry = cgroups.RdmaEntry
	// HugetlbStat alias
	HugetlbStat = cgroups.HugetlbStat
)
