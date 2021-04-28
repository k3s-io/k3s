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

package sandbox

import (
	"strconv"
	"sync"
	"time"

	runtime "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

// The sandbox state machine in the CRI plugin:
//                    +              +
//                    |              |
//                    | Create(Run)  | Load
//                    |              |
//      Start         |              |
//     (failed)       |              |
// +------------------+              +-----------+
// |                  |              |           |
// |                  |              |           |
// |                  |              |           |
// |                  | Start(Run)   |           |
// |                  |              |           |
// | PortForward +----v----+         |           |
// |      +------+         |         |           |
// |      |      |  READY  <---------+           |
// |      +------>         |         |           |
// |             +----+----+         |           |
// |                  |              |           |
// |                  | Stop/Exit    |           |
// |                  |              |           |
// |             +----v----+         |           |
// |             |         <---------+      +----v----+
// |             | NOTREADY|                |         |
// |             |         <----------------+ UNKNOWN |
// |             +----+----+       Stop     |         |
// |                  |                     +---------+
// |                  | Remove
// |                  v
// +-------------> DELETED

// State is the sandbox state we use in containerd/cri.
// It includes unknown, which is internal states not defined in CRI.
// The state mapping from internal states to CRI states:
// * ready -> ready
// * not ready -> not ready
// * unknown -> not ready
type State uint32

const (
	// StateReady is ready state, it means sandbox container
	// is running.
	StateReady State = iota
	// StateNotReady is notready state, it ONLY means sandbox
	// container is not running.
	// StopPodSandbox should still be called for NOTREADY sandbox to
	// cleanup resources other than sandbox container, e.g. network namespace.
	// This is an assumption made in CRI.
	StateNotReady
	// StateUnknown is unknown state. Sandbox only goes
	// into unknown state when its status fails to be loaded.
	StateUnknown
)

// String returns the string representation of the state
func (s State) String() string {
	switch s {
	case StateReady:
		return runtime.PodSandboxState_SANDBOX_READY.String()
	case StateNotReady:
		return runtime.PodSandboxState_SANDBOX_NOTREADY.String()
	case StateUnknown:
		// PodSandboxState doesn't have an unknown state, but State does, so return a string using the same convention
		return "SANDBOX_UNKNOWN"
	default:
		return "invalid sandbox state value: " + strconv.Itoa(int(s))
	}
}

// Status is the status of a sandbox.
type Status struct {
	// Pid is the init process id of the sandbox container.
	Pid uint32
	// CreatedAt is the created timestamp.
	CreatedAt time.Time
	// State is the state of the sandbox.
	State State
}

// UpdateFunc is function used to update the sandbox status. If there
// is an error, the update will be rolled back.
type UpdateFunc func(Status) (Status, error)

// StatusStorage manages the sandbox status.
// The status storage for sandbox is different from container status storage,
// because we don't checkpoint sandbox status. If we need checkpoint in the
// future, we should combine this with container status storage.
type StatusStorage interface {
	// Get a sandbox status.
	Get() Status
	// Update the sandbox status. Note that the update MUST be applied
	// in one transaction.
	Update(UpdateFunc) error
}

// StoreStatus creates the storage containing the passed in sandbox status with the
// specified id.
// The status MUST be created in one transaction.
func StoreStatus(status Status) StatusStorage {
	return &statusStorage{status: status}
}

type statusStorage struct {
	sync.RWMutex
	status Status
}

// Get a copy of sandbox status.
func (s *statusStorage) Get() Status {
	s.RLock()
	defer s.RUnlock()
	return s.status
}

// Update the sandbox status.
func (s *statusStorage) Update(u UpdateFunc) error {
	s.Lock()
	defer s.Unlock()
	newStatus, err := u(s.status)
	if err != nil {
		return err
	}
	s.status = newStatus
	return nil
}
