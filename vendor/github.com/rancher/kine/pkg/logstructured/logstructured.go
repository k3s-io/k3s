package logstructured

import (
	"context"
	"sync"
	"time"

	"github.com/rancher/kine/pkg/server"
	"github.com/sirupsen/logrus"
)

type Log interface {
	Start(ctx context.Context) error
	CurrentRevision(ctx context.Context) (int64, error)
	List(ctx context.Context, prefix, startKey string, limit, revision int64, includeDeletes bool) (int64, []*server.Event, error)
	After(ctx context.Context, prefix string, revision, limit int64) (int64, []*server.Event, error)
	Watch(ctx context.Context, prefix string) <-chan []*server.Event
	Count(ctx context.Context, prefix string) (int64, int64, error)
	Append(ctx context.Context, event *server.Event) (int64, error)
}

type LogStructured struct {
	log Log
}

func New(log Log) *LogStructured {
	return &LogStructured{
		log: log,
	}
}

func (l *LogStructured) Start(ctx context.Context) error {
	if err := l.log.Start(ctx); err != nil {
		return err
	}
	l.Create(ctx, "/registry/health", []byte(`{"health":"true"}`), 0)
	go l.ttl(ctx)
	return nil
}

func (l *LogStructured) Get(ctx context.Context, key string, revision int64) (revRet int64, kvRet *server.KeyValue, errRet error) {
	defer func() {
		l.adjustRevision(ctx, &revRet)
		logrus.Debugf("GET %s, rev=%d => rev=%d, kv=%v, err=%v", key, revision, revRet, kvRet != nil, errRet)
	}()

	rev, event, err := l.get(ctx, key, revision, false)
	if event == nil {
		return rev, nil, err
	}
	return rev, event.KV, err
}

func (l *LogStructured) get(ctx context.Context, key string, revision int64, includeDeletes bool) (int64, *server.Event, error) {
	rev, events, err := l.log.List(ctx, key, "", 1, revision, includeDeletes)
	if err == server.ErrCompacted {
		// ignore compacted when getting by revision
		err = nil
	}
	if err != nil {
		return 0, nil, err
	}
	if revision != 0 {
		rev = revision
	}
	if len(events) == 0 {
		return rev, nil, nil
	}
	return rev, events[0], nil
}

func (l *LogStructured) adjustRevision(ctx context.Context, rev *int64) {
	if *rev != 0 {
		return
	}

	if newRev, err := l.log.CurrentRevision(ctx); err == nil {
		*rev = newRev
	}
}

func (l *LogStructured) Create(ctx context.Context, key string, value []byte, lease int64) (revRet int64, errRet error) {
	defer func() {
		l.adjustRevision(ctx, &revRet)
		logrus.Debugf("CREATE %s, size=%d, lease=%d => rev=%d, err=%v", key, len(value), lease, revRet, errRet)
	}()

	rev, prevEvent, err := l.get(ctx, key, 0, true)
	if err != nil {
		return 0, err
	}
	createEvent := &server.Event{
		Create: true,
		KV: &server.KeyValue{
			Key:   key,
			Value: value,
			Lease: lease,
		},
		PrevKV: &server.KeyValue{
			ModRevision: rev,
		},
	}
	if prevEvent != nil {
		if !prevEvent.Delete {
			return 0, server.ErrKeyExists
		}
		createEvent.PrevKV = prevEvent.KV
	}

	revRet, errRet = l.log.Append(ctx, createEvent)
	return
}

func (l *LogStructured) Delete(ctx context.Context, key string, revision int64) (revRet int64, kvRet *server.KeyValue, deletedRet bool, errRet error) {
	defer func() {
		l.adjustRevision(ctx, &revRet)
		logrus.Debugf("DELETE %s, rev=%d => rev=%d, kv=%v, deleted=%v, err=%v", key, revision, revRet, kvRet != nil, deletedRet, errRet)
	}()

	rev, event, err := l.get(ctx, key, 0, true)
	if err != nil {
		return 0, nil, false, err
	}

	if event == nil {
		return rev, nil, true, nil
	}

	if event.Delete {
		return rev, event.KV, true, nil
	}

	if revision != 0 && event.KV.ModRevision != revision {
		return rev, event.KV, false, nil
	}

	deleteEvent := &server.Event{
		Delete: true,
		KV:     event.KV,
		PrevKV: event.KV,
	}

	rev, err = l.log.Append(ctx, deleteEvent)
	if err != nil {
		// If error on Append we assume it's a UNIQUE constraint error, so we fetch the latest (if we can)
		// and return that the delete failed
		latestRev, latestEvent, latestErr := l.get(ctx, key, 0, true)
		if latestErr != nil || latestEvent == nil {
			return rev, event.KV, false, nil
		}
		return latestRev, latestEvent.KV, false, nil
	}
	return rev, event.KV, true, err
}

