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

package server

import (
	"sync"
	"time"

	"github.com/containerd/containerd"
	eventtypes "github.com/containerd/containerd/api/events"
	containerdio "github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/events"
	"github.com/containerd/typeurl"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"k8s.io/apimachinery/pkg/util/clock"

	"github.com/containerd/cri/pkg/constants"
	ctrdutil "github.com/containerd/cri/pkg/containerd/util"
	"github.com/containerd/cri/pkg/store"
	containerstore "github.com/containerd/cri/pkg/store/container"
	sandboxstore "github.com/containerd/cri/pkg/store/sandbox"
)

const (
	backOffInitDuration        = 1 * time.Second
	backOffMaxDuration         = 5 * time.Minute
	backOffExpireCheckDuration = 1 * time.Second

	// handleEventTimeout is the timeout for handling 1 event. Event monitor
	// handles events in serial, if one event blocks the event monitor, no
	// other events can be handled.
	// Add a timeout for each event handling, events that timeout will be requeued and
	// handled again in the future.
	handleEventTimeout = 10 * time.Second
)

// eventMonitor monitors containerd event and updates internal state correspondingly.
// TODO(random-liu): Handle event for each container in a separate goroutine.
type eventMonitor struct {
	c       *criService
	ch      <-chan *events.Envelope
	errCh   <-chan error
	ctx     context.Context
	cancel  context.CancelFunc
	backOff *backOff
}

type backOff struct {
	queuePool map[string]*backOffQueue
	// tickerMu is mutex used to protect the ticker.
	tickerMu      sync.Mutex
	ticker        *time.Ticker
	minDuration   time.Duration
	maxDuration   time.Duration
	checkDuration time.Duration
	clock         clock.Clock
}

type backOffQueue struct {
	events     []interface{}
	expireTime time.Time
	duration   time.Duration
	clock      clock.Clock
}

// Create new event monitor. New event monitor will start subscribing containerd event. All events
// happen after it should be monitored.
func newEventMonitor(c *criService) *eventMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	return &eventMonitor{
		c:       c,
		ctx:     ctx,
		cancel:  cancel,
		backOff: newBackOff(),
	}
}

// subscribe starts to subscribe containerd events.
func (em *eventMonitor) subscribe(subscriber events.Subscriber) {
	// note: filters are any match, if you want any match but not in namespace foo
	// then you have to manually filter namespace foo
	filters := []string{
		`topic=="/tasks/exit"`,
		`topic=="/tasks/oom"`,
		`topic~="/images/"`,
	}
	em.ch, em.errCh = subscriber.Subscribe(em.ctx, filters...)
}

func convertEvent(e *gogotypes.Any) (string, interface{}, error) {
	id := ""
	evt, err := typeurl.UnmarshalAny(e)
	if err != nil {
		return "", nil, errors.Wrap(err, "failed to unmarshalany")
	}

	switch e := evt.(type) {
	case *eventtypes.TaskExit:
		id = e.ContainerID
	case *eventtypes.TaskOOM:
		id = e.ContainerID
	case *eventtypes.ImageCreate:
		id = e.Name
	case *eventtypes.ImageUpdate:
		id = e.Name
	case *eventtypes.ImageDelete:
		id = e.Name
	default:
		return "", nil, errors.New("unsupported event")
	}
	return id, evt, nil
}

// start starts the event monitor which monitors and handles all subscribed events. It returns
// an error channel for the caller to wait for stop errors from the event monitor.
// start must be called after subscribe.
func (em *eventMonitor) start() <-chan error {
	errCh := make(chan error)
	if em.ch == nil || em.errCh == nil {
		panic("event channel is nil")
	}
	backOffCheckCh := em.backOff.start()
	go func() {
		defer close(errCh)
		for {
			select {
			case e := <-em.ch:
				logrus.Debugf("Received containerd event timestamp - %v, namespace - %q, topic - %q", e.Timestamp, e.Namespace, e.Topic)
				if e.Namespace != constants.K8sContainerdNamespace {
					logrus.Debugf("Ignoring events in namespace - %q", e.Namespace)
					break
				}
				id, evt, err := convertEvent(e.Event)
				if err != nil {
					logrus.WithError(err).Errorf("Failed to convert event %+v", e)
					break
				}
				if em.backOff.isInBackOff(id) {
					logrus.Infof("Events for %q is in backoff, enqueue event %+v", id, evt)
					em.backOff.enBackOff(id, evt)
					break
				}
				if err := em.handleEvent(evt); err != nil {
					logrus.WithError(err).Errorf("Failed to handle event %+v for %s", evt, id)
					em.backOff.enBackOff(id, evt)
				}
			case err := <-em.errCh:
				// Close errCh in defer directly if there is no error.
				if err != nil {
					logrus.WithError(err).Errorf("Failed to handle event stream")
					errCh <- err
				}
				return
			case <-backOffCheckCh:
				ids := em.backOff.getExpiredIDs()
				for _, id := range ids {
					queue := em.backOff.deBackOff(id)
					for i, any := range queue.events {
						if err := em.handleEvent(any); err != nil {
							logrus.WithError(err).Errorf("Failed to handle backOff event %+v for %s", any, id)
							em.backOff.reBackOff(id, queue.events[i:], queue.duration)
							break
						}
					}
				}
			}
		}
	}()
	return errCh
}

