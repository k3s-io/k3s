package utils

import (
	"io"
	"sync"
)

type Listener interface {
	OnUpdate(instance interface{})
}

type ListenerFunc func(instance interface{})

func (f ListenerFunc) OnUpdate(instance interface{}) {
	f(instance)
}

// Broadcaster holds the details of registered listeners
type Broadcaster struct {
	listenerLock sync.RWMutex
	listeners    []Listener
}

// Add lets to register a listener
func (b *Broadcaster) Add(listener Listener) {
	b.listenerLock.Lock()
	defer b.listenerLock.Unlock()
	b.listeners = append(b.listeners, listener)
}

// Notify notifies an update to registered listeners
func (b *Broadcaster) Notify(instance interface{}) {
	b.listenerLock.RLock()
	listeners := b.listeners
	b.listenerLock.RUnlock()
	for _, listener := range listeners {
		go listener.OnUpdate(instance)
	}
}

// CloseCloserDisregardError it is a common need throughout kube-router's code base to need close a closer in defer
// statements, this allows an action like that to pass a linter as well as describe its intention well
func CloseCloserDisregardError(handler io.Closer) {
	_ = handler.Close()
}
