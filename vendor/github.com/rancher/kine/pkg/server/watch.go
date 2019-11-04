package server

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/sirupsen/logrus"
)

var (
	watchID int64
)

func (s *KVServerBridge) Watch(ws etcdserverpb.Watch_WatchServer) error {
	w := watcher{
		server:  ws,
		backend: s.limited.backend,
		watches: map[int64]func(){},
	}
	defer w.Close()

	for {
		msg, err := ws.Recv()
		if err != nil {
			return err
		}

		if msg.GetCreateRequest() != nil {
			w.Start(msg.GetCreateRequest())
		} else if msg.GetCancelRequest() != nil {
			w.Cancel(msg.GetCancelRequest().WatchId)
		}
	}
}

type watcher struct {
	sync.Mutex

	wg      sync.WaitGroup
	backend Backend
	server  etcdserverpb.Watch_WatchServer
	watches map[int64]func()
}

func (w *watcher) Start(r *etcdserverpb.WatchCreateRequest) {
	w.Lock()
	defer w.Unlock()

	ctx, cancel := context.WithCancel(context.Background())

	id := atomic.AddInt64(&watchID, 1)
	w.watches[id] = cancel
	w.wg.Add(1)

	key := string(r.Key)

	logrus.Debugf("WATCH START id=%d, key=%s, revision=%d", id, key, r.StartRevision)

	go func() {
		defer w.wg.Done()
		if err := w.server.Send(&etcdserverpb.WatchResponse{
			Header:  &etcdserverpb.ResponseHeader{},
			Created: true,
			WatchId: id,
		}); err != nil {
			w.Cancel(id)
			return
		}

		for events := range w.backend.Watch(ctx, key, r.StartRevision) {
			if len(events) == 0 {
				continue
			}

			if logrus.IsLevelEnabled(logrus.DebugLevel) {
				for _, event := range events {
					logrus.Debugf("WATCH READ id=%d, key=%s, revision=%d", id, event.KV.Key, event.KV.ModRevision)
				}
			}

			if err := w.server.Send(&etcdserverpb.WatchResponse{
				Header:  txnHeader(events[len(events)-1].KV.ModRevision),
				WatchId: id,
				Events:  toEvents(events...),
			}); err != nil {
				w.Cancel(id)
				continue
			}
		}
		logrus.Debugf("WATCH CLOSE id=%d, key=%s", id, key)
	}()
}

func toEvents(events ...*Event) []*mvccpb.Event {
	ret := make([]*mvccpb.Event, 0, len(events))
	for _, e := range events {
		ret = append(ret, toEvent(e))
	}
	return ret
}

func toEvent(event *Event) *mvccpb.Event {
	e := &mvccpb.Event{
		Kv:     toKV(event.KV),
		PrevKv: toKV(event.PrevKV),
	}
	if event.Delete {
		e.Type = mvccpb.DELETE
	} else {
		e.Type = mvccpb.PUT
	}

	return e
}

func (w *watcher) Cancel(watchID int64) {
	w.Lock()
	defer w.Unlock()
	if cancel, ok := w.watches[watchID]; ok {
		cancel()
		delete(w.watches, watchID)
	}
	err := w.server.Send(&etcdserverpb.WatchResponse{
		Header:   &etcdserverpb.ResponseHeader{},
		Canceled: true,
		WatchId:  watchID,
	})
	if err != nil {
		logrus.Errorf("Failed to send cancel response for watchID %d: %v", watchID, err)
	}
}

func (w *watcher) Close() {
	w.Lock()
	defer w.Unlock()
	for _, v := range w.watches {
		v()
	}
	w.wg.Wait()
}
