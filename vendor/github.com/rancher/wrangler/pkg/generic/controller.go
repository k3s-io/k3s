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

package generic

import (
	"context"
	errors2 "errors"
	"fmt"
	"strings"
	"time"

	errors3 "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

var ErrSkip = errors2.New("skip processing")

type Handler func(key string, obj runtime.Object) (runtime.Object, error)

// Controller is the controller implementation for Foo resources
type Controller struct {
	name      string
	workqueue workqueue.RateLimitingInterface
	informer  cache.SharedIndexInformer
	handler   Handler
	gvk       schema.GroupVersionKind
}

type ControllerMeta interface {
	Informer() cache.SharedIndexInformer
	GroupVersionKind() schema.GroupVersionKind

	AddGenericHandler(ctx context.Context, name string, handler Handler)
	AddGenericRemoveHandler(ctx context.Context, name string, handler Handler)
	Updater() Updater
}

// NewController returns a new sample controller
func NewController(gvk schema.GroupVersionKind, informer cache.SharedIndexInformer, workqueue workqueue.RateLimitingInterface, handler Handler) *Controller {
	controller := &Controller{
		name:      gvk.String(),
		handler:   handler,
		informer:  informer,
		workqueue: workqueue,
	}

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newMeta, err := meta.Accessor(new)
			utilruntime.Must(err)
			oldMeta, err := meta.Accessor(old)
			utilruntime.Must(err)
			if newMeta.GetResourceVersion() == oldMeta.GetResourceVersion() {
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}

func (c *Controller) Informer() cache.SharedIndexInformer {
	return c.informer
}

func (c *Controller) GroupVersionKind() schema.GroupVersionKind {
	return c.gvk
}

func (c *Controller) run(threadiness int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	logrus.Infof("Starting %s controller", c.name)

	// Launch two workers to process Foo resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	logrus.Infof("Shutting down %s workers", c.name)
}

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	if ok := cache.WaitForCacheSync(stopCh, c.informer.HasSynced); !ok {
		c.workqueue.ShutDown()
		return fmt.Errorf("failed to wait for caches to sync")
	}

	go c.run(threadiness, stopCh)
	return nil
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	if err := c.processSingleItem(obj); err != nil {
		if !strings.Contains(err.Error(), "please apply your changes to the latest version and try again") {
			utilruntime.HandleError(err)
		}
		return true
	}

	return true
}

func (c *Controller) processSingleItem(obj interface{}) error {
	var (
		key string
		ok  bool
	)

	defer c.workqueue.Done(obj)

	if key, ok = obj.(string); !ok {
		c.workqueue.Forget(obj)
		utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
		return nil
	}
	if err := c.syncHandler(key); err != nil && errors3.Cause(err) != ErrSkip {
		c.workqueue.AddRateLimited(key)
		return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
	}

	c.workqueue.Forget(obj)
	return nil
}

func (c *Controller) syncHandler(key string) error {
	obj, exists, err := c.informer.GetStore().GetByKey(key)
	if err != nil {
		return err
	}
	if !exists {
		_, err := c.handler(key, nil)
		return err
	}

	_, err = c.handler(key, obj.(runtime.Object))
	return err
}

func (c *Controller) Enqueue(namespace, name string) {
	if namespace == "" {
		c.workqueue.AddRateLimited(name)
	} else {
		c.workqueue.AddRateLimited(namespace + "/" + name)
	}
}

func (c *Controller) EnqueueAfter(namespace, name string, duration time.Duration) {
	if namespace == "" {
		c.workqueue.AddAfter(name, duration)
	} else {
		c.workqueue.AddAfter(namespace+"/"+name, duration)
	}
}

func (c *Controller) enqueue(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

func (c *Controller) handleObject(obj interface{}) {
	if _, ok := obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		_, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
	}
	c.enqueue(obj)
}
