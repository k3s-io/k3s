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

package server

import (
	"github.com/containerd/cgroups"
	tasks "github.com/containerd/containerd/api/services/tasks/v1"
	"github.com/containerd/containerd/api/types"
	"github.com/containerd/typeurl"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	runtime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

	containerstore "github.com/containerd/cri/pkg/store/container"
)

// ListContainerStats returns stats of all running containers.
func (c *criService) ListContainerStats(
	ctx context.Context,
	in *runtime.ListContainerStatsRequest,
) (*runtime.ListContainerStatsResponse, error) {
	request, containers, err := c.buildTaskMetricsRequest(in)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build metrics request")
	}
	resp, err := c.client.TaskService().Metrics(ctx, &request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch metrics for tasks")
	}
	criStats, err := c.toCRIContainerStats(resp.Metrics, containers)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert to cri containerd stats format")
	}
	return criStats, nil
}

func (c *criService) toCRIContainerStats(
	stats []*types.Metric,
	containers []containerstore.Container,
) (*runtime.ListContainerStatsResponse, error) {
	statsMap := make(map[string]*types.Metric)
	for _, stat := range stats {
		statsMap[stat.ID] = stat
	}
	containerStats := new(runtime.ListContainerStatsResponse)
	for _, cntr := range containers {
		cs, err := c.getContainerMetrics(cntr.Metadata, statsMap[cntr.ID])
		if err != nil {
			return nil, errors.Wrapf(err, "failed to decode container metrics for %q", cntr.ID)
		}
		containerStats.Stats = append(containerStats.Stats, cs)
	}
	return containerStats, nil
}

func (c *criService) getContainerMetrics(
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
		metrics := s.(*cgroups.Metrics)
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
	}

	return &cs, nil
}

func (c *criService) normalizeContainerStatsFilter(filter *runtime.ContainerStatsFilter) {
	if cntr, err := c.containerStore.Get(filter.GetId()); err == nil {
		filter.Id = cntr.ID
	}
	if sb, err := c.sandboxStore.Get(filter.GetPodSandboxId()); err == nil {
		filter.PodSandboxId = sb.ID
	}
}

// buildTaskMetricsRequest constructs a tasks.MetricsRequest based on
// the information in the stats request and the containerStore
func (c *criService) buildTaskMetricsRequest(
	r *runtime.ListContainerStatsRequest,
) (tasks.MetricsRequest, []containerstore.Container, error) {
	var req tasks.MetricsRequest
	if r.GetFilter() == nil {
		return req, nil, nil
	}
	c.normalizeContainerStatsFilter(r.GetFilter())
	var containers []containerstore.Container
	for _, cntr := range c.containerStore.List() {
		if r.GetFilter().GetId() != "" && cntr.ID != r.GetFilter().GetId() {
			continue
		}
		if r.GetFilter().GetPodSandboxId() != "" && cntr.SandboxID != r.GetFilter().GetPodSandboxId() {
			continue
		}
		if r.GetFilter().GetLabelSelector() != nil &&
			!matchLabelSelector(r.GetFilter().GetLabelSelector(), cntr.Config.GetLabels()) {
			continue
		}
		containers = append(containers, cntr)
		req.Filters = append(req.Filters, "id=="+cntr.ID)
	}
	return req, containers, nil
}

func matchLabelSelector(selector, labels map[string]string) bool {
	for k, v := range selector {
		if val, ok := labels[k]; ok {
			if v != val {
				return false
			}
		} else {
			return false
		}
	}
	return true
}

// getWorkingSet calculates workingset memory from cgroup memory stats.
// The caller should make sure memory is not nil.
// workingset = usage - total_inactive_file
func getWorkingSet(memory *cgroups.MemoryStat) uint64 {
	if memory.Usage == nil {
		return 0
	}
	var workingSet uint64
	if memory.TotalInactiveFile < memory.Usage.Usage {
		workingSet = memory.Usage.Usage - memory.TotalInactiveFile
	}
	return workingSet
}
