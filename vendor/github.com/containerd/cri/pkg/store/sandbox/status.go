/*
Copyright 2018 The Containerd Authors.

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
	"sync"
	"time"
)

// State is the sandbox state we use in containerd/cri.
// It has unknown state defined.
type State uint32

const (
	// StateUnknown is unknown state of sandbox. Sandbox
	// is in unknown state before its corresponding sandbox container
	// is created. Sandbox in unknown state should be ignored by most
	// functions, unless the caller needs to update sandbox state.
	StateUnknown State = iota
	// StateReady is ready state, it means sandbox container
	// is running.
	StateReady
	// StateNotReady is notready state, it ONLY means sandbox
	// container is not running.
	// StopPodSandbox should still be called for NOTREADY sandbox to
	// cleanup resources other than sandbox container, e.g. network namespace.
	// This is an assumption made in CRI.
	StateNotReady
)

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
