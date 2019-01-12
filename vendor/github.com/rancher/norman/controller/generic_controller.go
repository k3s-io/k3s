package controller

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	errors2 "github.com/pkg/errors"
	"github.com/rancher/norman/metrics"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/types"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const MetricsQueueEnv = "NORMAN_QUEUE_METRICS"
const MetricsReflectorEnv = "NORMAN_REFLECTOR_METRICS"

var (
	resyncPeriod = 2 * time.Hour
)

// Override the metrics providers
func init() {
	if os.Getenv(MetricsQueueEnv) != "true" {
		DisableControllerWorkqueuMetrics()
	}
	if os.Getenv(MetricsReflectorEnv) != "true" {
		DisableControllerReflectorMetrics()
	}
}

type HandlerFunc func(key string, obj interface{}) (interface{}, error)

type GenericController interface {
	SetThreadinessOverride(count int)
	Informer() cache.SharedIndexInformer
	AddHandler(ctx context.Context, name string, handler HandlerFunc)
	HandlerCount() int
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type Backend interface {
	List(opts metav1.ListOptions) (runtime.Object, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	ObjectFactory() objectclient.ObjectFactory
}

type handlerDef struct {
	name       string
	generation int
	handler    HandlerFunc
}

type generationKey struct {
	generation int
	key        string
}

type genericController struct {
	sync.Mutex
	threadinessOverride int
	generation          int
	informer            cache.SharedIndexInformer
	handlers            []*handlerDef
	queue               workqueue.RateLimitingInterface
	name                string
	running             bool
	synced              bool
}

func NewGenericController(name string, genericClient Backend) GenericController {
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc:  genericClient.List,
			WatchFunc: genericClient.Watch,
		},
		genericClient.ObjectFactory().Object(), resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})

	rl := workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(500*time.Millisecond, 1000*time.Second),
		// 10 qps, 100 bucket size.  This is only for retry speed and its only the overall factor (not per item)
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
	)

	return &genericController{
		informer: informer,
		queue:    workqueue.NewNamedRateLimitingQueue(rl, name),
		name:     name,
	}
}

func (g *genericController) SetThreadinessOverride(count int) {
	g.threadinessOverride = count
}

func (g *genericController) HandlerCount() int {
	return len(g.handlers)
}

func (g *genericController) Informer() cache.SharedIndexInformer {
	return g.informer
}

func (g *genericController) Enqueue(namespace, name string) {
	if namespace == "" {
		g.queue.Add(name)
	} else {
		g.queue.Add(namespace + "/" + name)
	}
}

func (g *genericController) AddHandler(ctx context.Context, name string, handler HandlerFunc) {
	g.Lock()
	h := &handlerDef{
		name:       name,
		generation: g.generation,
		handler:    handler,
	}
	g.handlers = append(g.handlers, h)
	g.Unlock()

	go func() {
		<-ctx.Done()
		g.Lock()
		var handlers []*handlerDef
		for _, handler := range g.handlers {
			if handler != h {
				handlers = append(handlers, h)
			}
		}
		g.handlers = handlers
		g.Unlock()
	}()
}

func (g *genericController) Sync(ctx context.Context) error {
	g.Lock()
	defer g.Unlock()

	return g.sync(ctx)
}

func (g *genericController) sync(ctx context.Context) error {
	if g.synced {
		return nil
	}

	defer utilruntime.HandleCrash()

	g.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: g.queueObject,
		UpdateFunc: func(_, obj interface{}) {
			g.queueObject(obj)
		},
		DeleteFunc: g.queueObject,
	})

	logrus.Debugf("Syncing %s Controller", g.name)

	go g.informer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), g.informer.HasSynced) {
		return fmt.Errorf("failed to sync controller %s", g.name)
	}
	logrus.Debugf("Syncing %s Controller Done", g.name)

	g.synced = true
	return nil
}