// stop stops the event monitor. It will close the event channel.
// Once event monitor is stopped, it can't be started.
func (em *eventMonitor) stop() {
	em.backOff.stop()
	em.cancel()
}

// handleEvent handles a containerd event.
func (em *eventMonitor) handleEvent(any interface{}) error {
	ctx := ctrdutil.NamespacedContext()
	ctx, cancel := context.WithTimeout(ctx, handleEventTimeout)
	defer cancel()

	switch e := any.(type) {
	// If containerd-shim exits unexpectedly, there will be no corresponding event.
	// However, containerd could not retrieve container state in that case, so it's
	// fine to leave out that case for now.
	// TODO(random-liu): [P2] Handle containerd-shim exit.
	case *eventtypes.TaskExit:
		logrus.Infof("TaskExit event %+v", e)
		// Use ID instead of ContainerID to rule out TaskExit event for exec.
		cntr, err := em.c.containerStore.Get(e.ID)
		if err == nil {
			if err := handleContainerExit(ctx, e, cntr); err != nil {
				return errors.Wrap(err, "failed to handle container TaskExit event")
			}
			return nil
		} else if err != store.ErrNotExist {
			return errors.Wrap(err, "can't find container for TaskExit event")
		}
		// Use GetAll to include sandbox in init state.
		sb, err := em.c.sandboxStore.GetAll(e.ID)
		if err == nil {
			if err := handleSandboxExit(ctx, e, sb); err != nil {
				return errors.Wrap(err, "failed to handle sandbox TaskExit event")
			}
			return nil
		} else if err != store.ErrNotExist {
			return errors.Wrap(err, "can't find sandbox for TaskExit event")
		}
		return nil
	case *eventtypes.TaskOOM:
		logrus.Infof("TaskOOM event %+v", e)
		// For TaskOOM, we only care which container it belongs to.
		cntr, err := em.c.containerStore.Get(e.ContainerID)
		if err != nil {
			if err != store.ErrNotExist {
				return errors.Wrap(err, "can't find container for TaskOOM event")
			}
			return nil
		}
		err = cntr.Status.UpdateSync(func(status containerstore.Status) (containerstore.Status, error) {
			status.Reason = oomExitReason
			return status, nil
		})
		if err != nil {
			return errors.Wrap(err, "failed to update container status for TaskOOM event")
		}
	case *eventtypes.ImageCreate:
		logrus.Infof("ImageCreate event %+v", e)
		return em.c.updateImage(ctx, e.Name)
	case *eventtypes.ImageUpdate:
		logrus.Infof("ImageUpdate event %+v", e)
		return em.c.updateImage(ctx, e.Name)
	case *eventtypes.ImageDelete:
		logrus.Infof("ImageDelete event %+v", e)
		return em.c.updateImage(ctx, e.Name)
	}

	return nil
}

// handleContainerExit handles TaskExit event for container.
func handleContainerExit(ctx context.Context, e *eventtypes.TaskExit, cntr containerstore.Container) error {
	// Attach container IO so that `Delete` could cleanup the stream properly.
	task, err := cntr.Container.Task(ctx,
		func(*containerdio.FIFOSet) (containerdio.IO, error) {
			// We can't directly return cntr.IO here, because
			// even if cntr.IO is nil, the cio.IO interface
			// is not.
			// See https://tour.golang.org/methods/12:
			//   Note that an interface value that holds a nil
			//   concrete value is itself non-nil.
			if cntr.IO != nil {
				return cntr.IO, nil
			}
			return nil, nil
		},
	)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return errors.Wrapf(err, "failed to load task for container")
		}
	} else {
		// TODO(random-liu): [P1] This may block the loop, we may want to spawn a worker
		if _, err = task.Delete(ctx, containerd.WithProcessKill); err != nil {
			if !errdefs.IsNotFound(err) {
				return errors.Wrap(err, "failed to stop container")
			}
			// Move on to make sure container status is updated.
		}
	}
	err = cntr.Status.UpdateSync(func(status containerstore.Status) (containerstore.Status, error) {
		// If FinishedAt has been set (e.g. with start failure), keep as
		// it is.
		if status.FinishedAt != 0 {
			return status, nil
		}
		status.Pid = 0
		status.FinishedAt = e.ExitedAt.UnixNano()
		status.ExitCode = int32(e.ExitStatus)
		return status, nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to update container state")
	}
	// Using channel to propagate the information of container stop
	cntr.Stop()
	return nil
}

