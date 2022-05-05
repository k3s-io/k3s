//go:build ctrd
// +build ctrd

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

package containerd

import (
	_ "github.com/containerd/containerd/metrics/cgroups"
	_ "github.com/containerd/containerd/runtime/v2"
	_ "github.com/containerd/containerd/runtime/v2/runc/options"
	_ "github.com/containerd/containerd/snapshots/btrfs/plugin"
	_ "github.com/containerd/containerd/snapshots/native/plugin"
	_ "github.com/containerd/containerd/snapshots/overlay/plugin"
	_ "github.com/containerd/fuse-overlayfs-snapshotter/plugin"
	_ "github.com/containerd/stargz-snapshotter/service/plugin"
)
