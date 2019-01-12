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

package snapshot

import (
	"sync"

	snapshot "github.com/containerd/containerd/snapshots"

	"github.com/containerd/cri/pkg/store"
)

// Snapshot contains the information about the snapshot.
type Snapshot struct {
	// Key is the key of the snapshot
	Key string
	// Kind is the kind of the snapshot (active, commited, view)
	Kind snapshot.Kind
	// Size is the size of the snapshot in bytes.
	Size uint64
	// Inodes is the number of inodes used by the snapshot
	Inodes uint64
	// Timestamp is latest update time (in nanoseconds) of the snapshot
	// information.
	Timestamp int64
}

// Store stores all snapshots.
type Store struct {
	lock      sync.RWMutex
	snapshots map[string]Snapshot
}

// NewStore creates a snapshot store.
func NewStore() *Store {
	return &Store{snapshots: make(map[string]Snapshot)}
}

// Add a snapshot into the store.
func (s *Store) Add(snapshot Snapshot) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.snapshots[snapshot.Key] = snapshot
}

// Get returns the snapshot with specified key. Returns store.ErrNotExist if the
// snapshot doesn't exist.
func (s *Store) Get(key string) (Snapshot, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	if sn, ok := s.snapshots[key]; ok {
		return sn, nil
	}
	return Snapshot{}, store.ErrNotExist
}

// List lists all snapshots.
func (s *Store) List() []Snapshot {
	s.lock.RLock()
	defer s.lock.RUnlock()
	var snapshots []Snapshot
	for _, sn := range s.snapshots {
		snapshots = append(snapshots, sn)
	}
	return snapshots
}

// Delete deletes the snapshot with specified key.
func (s *Store) Delete(key string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.snapshots, key)
}
