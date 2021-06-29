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

package v1

import (
	"context"
	"sync"

	"github.com/containerd/cgroups"
	eventstypes "github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/pkg/oom"
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/containerd/runtime/v2/shim"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// New returns an epoll implementation that listens to OOM events
// from a container's cgroups.
func New(publisher shim.Publisher) (oom.Watcher, error) {
	fd, err := unix.EpollCreate1(unix.EPOLL_CLOEXEC)
	if err != nil {
		return nil, err
	}
	return &epoller{
		fd:        fd,
		publisher: publisher,
		set:       make(map[uintptr]*item),
	}, nil
}

// epoller implementation for handling OOM events from a container's cgroup
type epoller struct {
	mu sync.Mutex

	fd        int
	publisher shim.Publisher
	set       map[uintptr]*item
}

type item struct {
	id string
	cg cgroups.Cgroup
}

// Close the epoll fd
func (e *epoller) Close() error {
	return unix.Close(e.fd)
}

// Run the epoll loop
func (e *epoller) Run(ctx context.Context) {
	var events [128]unix.EpollEvent
	for {
		select {
		case <-ctx.Done():
			e.Close()
			return
		default:
			n, err := unix.EpollWait(e.fd, events[:], -1)
			if err != nil {
				if err == unix.EINTR {
					continue
				}
				logrus.WithError(err).Error("cgroups: epoll wait")
			}
			for i := 0; i < n; i++ {
				e.process(ctx, uintptr(events[i].Fd))
			}
		}
	}
}

// Add cgroups.Cgroup to the epoll monitor
func (e *epoller) Add(id string, cgx interface{}) error {
	cg, ok := cgx.(cgroups.Cgroup)
	if !ok {
		return errors.Errorf("expected cgroups.Cgroup, got: %T", cgx)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	fd, err := cg.OOMEventFD()
	if err != nil {
		return err
	}
	e.set[fd] = &item{
		id: id,
		cg: cg,
	}
	event := unix.EpollEvent{
		Fd:     int32(fd),
		Events: unix.EPOLLHUP | unix.EPOLLIN | unix.EPOLLERR,
	}
	return unix.EpollCtl(e.fd, unix.EPOLL_CTL_ADD, int(fd), &event)
}

func (e *epoller) process(ctx context.Context, fd uintptr) {
	flush(fd)
	e.mu.Lock()
	i, ok := e.set[fd]
	if !ok {
		e.mu.Unlock()
		return
	}
	e.mu.Unlock()
	if i.cg.State() == cgroups.Deleted {
		e.mu.Lock()
		delete(e.set, fd)
		e.mu.Unlock()
		unix.Close(int(fd))
		return
	}
	if err := e.publisher.Publish(ctx, runtime.TaskOOMEventTopic, &eventstypes.TaskOOM{
		ContainerID: i.id,
	}); err != nil {
		logrus.WithError(err).Error("publish OOM event")
	}
}

func flush(fd uintptr) error {
	var buf [8]byte
	_, err := unix.Read(int(fd), buf[:])
	return err
}