func (g *genericController) Start(ctx context.Context, threadiness int) error {
	g.Lock()
	defer g.Unlock()

	if err := g.sync(ctx); err != nil {
		return err
	}

	if !g.running {
		if g.threadinessOverride > 0 {
			threadiness = g.threadinessOverride
		}
		go g.run(ctx, threadiness)
	}

	if g.running {
		for _, h := range g.handlers {
			if h.generation != g.generation {
				continue
			}
			for _, key := range g.informer.GetStore().ListKeys() {
				g.queueObject(generationKey{
					generation: g.generation,
					key:        key,
				})
			}
			break
		}
	}

	g.generation++
	g.running = true
	return nil
}

func (g *genericController) queueObject(obj interface{}) {
	if _, ok := obj.(generationKey); ok {
		g.queue.Add(obj)
		return
	}

	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err == nil {
		g.queue.Add(key)
	}
}

func (g *genericController) run(ctx context.Context, threadiness int) {
	defer utilruntime.HandleCrash()
	defer g.queue.ShutDown()

	for i := 0; i < threadiness; i++ {
		go wait.Until(g.runWorker, time.Second, ctx.Done())
	}

	<-ctx.Done()
	logrus.Infof("Shutting down %s controller", g.name)
}

func (g *genericController) runWorker() {
	for g.processNextWorkItem() {
	}
}

func (g *genericController) processNextWorkItem() bool {
	key, quit := g.queue.Get()
	if quit {
		return false
	}
	defer g.queue.Done(key)

	// do your work on the key.  This method will contains your "do stuff" logic
	err := g.syncHandler(key)
	checkErr := err
	if handlerErr, ok := checkErr.(*handlerError); ok {
		checkErr = handlerErr.err
	}
	if _, ok := checkErr.(*ForgetError); err == nil || ok {
		if ok {
			logrus.Debugf("%v %v completed with dropped err: %v", g.name, key, err)
		}
		g.queue.Forget(key)
		return true
	}

	if err := filterConflictsError(err); err != nil {
		logrus.Errorf("%v %v %v", g.name, key, err)
	}

	if gk, ok := key.(generationKey); ok {
		g.queue.AddRateLimited(gk.key)
	} else {
		g.queue.AddRateLimited(key)
	}

	return true
}

func ignoreError(err error, checkString bool) bool {
	err = errors2.Cause(err)
	if errors.IsConflict(err) {
		return true
	}
	if _, ok := err.(*ForgetError); ok {
		return true
	}
	if checkString {
		return strings.HasSuffix(err.Error(), "please apply your changes to the latest version and try again")
	}
	return false
}

func filterConflictsError(err error) error {
	if ignoreError(err, false) {
		return nil
	}

	if errs, ok := errors2.Cause(err).(*types.MultiErrors); ok {
		var newErrors []error
		for _, err := range errs.Errors {
			if !ignoreError(err, true) {
				newErrors = append(newErrors)
			}
		}
		return types.NewErrors(newErrors...)
	}

	return err
}

func (g *genericController) syncHandler(key interface{}) (err error) {
	defer utilruntime.RecoverFromPanic(&err)

	generation := -1
	var s string
	var obj interface{}

	switch v := key.(type) {
	case string:
		s = v
	case generationKey:
		generation = v.generation
		s = v.key
	default:
		return nil
	}

	obj, exists, err := g.informer.GetStore().GetByKey(s)
	if err != nil {
		return err
	} else if !exists {
		obj = nil
	}

	var errs []error
	for _, handler := range g.handlers {
		if generation > -1 && handler.generation != generation {
			continue
		}

		logrus.Debugf("%s calling handler %s %s", g.name, handler.name, s)
		metrics.IncTotalHandlerExecution(g.name, handler.name)
		if newObj, err := handler.handler(s, obj); err != nil {
			if !ignoreError(err, false) {
				metrics.IncTotalHandlerFailure(g.name, handler.name, s)
			}
			errs = append(errs, &handlerError{
				name: handler.name,
				err:  err,
			})
		} else if newObj != nil && !reflect.ValueOf(newObj).IsNil() {
			obj = newObj
		}
	}
	err = types.NewErrors(errs...)
	return
}

type handlerError struct {
	name string
	err  error
}

func (h *handlerError) Error() string {
	return fmt.Sprintf("[%s] failed with : %v", h.name, h.err)
}

func (h *handlerError) Cause() error {
	return h.err
}
