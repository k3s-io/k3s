package tls

import (
	"context"

	"github.com/rancher/dynamiclistener"
	v1 "github.com/rancher/k3s/pkg/apis/k3s.cattle.io/v1"
	k3sclient "github.com/rancher/k3s/pkg/generated/controllers/k3s.cattle.io/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ns   = "kube-system"
	name = "tls-config"
)

func NewServer(ctx context.Context, listenerConfigs k3sclient.ListenerConfigController, config dynamiclistener.UserConfig) (dynamiclistener.ServerInterface, error) {
	storage := &listenerConfigStorage{
		client: listenerConfigs,
		cache:  listenerConfigs.Cache(),
	}

	server, err := dynamiclistener.NewServer(storage, config)
	if err != nil {
		return nil, err
	}

	listenerConfigs.OnChange(ctx, "listen-config", func(key string, obj *v1.ListenerConfig) (*v1.ListenerConfig, error) {
		if obj == nil {
			return nil, nil
		}
		return obj, server.Update(fromStorage(obj))
	})

	return server, err
}

type listenerConfigStorage struct {
	cache  k3sclient.ListenerConfigCache
	client k3sclient.ListenerConfigClient
}

func (l *listenerConfigStorage) Set(config *dynamiclistener.ListenerStatus) (*dynamiclistener.ListenerStatus, error) {
	if config == nil {
		return nil, nil
	}

	obj, err := l.cache.Get(ns, name)
	if errors.IsNotFound(err) {
		ls := v1.NewListenerConfig(ns, name, v1.ListenerConfig{
			Status: *config,
		})

		ls, err := l.client.Create(ls)
		return fromStorage(ls), err
	} else if err != nil {
		return nil, err
	}

	obj = obj.DeepCopy()
	obj.ResourceVersion = config.Revision
	obj.Status = *config
	obj.Status.Revision = ""

	obj, err = l.client.Update(obj)
	return fromStorage(obj), err
}

func (l *listenerConfigStorage) Get() (*dynamiclistener.ListenerStatus, error) {
	obj, err := l.cache.Get(ns, name)
	if errors.IsNotFound(err) {
		obj, err = l.client.Get(ns, name, metav1.GetOptions{})
	}
	if errors.IsNotFound(err) {
		return &dynamiclistener.ListenerStatus{}, nil
	}
	return fromStorage(obj), err
}

func fromStorage(obj *v1.ListenerConfig) *dynamiclistener.ListenerStatus {
	if obj == nil {
		return nil
	}

	copy := obj.DeepCopy()
	copy.Status.Revision = obj.ResourceVersion
	return &copy.Status
}
