package generic

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type ControllerManager struct {
	lock        sync.Mutex
	generation  int
	started     map[schema.GroupVersionKind]bool
	controllers map[schema.GroupVersionKind]*Controller
	handlers    map[schema.GroupVersionKind]*Handlers
}

func (g *ControllerManager) Start(ctx context.Context, defaultThreadiness int, threadiness map[schema.GroupVersionKind]int) error {
	g.lock.Lock()
	defer g.lock.Unlock()

	for gvk, controller := range g.controllers {
		if g.started[gvk] {
			continue
		}

		threadiness, ok := threadiness[gvk]
		if !ok {
			threadiness = defaultThreadiness
		}
		if err := controller.Run(threadiness, ctx.Done()); err != nil {
			return err
		}

		if g.started == nil {
			g.started = map[schema.GroupVersionKind]bool{}
		}
		g.started[gvk] = true
	}

	return nil
}

func (g *ControllerManager) Enqueue(gvk schema.GroupVersionKind, informer cache.SharedIndexInformer, namespace, name string) {
	_, controller, _ := g.getController(gvk, informer, true)
	controller.Enqueue(namespace, name)
}

func (g *ControllerManager) removeHandler(gvk schema.GroupVersionKind, generation int) {
	g.lock.Lock()
	defer g.lock.Unlock()

	handlers, ok := g.handlers[gvk]
	if !ok {
		return
	}

	var newHandlers []handlerEntry
	for _, h := range handlers.handlers {
		if h.generation == generation {
			continue
		}
		newHandlers = append(newHandlers, h)
	}

	handlers.handlers = newHandlers
}

func (g *ControllerManager) getController(gvk schema.GroupVersionKind, informer cache.SharedIndexInformer, lock bool) (*Handlers, *Controller, bool) {
	if lock {
		g.lock.Lock()
		defer g.lock.Unlock()
	}

	if controller, ok := g.controllers[gvk]; ok {
		return g.handlers[gvk], controller, true
	}

	handlers := &Handlers{}

	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), gvk.String())
	controller := NewController(gvk.String(), informer, queue, handlers.Handle)

	if g.handlers == nil {
		g.handlers = map[schema.GroupVersionKind]*Handlers{}
	}

	if g.controllers == nil {
		g.controllers = map[schema.GroupVersionKind]*Controller{}
	}

	g.handlers[gvk] = handlers
	g.controllers[gvk] = controller

	return handlers, controller, false
}

func (g *ControllerManager) AddHandler(ctx context.Context, gvk schema.GroupVersionKind, informer cache.SharedIndexInformer, name string, handler Handler) {
	g.lock.Lock()
	defer g.lock.Unlock()

	g.generation++
	entry := handlerEntry{
		generation: g.generation,
		name:       name,
		handler:    handler,
	}

	go func() {
		<-ctx.Done()
		g.removeHandler(gvk, entry.generation)
	}()

	handlers, controller, ok := g.getController(gvk, informer, false)
	handlers.handlers = append(handlers.handlers, entry)

	if ok {
		for _, key := range controller.informer.GetStore().ListKeys() {
			controller.workqueue.Add(key)
		}
	}
}
