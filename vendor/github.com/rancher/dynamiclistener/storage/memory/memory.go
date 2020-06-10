package memory

import (
	"github.com/rancher/dynamiclistener"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

func New() dynamiclistener.TLSStorage {
	return &memory{}
}

func NewBacked(storage dynamiclistener.TLSStorage) dynamiclistener.TLSStorage {
	return &memory{storage: storage}
}

type memory struct {
	storage dynamiclistener.TLSStorage
	secret  *v1.Secret
}

func (m *memory) Get() (*v1.Secret, error) {
	if m.secret == nil && m.storage != nil {
		secret, err := m.storage.Get()
		if err != nil {
			return nil, err
		}
		m.secret = secret
	}

	return m.secret, nil
}

func (m *memory) Update(secret *v1.Secret) error {
	if m.storage != nil {
		if err := m.storage.Update(secret); err != nil {
			return err
		}
	}

	logrus.Infof("Active TLS secret %s (ver=%s) (count %d): %v", secret.Name, secret.ResourceVersion, len(secret.Annotations)-1, secret.Annotations)
	m.secret = secret
	return nil
}
