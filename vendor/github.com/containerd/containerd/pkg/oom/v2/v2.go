// +build linux

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

	cgroupsv2 "github.com/containerd/cgroups/v2"
	eventstypes "github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/pkg/oom"
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/containerd/runtime/v2/shim"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// New returns an implementation that listens to OOM events
// from a container's cgroups.
func New(publisher shim.Publisher) (oom.Watcher, error) {
	return &watcher{
		itemCh:    make(chan item),
		publisher: publisher,
	}, nil
}

// watcher implementation for handling OOM events from a container's cgroup
type watcher struct {
	itemCh    chan item
	publisher shim.Publisher
}

type item struct {
	id  string
	ev  cgroupsv2.Event
	err error
}

// Close closes the watcher
func (w *watcher) Close() error {
	return nil
}

// Run the loop
func (w *watcher) Run(ctx context.Context) {
	lastOOMMap := make(map[string]uint64) // key: id, value: ev.OOM
	for {
		select {
		case <-ctx.Done():
			w.Close()
			return
		case i := <-w.itemCh:
			if i.err != nil {
				delete(lastOOMMap, i.id)
				continue
			}
			lastOOM := lastOOMMap[i.id]
			if i.ev.OOM > lastOOM {
				if err := w.publisher.Publish(ctx, runtime.TaskOOMEventTopic, &eventstypes.TaskOOM{
					ContainerID: i.id,
				}); err != nil {
					logrus.WithError(err).Error("publish OOM event")
				}
			}
			if i.ev.OOM > 0 {
				lastOOMMap[i.id] = i.ev.OOM
			}
		}
	}
}

// Add cgroups.Cgroup to the epoll monitor
func (w *watcher) Add(id string, cgx interface{}) error {
	cg, ok := cgx.(*cgroupsv2.Manager)
	if !ok {
		return errors.Errorf("expected *cgroupsv2.Manager, got: %T", cgx)
	}
	// FIXME: cgroupsv2.Manager does not support closing eventCh routine currently.
	// The routine shuts down when an error happens, mostly when the cgroup is deleted.
	eventCh, errCh := cg.EventChan()
	go func() {
		for {
			i := item{id: id}
			select {
			case ev := <-eventCh:
				i.ev = ev
				w.itemCh <- i
			case err := <-errCh:
				i.err = err
				w.itemCh <- i
				// we no longer get any event/err when we got an err
				logrus.WithError(err).Warn("error from *cgroupsv2.Manager.EventChan")
				return
			}
		}
	}()
	return nil
}
