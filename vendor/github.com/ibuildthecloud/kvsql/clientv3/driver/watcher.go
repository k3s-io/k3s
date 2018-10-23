package driver

import (
	"context"
	"io"
	"strings"
)

type Event struct {
	KV    *KeyValue
	Err   error
	Start bool
}

func matchesKey(prefix bool, key string, kv *KeyValue) bool {
	if kv == nil {
		return false
	}
	if prefix {
		return strings.HasPrefix(kv.Key, key[:len(key)-1])
	}
	return kv.Key == key
}

func (g *Generic) globalWatcher() (chan map[string]interface{}, error) {
	ctx, cancel := context.WithCancel(context.Background())
	g.cancel = cancel
	result := make(chan map[string]interface{}, 100)

	go func() {
		defer close(result)
		for {
			select {
			case <-ctx.Done():
				return
			case e := <-g.changes:
				result <- map[string]interface{}{
					"data": e,
				}
			}
		}
	}()

	return result, nil
}

func (g *Generic) Watch(ctx context.Context, key string, revision int64) <-chan Event {
	ctx, parentCancel := context.WithCancel(ctx)

	prefix := strings.HasSuffix(key, "%")

	events, err := g.broadcaster.Subscribe(ctx, g.globalWatcher)
	if err != nil {
		panic(err)
	}

	watchChan := make(chan Event)
	go func() (returnErr error) {
		defer func() {
			sendErrorAndClose(watchChan, returnErr)
			parentCancel()
		}()

		start(watchChan)

		if revision > 0 {
			keys, err := g.replayEvents(ctx, key, revision)
			if err != nil {
				return err
			}

			for _, k := range keys {
				watchChan <- Event{KV: k}
			}
		}

		for e := range events {
			k, ok := e["data"].(*KeyValue)
			if ok && matchesKey(prefix, key, k) {
				watchChan <- Event{KV: k}
			}
		}

		return nil
	}()

	return watchChan
}

func start(watchResponses chan Event) {
	watchResponses <- Event{
		Start: true,
	}
}

func sendErrorAndClose(watchResponses chan Event, err error) {
	if err == nil {
		err = io.EOF
	}
	watchResponses <- Event{Err: err}
	close(watchResponses)
}

// Close closes the watcher and cancels all watch requests.
func (g *Generic) Close() error {
	if g.cancel != nil {
		g.cancel()
		g.cancel = nil
	}
	return nil
}
