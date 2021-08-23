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
	"context"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/log"
	"github.com/containerd/nri"
	v1 "github.com/containerd/nri/types/v1"
)

// WithNRISandboxDelete calls delete for a sandbox'd task
func WithNRISandboxDelete(sandboxID string) containerd.ProcessDeleteOpts {
	return func(ctx context.Context, p containerd.Process) error {
		task, ok := p.(containerd.Task)
		if !ok {
			return nil
		}
		nric, err := nri.New()
		if err != nil {
			log.G(ctx).WithError(err).Error("unable to create nri client")
			return nil
		}
		if nric == nil {
			return nil
		}
		sb := &nri.Sandbox{
			ID: sandboxID,
		}
		if _, err := nric.InvokeWithSandbox(ctx, task, v1.Delete, sb); err != nil {
			log.G(ctx).WithError(err).Errorf("Failed to delete nri for %q", task.ID())
		}
		return nil
	}
}
