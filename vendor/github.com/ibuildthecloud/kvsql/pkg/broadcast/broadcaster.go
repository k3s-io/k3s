package broadcast

import (
	"context"
	"sync"
)

type ConnectFunc func() (chan map[string]interface{}, error)

type Broadcaster struct {
	sync.Mutex
	running bool
	subs    map[chan map[string]interface{}]struct{}
}

func (b *Broadcaster) Subscribe(ctx context.Context, connect ConnectFunc) (chan map[string]interface{}, error) {
	b.Lock()
	defer b.Unlock()

	if !b.running {
		if err := b.start(connect); err != nil {
			return nil, err
		}
	}

	sub := make(chan map[string]interface{}, 100)
	if b.subs == nil {
		b.subs = map[chan map[string]interface{}]struct{}{}
	}
	b.subs[sub] = struct{}{}
	go func() {
		<-ctx.Done()
		b.unsub(sub, true)
	}()

	return sub, nil
}

func (b *Broadcaster) unsub(sub chan map[string]interface{}, lock bool) {
	if lock {
		b.Lock()
	}
	if _, ok := b.subs[sub]; ok {
		close(sub)
		delete(b.subs, sub)
	}
	if lock {
		b.Unlock()
	}
}

func (b *Broadcaster) start(connect ConnectFunc) error {
	c, err := connect()
	if err != nil {
		return err
	}

	go b.stream(c)
	b.running = true
	return nil
}

func (b *Broadcaster) stream(input chan map[string]interface{}) {
	for item := range input {
		b.Lock()
		for sub := range b.subs {
			newItem := cloneMap(item)
			select {
			case sub <- newItem:
			default:
				// Slow consumer, drop
				go b.unsub(sub, true)
			}
		}
		b.Unlock()
	}

	b.Lock()
	for sub := range b.subs {
		b.unsub(sub, false)
	}
	b.running = false
	b.Unlock()
}

func cloneMap(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}

	result := map[string]interface{}{}
	for k, v := range data {
		result[k] = cloneValue(v)
	}

	return result
}

func cloneValue(v interface{}) interface{} {
	switch t := v.(type) {
	case []interface{}:
		return cloneSlice(t)
	case []map[string]interface{}:
		return cloneMapSlice(t)
	case map[string]interface{}:
		return cloneMap(t)
	default:
		return v
	}
}

func cloneMapSlice(data []map[string]interface{}) []interface{} {
	result := make([]interface{}, len(data))
	for i := range data {
		result[i] = cloneValue(data[i])
	}
	return result
}

func cloneSlice(data []interface{}) []interface{} {
	result := make([]interface{}, len(data))
	for i := range data {
		result[i] = cloneValue(data[i])
	}
	return result
}
