// +build !windows

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

package server

import (
	"fmt"

	"github.com/containerd/containerd/api/types"
	v1 "github.com/containerd/containerd/metrics/types/v1"
	v2 "github.com/containerd/containerd/metrics/types/v2"
	"github.com/containerd/typeurl"
	"github.com/pkg/errors"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	containerstore "github.com/containerd/cri/pkg/store/container"
)

func (c *criService) containerMetrics(
	meta containerstore.Metadata,
	stats *types.Metric,
) (*runtime.ContainerStats, error) {
	var cs runtime.ContainerStats
	var usedBytes, inodesUsed uint64
	sn, err := c.snapshotStore.Get(meta.ID)
	// If snapshotstore doesn't have cached snapshot information
	// set WritableLayer usage to zero
	if err == nil {
		usedBytes = sn.Size
		inodesUsed = sn.Inodes
	}
	cs.WritableLayer = &runtime.FilesystemUsage{
		Timestamp: sn.Timestamp,
		FsId: &runtime.FilesystemIdentifier{
			Mountpoint: c.imageFSPath,
		},
		UsedBytes:  &runtime.UInt64Value{Value: usedBytes},
		InodesUsed: &runtime.UInt64Value{Value: inodesUsed},
	}
	cs.Attributes = &runtime.ContainerAttributes{
		Id:          meta.ID,
		Metadata:    meta.Config.GetMetadata(),
		Labels:      meta.Config.GetLabels(),
		Annotations: meta.Config.GetAnnotations(),
	}

	if stats != nil {
		s, err := typeurl.UnmarshalAny(stats.Data)
		if err != nil {
			return nil, errors.Wrap(err, "failed to extract container metrics")
		}
		switch metrics := s.(type) {
		case *v1.Metrics:
			if metrics.CPU != nil && metrics.CPU.Usage != nil {
				cs.Cpu = &runtime.CpuUsage{
					Timestamp:            stats.Timestamp.UnixNano(),
					UsageCoreNanoSeconds: &runtime.UInt64Value{Value: metrics.CPU.Usage.Total},
				}
			}
			if metrics.Memory != nil && metrics.Memory.Usage != nil {
				cs.Memory = &runtime.MemoryUsage{
					Timestamp: stats.Timestamp.UnixNano(),
					WorkingSetBytes: &runtime.UInt64Value{
						Value: getWorkingSet(metrics.Memory),
					},
				}
			}
		case *v2.Metrics:
			if metrics.CPU != nil {
				cs.Cpu = &runtime.CpuUsage{
					Timestamp:            stats.Timestamp.UnixNano(),
					UsageCoreNanoSeconds: &runtime.UInt64Value{Value: metrics.CPU.UsageUsec * 1000},
				}
			}
			if metrics.Memory != nil {
				cs.Memory = &runtime.MemoryUsage{
					Timestamp: stats.Timestamp.UnixNano(),
					WorkingSetBytes: &runtime.UInt64Value{
						Value: getWorkingSetV2(metrics.Memory),
					},
				}
			}
		default:
			return &cs, errors.New(fmt.Sprintf("unxpected metrics type: %v", metrics))
		}
	}

	return &cs, nil
}

// getWorkingSet calculates workingset memory from cgroup memory stats.
// The caller should make sure memory is not nil.
// workingset = usage - total_inactive_file
func getWorkingSet(memory *v1.MemoryStat) uint64 {
	if memory.Usage == nil {
		return 0
	}
	var workingSet uint64
	if memory.TotalInactiveFile < memory.Usage.Usage {
		workingSet = memory.Usage.Usage - memory.TotalInactiveFile
	}
	return workingSet
}

// getWorkingSetV2 calculates workingset memory from cgroupv2 memory stats.
// The caller should make sure memory is not nil.
// workingset = usage - inactive_file
func getWorkingSetV2(memory *v2.MemoryStat) uint64 {
	var workingSet uint64
	if memory.InactiveFile < memory.Usage {
		workingSet = memory.Usage - memory.InactiveFile
	}
	return workingSet
}
