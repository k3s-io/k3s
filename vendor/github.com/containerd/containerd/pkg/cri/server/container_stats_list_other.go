// +build !windows,!linux

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
	"github.com/containerd/containerd/api/types"
	"github.com/containerd/containerd/errdefs"
	"github.com/pkg/errors"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	containerstore "github.com/containerd/containerd/pkg/cri/store/container"
)

func (c *criService) containerMetrics(
	meta containerstore.Metadata,
	stats *types.Metric,
) (*runtime.ContainerStats, error) {
	var cs runtime.ContainerStats
	return &cs, errors.Wrap(errdefs.ErrNotImplemented, "container metrics")
}
