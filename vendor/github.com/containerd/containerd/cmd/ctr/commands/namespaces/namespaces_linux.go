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

package namespaces

import (
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/runtime/opts"
	"github.com/urfave/cli"
)

func deleteOpts(context *cli.Context) []namespaces.DeleteOpts {
	var delOpts []namespaces.DeleteOpts
	if context.Bool("cgroup") {
		delOpts = append(delOpts, opts.WithNamespaceCgroupDeletion)
	}
	return delOpts
}
