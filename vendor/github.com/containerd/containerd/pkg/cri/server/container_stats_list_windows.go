// +build windows

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
	wstats "github.com/Microsoft/hcsshim/cmd/containerd-shim-runhcs-v1/stats"
	"github.com/containerd/containerd/api/types"
	"github.com/containerd/typeurl"
	"github.com/pkg/errors"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	containerstore "github.com/containerd/containerd/pkg/cri/store/container"
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
		wstats := s.(*wstats.Statistics).GetWindows()
		if wstats == nil {
			return nil, errors.New("windows stats is empty")
		}
		if wstats.Processor != nil {
			cs.Cpu = &runtime.CpuUsage{
				Timestamp:            wstats.Timestamp.UnixNano(),
				UsageCoreNanoSeconds: &runtime.UInt64Value{Value: wstats.Processor.TotalRuntimeNS},
			}
		}
		if wstats.Memory != nil {
			cs.Memory = &runtime.MemoryUsage{
				Timestamp: wstats.Timestamp.UnixNano(),
				WorkingSetBytes: &runtime.UInt64Value{
					Value: wstats.Memory.MemoryUsagePrivateWorkingSetBytes,
				},
			}
		}
	}
	return &cs, nil
}
