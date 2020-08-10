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

package opts

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/runtime/linux/runctypes"
)

// WithContainerdShimCgroup returns function that sets the containerd
// shim cgroup path
func WithContainerdShimCgroup(path string) containerd.NewTaskOpts {
	return func(_ context.Context, _ *containerd.Client, r *containerd.TaskInfo) error {
		r.Options = &runctypes.CreateOptions{
			ShimCgroup: path,
		}
		return nil
	}
}

//TODO: Since Options is an interface different WithXXX will be needed to set different
// combinations of CreateOptions.
