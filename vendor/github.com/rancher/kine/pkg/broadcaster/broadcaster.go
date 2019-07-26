package broadcaster

import (
	"context"
	"sync"
)

type ConnectFunc func() (chan interface{}, error)

type Broadcaster struct {
	sync.Mutex
	running bool
	subs    map[chan interface{}]struct{}
}

func (b *Broadcaster) Subscribe(ctx context.Context, connect ConnectFunc) (<-chan interface{}, error) {
	b.Lock()
	defer b.Unlock()

	if !b.running {
		if err := b.start(connect); err != nil {
			return nil, err
		}
	}

	sub := make(chan interface{}, 100)
	if b.subs == nil {
		b.subs = map[chan interface{}]struct{}{}
	}
	b.subs[sub] = struct{}{}
	go func() {
		<-ctx.Done()
		b.unsub(sub, true)
	}()

	return sub, nil
}

func (b *Broadcaster) unsub(sub chan interface{}, lock bool) {
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

func (b *Broadcaster) stream(input chan interface{}) {
	for item := range input {
		b.Lock()
		for sub := range b.subs {
			select {
			case sub <- item:
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
