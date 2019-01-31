package changeset

import (
	"context"
	"strings"

	"github.com/rancher/norman/controller"
	"k8s.io/apimachinery/pkg/runtime"
)

type Key struct {
	Namespace string
	Name      string
}

type ControllerProvider interface {
	Generic() controller.GenericController
}

type Enqueuer interface {
	Enqueue(namespace, name string)
}

type Resolver func(namespace, name string, obj runtime.Object) ([]Key, error)

func Watch(ctx context.Context, name string, resolve Resolver, enq Enqueuer, controllers ...ControllerProvider) {
	for _, c := range controllers {
		watch(ctx, name, enq, resolve, c.Generic())
	}
}

func watch(ctx context.Context, name string, enq Enqueuer, resolve Resolver, genericController controller.GenericController) {
	genericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		obj, exists, err := genericController.Informer().GetStore().GetByKey(key)
		if err != nil {
			return nil, err
		}

		if !exists {
			obj = nil
		}

		var (
			ns   string
			name string
		)

		parts := strings.SplitN(key, "/", 2)
		if len(parts) == 2 {
			ns = parts[0]
			name = parts[1]
		} else {
			name = parts[0]
		}

		ro, ok := obj.(runtime.Object)
		if !ok {
			ro = nil
		}

		keys, err := resolve(ns, name, ro)
		if err != nil {
			return nil, err
		}

		for _, key := range keys {
			if key.Name != "" {
				enq.Enqueue(key.Namespace, key.Name)
			}
		}

		return nil, nil
	})
}
