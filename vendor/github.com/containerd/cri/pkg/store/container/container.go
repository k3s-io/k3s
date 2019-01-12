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

package container

import (
	"sync"

	"github.com/containerd/containerd"
	"github.com/docker/docker/pkg/truncindex"
	runtime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

	cio "github.com/containerd/cri/pkg/server/io"
	"github.com/containerd/cri/pkg/store"
)

// Container contains all resources associated with the container. All methods to
// mutate the internal state are thread-safe.
type Container struct {
	// Metadata is the metadata of the container, it is **immutable** after created.
	Metadata
	// Status stores the status of the container.
	Status StatusStorage
	// Container is the containerd container client.
	Container containerd.Container
	// Container IO
	IO *cio.ContainerIO
	// StopCh is used to propagate the stop information of the container.
	*store.StopCh
}

// Opts sets specific information to newly created Container.
type Opts func(*Container) error

// WithContainer adds the containerd Container to the internal data store.
func WithContainer(cntr containerd.Container) Opts {
	return func(c *Container) error {
		c.Container = cntr
		return nil
	}
}

// WithContainerIO adds IO into the container.
func WithContainerIO(io *cio.ContainerIO) Opts {
	return func(c *Container) error {
		c.IO = io
		return nil
	}
}

// WithStatus adds status to the container.
func WithStatus(status Status, root string) Opts {
	return func(c *Container) error {
		s, err := StoreStatus(root, c.ID, status)
		if err != nil {
			return err
		}
		c.Status = s
		if s.Get().State() == runtime.ContainerState_CONTAINER_EXITED {
			c.Stop()
		}
		return nil
	}
}

// NewContainer creates an internally used container type.
func NewContainer(metadata Metadata, opts ...Opts) (Container, error) {
	c := Container{
		Metadata: metadata,
		StopCh:   store.NewStopCh(),
	}
	for _, o := range opts {
		if err := o(&c); err != nil {
			return Container{}, err
		}
	}
	return c, nil
}

// Delete deletes checkpoint for the container.
func (c *Container) Delete() error {
	return c.Status.Delete()
}

// Store stores all Containers.
type Store struct {
	lock       sync.RWMutex
	containers map[string]Container
	idIndex    *truncindex.TruncIndex
}

// NewStore creates a container store.
func NewStore() *Store {
	return &Store{
		containers: make(map[string]Container),
		idIndex:    truncindex.NewTruncIndex([]string{}),
	}
}

// Add a container into the store. Returns store.ErrAlreadyExist if the
// container already exists.
func (s *Store) Add(c Container) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, ok := s.containers[c.ID]; ok {
		return store.ErrAlreadyExist
	}
	if err := s.idIndex.Add(c.ID); err != nil {
		return err
	}
	s.containers[c.ID] = c
	return nil
}

// Get returns the container with specified id. Returns store.ErrNotExist
// if the container doesn't exist.
func (s *Store) Get(id string) (Container, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	id, err := s.idIndex.Get(id)
	if err != nil {
		if err == truncindex.ErrNotExist {
			err = store.ErrNotExist
		}
		return Container{}, err
	}
	if c, ok := s.containers[id]; ok {
		return c, nil
	}
	return Container{}, store.ErrNotExist
}

// List lists all containers.
func (s *Store) List() []Container {
	s.lock.RLock()
	defer s.lock.RUnlock()
	var containers []Container
	for _, c := range s.containers {
		containers = append(containers, c)
	}
	return containers
}

// Delete deletes the container from store with specified id.
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
	delete(s.containers, id)
}
