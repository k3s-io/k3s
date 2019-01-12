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
	tasks "github.com/containerd/containerd/api/services/tasks/v1"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	runtime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// ContainerStats returns stats of the container. If the container does not
// exist, the call returns an error.
func (c *criService) ContainerStats(ctx context.Context, in *runtime.ContainerStatsRequest) (*runtime.ContainerStatsResponse, error) {
	cntr, err := c.containerStore.Get(in.GetContainerId())
	if err != nil {
		return nil, errors.Wrap(err, "failed to find container")
	}
	request := &tasks.MetricsRequest{Filters: []string{"id==" + cntr.ID}}
	resp, err := c.client.TaskService().Metrics(ctx, request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch metrics for task")
	}
	if len(resp.Metrics) != 1 {
		return nil, errors.Errorf("unexpected metrics response: %+v", resp.Metrics)
	}

	cs, err := c.getContainerMetrics(cntr.Metadata, resp.Metrics[0])
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode container metrics")
	}
	return &runtime.ContainerStatsResponse{Stats: cs}, nil
}
