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

package v2

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/events/exchange"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/metadata"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/containerd/runtime"
	bolt "go.etcd.io/bbolt"
)

func init() {
	plugin.Register(&plugin.Registration{
		Type: plugin.RuntimePluginV2,
		ID:   "task",
		Requires: []plugin.Type{
			plugin.MetadataPlugin,
		},
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			ic.Meta.Platforms = supportedPlatforms()
			if err := os.MkdirAll(ic.Root, 0711); err != nil {
				return nil, err
			}
			if err := os.MkdirAll(ic.State, 0711); err != nil {
				return nil, err
			}
			m, err := ic.Get(plugin.MetadataPlugin)
			if err != nil {
				return nil, err
			}
			return New(ic.Context, ic.Root, ic.State, ic.Address, ic.Events, m.(*metadata.DB))
		},
	})
}

// New task manager for v2 shims
func New(ctx context.Context, root, state, containerdAddress string, events *exchange.Exchange, db *metadata.DB) (*TaskManager, error) {
	for _, d := range []string{root, state} {
		if err := os.MkdirAll(d, 0711); err != nil {
			return nil, err
		}
	}
	m := &TaskManager{
		root:              root,
		state:             state,
		containerdAddress: containerdAddress,
		tasks:             runtime.NewTaskList(),
		events:            events,
		db:                db,
	}
	if err := m.loadExistingTasks(ctx); err != nil {
		return nil, err
	}
	return m, nil
}

// TaskManager manages v2 shim's and their tasks
type TaskManager struct {
	root              string
	state             string
	containerdAddress string

	tasks  *runtime.TaskList
	events *exchange.Exchange
	db     *metadata.DB
}

// ID of the task manager
func (m *TaskManager) ID() string {
	return fmt.Sprintf("%s.%s", plugin.RuntimePluginV2, "task")
}

// Create a new task
func (m *TaskManager) Create(ctx context.Context, id string, opts runtime.CreateOpts) (_ runtime.Task, err error) {
	bundle, err := NewBundle(ctx, m.root, m.state, id, opts.Spec.Value)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			bundle.Delete()
		}
	}()
	b := shimBinary(ctx, bundle, opts.Runtime, m.containerdAddress, m.events, m.tasks)
	shim, err := b.Start(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			shim.Shutdown(ctx)
			shim.Close()
		}
	}()
	t, err := shim.Create(ctx, opts)
	if err != nil {
		return nil, err
	}
	m.tasks.Add(ctx, t)
	return t, nil
}

// Get a specific task
func (m *TaskManager) Get(ctx context.Context, id string) (runtime.Task, error) {
	return m.tasks.Get(ctx, id)
}

// Add a runtime task
func (m *TaskManager) Add(ctx context.Context, task runtime.Task) error {
	return m.tasks.Add(ctx, task)
}

// Delete a runtime task
func (m *TaskManager) Delete(ctx context.Context, id string) {
	m.tasks.Delete(ctx, id)
}

// Tasks lists all tasks
func (m *TaskManager) Tasks(ctx context.Context, all bool) ([]runtime.Task, error) {
	return m.tasks.GetAll(ctx, all)
}

func (m *TaskManager) loadExistingTasks(ctx context.Context) error {
	nsDirs, err := ioutil.ReadDir(m.state)
	if err != nil {
		return err
	}
	for _, nsd := range nsDirs {
		if !nsd.IsDir() {
			continue
		}
		ns := nsd.Name()
		log.G(ctx).WithField("namespace", ns).Debug("loading tasks in namespace")
		if err := m.loadTasks(namespaces.WithNamespace(ctx, ns)); err != nil {
			log.G(ctx).WithField("namespace", ns).WithError(err).Error("loading tasks in namespace")
			continue
		}
		if err := m.cleanupWorkDirs(namespaces.WithNamespace(ctx, ns)); err != nil {
			log.G(ctx).WithField("namespace", ns).WithError(err).Error("cleanup working directory in namespace")
			continue
		}
	}
	return nil
}

func (m *TaskManager) loadTasks(ctx context.Context) error {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return err
	}
	shimDirs, err := ioutil.ReadDir(filepath.Join(m.state, ns))
	if err != nil {
		return err
	}
	for _, sd := range shimDirs {
		if !sd.IsDir() {
			continue
		}
		id := sd.Name()
		bundle, err := LoadBundle(ctx, m.state, id)
		if err != nil {
			// fine to return error here, it is a programmer error if the context
			// does not have a namespace
			return err
		}
		// fast path
		bf, err := ioutil.ReadDir(bundle.Path)
		if err != nil {
			bundle.Delete()
			log.G(ctx).WithError(err).Errorf("fast path read bundle path for %s", bundle.Path)
			continue
		}
		if len(bf) == 0 {
			bundle.Delete()
			continue
		}
		shim, err := loadShim(ctx, bundle, m.events, m.tasks)
		if err != nil {
			log.G(ctx).WithError(err).Errorf("cleanup dead shim %s", id)
			container, err := m.container(ctx, id)
			if err != nil {
				log.G(ctx).WithError(err).Errorf("loading dead container %s", id)
				if err := mount.UnmountAll(filepath.Join(bundle.Path, "rootfs"), 0); err != nil {
					log.G(ctx).WithError(err).Errorf("forceful unmount of rootfs %s", id)
				}
				bundle.Delete()
				continue
			}
			binaryCall := shimBinary(ctx, bundle, container.Runtime.Name, m.containerdAddress, m.events, m.tasks)
			if _, err := binaryCall.Delete(ctx); err != nil {
				log.G(ctx).WithError(err).Errorf("binary call to delete for %s", id)
				continue
			}
			continue
		}
		m.tasks.Add(ctx, shim)
	}
	return nil
}

func (m *TaskManager) container(ctx context.Context, id string) (*containers.Container, error) {
	var container containers.Container
	if err := m.db.View(func(tx *bolt.Tx) error {
		store := metadata.NewContainerStore(tx)
		var err error
		container, err = store.Get(ctx, id)
		return err
	}); err != nil {
		return nil, err
	}
	return &container, nil
}

func (m *TaskManager) cleanupWorkDirs(ctx context.Context) error {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return err
	}
	dirs, err := ioutil.ReadDir(filepath.Join(m.root, ns))
	if err != nil {
		return err
	}
	for _, d := range dirs {
		// if the task was not loaded, cleanup and empty working directory
		// this can happen on a reboot where /run for the bundle state is cleaned up
		// but that persistent working dir is left
		if _, err := m.tasks.Get(ctx, d.Name()); err != nil {
			path := filepath.Join(m.root, ns, d.Name())
			if err := os.RemoveAll(path); err != nil {
				log.G(ctx).WithError(err).Errorf("cleanup working dir %s", path)
			}
		}
	}
	return nil
}