// handleSandboxExit handles TaskExit event for sandbox.
func handleSandboxExit(ctx context.Context, e *eventtypes.TaskExit, sb sandboxstore.Sandbox) error {
	// No stream attached to sandbox container.
	task, err := sb.Container.Task(ctx, nil)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return errors.Wrap(err, "failed to load task for sandbox")
		}
	} else {
		// TODO(random-liu): [P1] This may block the loop, we may want to spawn a worker
		if _, err = task.Delete(ctx, containerd.WithProcessKill); err != nil {
			if !errdefs.IsNotFound(err) {
				return errors.Wrap(err, "failed to stop sandbox")
			}
			// Move on to make sure container status is updated.
		}
	}
	err = sb.Status.Update(func(status sandboxstore.Status) (sandboxstore.Status, error) {
		// NOTE(random-liu): We SHOULD NOT change INIT state here.
		// If sandbox state is INIT when event monitor receives an TaskExit event,
		// it means that sandbox start has failed. In that case, `RunPodSandbox` will
		// cleanup everything immediately.
		// Once sandbox state goes out of INIT, it becomes visable to the user, which
		// is not what we want.
		if status.State != sandboxstore.StateInit {
			status.State = sandboxstore.StateNotReady
		}
		status.Pid = 0
		return status, nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to update sandbox state")
	}
	// Using channel to propagate the information of sandbox stop
	sb.Stop()
	return nil
}

func newBackOff() *backOff {
	return &backOff{
		queuePool:     map[string]*backOffQueue{},
		minDuration:   backOffInitDuration,
		maxDuration:   backOffMaxDuration,
		checkDuration: backOffExpireCheckDuration,
		clock:         clock.RealClock{},
	}
}

func (b *backOff) getExpiredIDs() []string {
	var ids []string
	for id, q := range b.queuePool {
		if q.isExpire() {
			ids = append(ids, id)
		}
	}
	return ids
}

func (b *backOff) isInBackOff(key string) bool {
	if _, ok := b.queuePool[key]; ok {
		return true
	}
	return false
}

// enBackOff start to backOff and put event to the tail of queue
func (b *backOff) enBackOff(key string, evt interface{}) {
	if queue, ok := b.queuePool[key]; ok {
		queue.events = append(queue.events, evt)
		return
	}
	b.queuePool[key] = newBackOffQueue([]interface{}{evt}, b.minDuration, b.clock)
}

// enBackOff get out the whole queue
func (b *backOff) deBackOff(key string) *backOffQueue {
	queue := b.queuePool[key]
	delete(b.queuePool, key)
	return queue
}

// enBackOff start to backOff again and put events to the queue
func (b *backOff) reBackOff(key string, events []interface{}, oldDuration time.Duration) {
	duration := 2 * oldDuration
	if duration > b.maxDuration {
		duration = b.maxDuration
	}
	b.queuePool[key] = newBackOffQueue(events, duration, b.clock)
}

func (b *backOff) start() <-chan time.Time {
	b.tickerMu.Lock()
	defer b.tickerMu.Unlock()
	b.ticker = time.NewTicker(b.checkDuration)
	return b.ticker.C
}

func (b *backOff) stop() {
	b.tickerMu.Lock()
	defer b.tickerMu.Unlock()
	if b.ticker != nil {
		b.ticker.Stop()
	}
}

func newBackOffQueue(events []interface{}, init time.Duration, c clock.Clock) *backOffQueue {
	return &backOffQueue{
		events:     events,
		duration:   init,
		expireTime: c.Now().Add(init),
		clock:      c,
	}
}

func (q *backOffQueue) isExpire() bool {
	// return time.Now >= expireTime
	return !q.clock.Now().Before(q.expireTime)
}