func (l *LogStructured) List(ctx context.Context, prefix, startKey string, limit, revision int64) (revRet int64, kvRet []*server.KeyValue, errRet error) {
	defer func() {
		logrus.Debugf("LIST %s, start=%s, limit=%d, rev=%d => rev=%d, kvs=%d, err=%v", prefix, startKey, limit, revision, revRet, len(kvRet), errRet)
	}()

	rev, events, err := l.log.List(ctx, prefix, startKey, limit, revision, false)
	if err != nil {
		return 0, nil, err
	}
	if revision == 0 && len(events) == 0 {
		// if no revision is requested and no events are returned, then
		// get the current revision and relist.  Relist is required because
		// between now and getting the current revision something could have
		// been created.
		currentRev, err := l.log.CurrentRevision(ctx)
		if err != nil {
			return 0, nil, err
		}
		return l.List(ctx, prefix, startKey, limit, currentRev)
	} else if revision != 0 {
		rev = revision
	}

	kvs := make([]*server.KeyValue, 0, len(events))
	for _, event := range events {
		kvs = append(kvs, event.KV)
	}
	return rev, kvs, nil
}

func (l *LogStructured) Count(ctx context.Context, prefix string) (revRet int64, count int64, err error) {
	defer func() {
		logrus.Debugf("COUNT %s => rev=%d, count=%d, err=%v", prefix, revRet, count, err)
	}()
	rev, count, err := l.log.Count(ctx, prefix)
	if err != nil {
		return 0, 0, err
	}

	if count == 0 {
		// if count is zero, then so is revision, so now get the current revision and re-count at that revision
		currentRev, err := l.log.CurrentRevision(ctx)
		if err != nil {
			return 0, 0, err
		}
		rev, rows, err := l.List(ctx, prefix, prefix, 1000, currentRev)
		return rev, int64(len(rows)), err
	}
	return rev, count, nil
}

func (l *LogStructured) Update(ctx context.Context, key string, value []byte, revision, lease int64) (revRet int64, kvRet *server.KeyValue, updateRet bool, errRet error) {
	defer func() {
		l.adjustRevision(ctx, &revRet)
		kvRev := int64(0)
		if kvRet != nil {
			kvRev = kvRet.ModRevision
		}
		logrus.Debugf("UPDATE %s, value=%d, rev=%d, lease=%v => rev=%d, kvrev=%d, updated=%v, err=%v", key, len(value), revision, lease, revRet, kvRev, updateRet, errRet)
	}()

	rev, event, err := l.get(ctx, key, 0, false)
	if err != nil {
		return 0, nil, false, err
	}

	if event == nil {
		return 0, nil, false, nil
	}

	if event.KV.ModRevision != revision {
		return rev, event.KV, false, nil
	}

	updateEvent := &server.Event{
		KV: &server.KeyValue{
			Key:            key,
			CreateRevision: event.KV.CreateRevision,
			Value:          value,
			Lease:          lease,
		},
		PrevKV: event.KV,
	}

	rev, err = l.log.Append(ctx, updateEvent)
	if err != nil {
		rev, event, err := l.get(ctx, key, 0, false)
		if event == nil {
			return rev, nil, false, err
		}
		return rev, event.KV, false, err
	}

	updateEvent.KV.ModRevision = rev
	return rev, updateEvent.KV, true, err
}

func (l *LogStructured) ttlEvents(ctx context.Context) chan *server.Event {
	result := make(chan *server.Event)
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		wg.Wait()
		close(result)
	}()

	go func() {
		defer wg.Done()
		rev, events, err := l.log.List(ctx, "/", "", 1000, 0, false)
		for len(events) > 0 {
			if err != nil {
				logrus.Errorf("failed to read old events for ttl")
				return
			}

			for _, event := range events {
				if event.KV.Lease > 0 {
					result <- event
				}
			}

			_, events, err = l.log.List(ctx, "/", events[len(events)-1].KV.Key, 1000, rev, false)
		}
	}()

	go func() {
		defer wg.Done()
		for events := range l.log.Watch(ctx, "/") {
			for _, event := range events {
				if event.KV.Lease > 0 {
					result <- event
				}
			}
		}
	}()

	return result
}

func (l *LogStructured) ttl(ctx context.Context) {
	// vary naive TTL support
	mutex := &sync.Mutex{}
	for event := range l.ttlEvents(ctx) {
		go func(event *server.Event) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(event.KV.Lease) * time.Second):
			}
			mutex.Lock()
			l.Delete(ctx, event.KV.Key, event.KV.ModRevision)
			mutex.Unlock()
		}(event)
	}
}

func (l *LogStructured) Watch(ctx context.Context, prefix string, revision int64) <-chan []*server.Event {
	logrus.Debugf("WATCH %s, revision=%d", prefix, revision)

	// starting watching right away so we don't miss anything
	ctx, cancel := context.WithCancel(ctx)
	readChan := l.log.Watch(ctx, prefix)

	// include the current revision in list
	if revision > 0 {
		revision -= 1
	}

	result := make(chan []*server.Event, 100)

	rev, kvs, err := l.log.After(ctx, prefix, revision, 0)
	if err != nil {
		logrus.Errorf("failed to list %s for revision %d", prefix, revision)
		cancel()
	}

	logrus.Debugf("WATCH LIST key=%s rev=%d => rev=%d kvs=%d", prefix, revision, rev, len(kvs))

	go func() {
		lastRevision := revision
		if len(kvs) > 0 {
			lastRevision = rev
		}

		if len(kvs) > 0 {
			result <- kvs
		}

		// always ensure we fully read the channel
		for i := range readChan {
			result <- filter(i, lastRevision)
		}
		close(result)
		cancel()
	}()

	return result
}

func filter(events []*server.Event, rev int64) []*server.Event {
	for len(events) > 0 && events[0].KV.ModRevision <= rev {
		events = events[1:]
	}

	return events
}
