package kubernetes

import (
	"context"
	"sync"
	"time"

	"github.com/rancher/dynamiclistener"
	"github.com/rancher/wrangler-api/pkg/generated/controllers/core"
	v1controller "github.com/rancher/wrangler-api/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/start"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
)

type CoreGetter func() *core.Factory

func New(ctx context.Context, core CoreGetter, namespace, name string, backing dynamiclistener.TLSStorage) dynamiclistener.TLSStorage {
	storage := &storage{
		name:      name,
		namespace: namespace,
		storage:   backing,
		ctx:       ctx,
	}

	// lazy init
	go func() {
		for {
			core := core()
			if core != nil {
				storage.init(core.Core().V1().Secret())
				start.All(ctx, 5, core)
				return
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}()

	return storage
}

type storage struct {
	sync.Mutex

	namespace, name string
	storage         dynamiclistener.TLSStorage
	secrets         v1controller.SecretClient
	ctx             context.Context
}

func (s *storage) init(secrets v1controller.SecretController) {
	s.Lock()
	defer s.Unlock()

	secrets.OnChange(s.ctx, "tls-storage", func(key string, secret *v1.Secret) (*v1.Secret, error) {
		if secret == nil {
			return nil, nil
		}
		if secret.Namespace == s.namespace && secret.Name == s.name {
			if err := s.Update(secret); err != nil {
				return nil, err
			}
		}

		return secret, nil
	})
	s.secrets = secrets
}

func (s *storage) Get() (*v1.Secret, error) {
	s.Lock()
	defer s.Unlock()

	return s.storage.Get()
}

func (s *storage) Update(secret *v1.Secret) (err error) {
	s.Lock()
	defer s.Unlock()

	if s.secrets != nil {
		if secret.UID == "" {
			secret.Name = s.name
			secret.Namespace = s.namespace
			secret, err = s.secrets.Create(secret)
			if err != nil {
				return err
			}
		} else {
			existingSecret, err := s.storage.Get()
			if err != nil {
				return err
			}
			if !equality.Semantic.DeepEqual(secret.Data, existingSecret.Data) {
				secret, err = s.secrets.Update(secret)
				if err != nil {
					return err
				}
			}
		}
	}

	return s.storage.Update(secret)
}
