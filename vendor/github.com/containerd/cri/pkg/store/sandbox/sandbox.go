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

package sandbox

import (
	"sync"

	"github.com/containerd/containerd"
	"github.com/docker/docker/pkg/truncindex"

	"github.com/containerd/cri/pkg/netns"
	"github.com/containerd/cri/pkg/store"
)

// Sandbox contains all resources associated with the sandbox. All methods to
// mutate the internal state are thread safe.
type Sandbox struct {
	// Metadata is the metadata of the sandbox, it is immutable after created.
	Metadata
	// Status stores the status of the sandbox.
	Status StatusStorage
	// Container is the containerd sandbox container client.
	Container containerd.Container
	// CNI network namespace client.
	// For hostnetwork pod, this is always nil;
	// For non hostnetwork pod, this should never be nil.
	NetNS *netns.NetNS
	// StopCh is used to propagate the stop information of the sandbox.
	*store.StopCh
}

// NewSandbox creates an internally used sandbox type. This functions reminds
// the caller that a sandbox must have a status.
func NewSandbox(metadata Metadata, status Status) Sandbox {
	s := Sandbox{
		Metadata: metadata,
		Status:   StoreStatus(status),
		StopCh:   store.NewStopCh(),
	}
	if status.State == StateNotReady {
		s.Stop()
	}
	return s
}

// Store stores all sandboxes.
type Store struct {
	lock      sync.RWMutex
	sandboxes map[string]Sandbox
	idIndex   *truncindex.TruncIndex
}

// NewStore creates a sandbox store.
func NewStore() *Store {
	return &Store{
		sandboxes: make(map[string]Sandbox),
		idIndex:   truncindex.NewTruncIndex([]string{}),
	}
}

// Add a sandbox into the store.
func (s *Store) Add(sb Sandbox) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, ok := s.sandboxes[sb.ID]; ok {
		return store.ErrAlreadyExist
	}
	if err := s.idIndex.Add(sb.ID); err != nil {
		return err
	}
	s.sandboxes[sb.ID] = sb
	return nil
}

// Get returns the sandbox with specified id. Returns store.ErrNotExist
// if the sandbox doesn't exist.
func (s *Store) Get(id string) (Sandbox, error) {
	sb, err := s.GetAll(id)
	if err != nil {
		return sb, err
	}
	if sb.Status.Get().State == StateUnknown {
		return Sandbox{}, store.ErrNotExist
	}
	return sb, nil
}

// GetAll returns the sandbox with specified id, including sandbox in unknown
// state. Returns store.ErrNotExist if the sandbox doesn't exist.
func (s *Store) GetAll(id string) (Sandbox, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	id, err := s.idIndex.Get(id)
	if err != nil {
		if err == truncindex.ErrNotExist {
			err = store.ErrNotExist
		}
		return Sandbox{}, err
	}
	if sb, ok := s.sandboxes[id]; ok {
		return sb, nil
	}
	return Sandbox{}, store.ErrNotExist
}

// List lists all sandboxes.
func (s *Store) List() []Sandbox {
	s.lock.RLock()
	defer s.lock.RUnlock()
	var sandboxes []Sandbox
	for _, sb := range s.sandboxes {
		if sb.Status.Get().State == StateUnknown {
			continue
		}
		sandboxes = append(sandboxes, sb)
	}
	return sandboxes
}

// Delete deletes the sandbox with specified id.
func (s *Store) Delete(id string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	id, err := s.idIndex.Get(id)
	if err != nil {
		// Note: The idIndex.Delete and delete doesn't handle truncated index.
		// So we need to return if there are error.
		return
	}
	s.idIndex.Delete(id) // nolint: errcheck
	delete(s.sandboxes, id)
}
