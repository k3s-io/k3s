// +build !windows,!freebsd

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

package tasks

import (
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/containerd/runtime"
	"github.com/pkg/errors"
)

var tasksServiceRequires = []plugin.Type{
	plugin.RuntimePlugin,
	plugin.RuntimePluginV2,
	plugin.MetadataPlugin,
	plugin.TaskMonitorPlugin,
}

func loadV1Runtimes(ic *plugin.InitContext) (map[string]runtime.PlatformRuntime, error) {
	rt, err := ic.GetByType(plugin.RuntimePlugin)
	if err != nil {
		return nil, err
	}

	runtimes := make(map[string]runtime.PlatformRuntime)
	for _, rr := range rt {
		ri, err := rr.Instance()
		if err != nil {
			log.G(ic.Context).WithError(err).Warn("could not load runtime instance due to initialization error")
			continue
		}
		r := ri.(runtime.PlatformRuntime)
		runtimes[r.ID()] = r
	}

	if len(runtimes) == 0 {
		return nil, errors.New("no runtimes available to create task service")
	}
	return runtimes, nil
}
