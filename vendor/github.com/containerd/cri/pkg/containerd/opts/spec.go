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
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/oci"

	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	runtimespec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

// DefaultSandboxCPUshares is default cpu shares for sandbox container.
// TODO(windows): Revisit cpu shares for windows (https://github.com/containerd/cri/issues/1297)
const DefaultSandboxCPUshares = 2

// WithRelativeRoot sets the root for the container
func WithRelativeRoot(root string) oci.SpecOpts {
	return func(ctx context.Context, client oci.Client, c *containers.Container, s *runtimespec.Spec) (err error) {
		if s.Root == nil {
			s.Root = &runtimespec.Root{}
		}
		s.Root.Path = root
		return nil
	}
}

// WithoutRoot sets the root to nil for the container.
func WithoutRoot(ctx context.Context, client oci.Client, c *containers.Container, s *runtimespec.Spec) error {
	s.Root = nil
	return nil
}

// WithProcessArgs sets the process args on the spec based on the image and runtime config
func WithProcessArgs(config *runtime.ContainerConfig, image *imagespec.ImageConfig) oci.SpecOpts {
	return func(ctx context.Context, client oci.Client, c *containers.Container, s *runtimespec.Spec) (err error) {
		command, args := config.GetCommand(), config.GetArgs()
		// The following logic is migrated from https://github.com/moby/moby/blob/master/daemon/commit.go
		// TODO(random-liu): Clearly define the commands overwrite behavior.
		if len(command) == 0 {
			// Copy array to avoid data race.
			if len(args) == 0 {
				args = append([]string{}, image.Cmd...)
			}
			if command == nil {
				command = append([]string{}, image.Entrypoint...)
			}
		}
		if len(command) == 0 && len(args) == 0 {
			return errors.New("no command specified")
		}
		return oci.WithProcessArgs(append(command, args...)...)(ctx, client, c, s)
	}
}

// mounts defines how to sort runtime.Mount.
// This is the same with the Docker implementation:
//   https://github.com/moby/moby/blob/17.05.x/daemon/volumes.go#L26
type orderedMounts []*runtime.Mount

// Len returns the number of mounts. Used in sorting.
func (m orderedMounts) Len() int {
	return len(m)
}

// Less returns true if the number of parts (a/b/c would be 3 parts) in the
// mount indexed by parameter 1 is less than that of the mount indexed by
// parameter 2. Used in sorting.
func (m orderedMounts) Less(i, j int) bool {
	return m.parts(i) < m.parts(j)
}

// Swap swaps two items in an array of mounts. Used in sorting
func (m orderedMounts) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

// parts returns the number of parts in the destination of a mount. Used in sorting.
func (m orderedMounts) parts(i int) int {
	return strings.Count(filepath.Clean(m[i].ContainerPath), string(os.PathSeparator))
}

// WithAnnotation sets the provided annotation
func WithAnnotation(k, v string) oci.SpecOpts {
	return func(ctx context.Context, client oci.Client, c *containers.Container, s *runtimespec.Spec) error {
		if s.Annotations == nil {
			s.Annotations = make(map[string]string)
		}
		s.Annotations[k] = v
		return nil
	}
}
