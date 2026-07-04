// Package dynacert provides dynamiclistener certificate cache helpers for k3s.
package dynacert

import (
	"os"

	"github.com/rancher/dynamiclistener"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

// RecoveringStorage wraps a TLSStorage so a corrupt or unreadable cache entry is
// removed and treated as empty (same as a missing file). This lets dynamiclistener
// regenerate certificates instead of failing every handshake when
// dynamic-cert.json is corrupt (k3s-io/k3s#14220).
type RecoveringStorage struct {
	Path  string
	Inner dynamiclistener.TLSStorage
}

func (s *RecoveringStorage) Get() (*corev1.Secret, error) {
	secret, err := s.Inner.Get()
	if err != nil {
		logrus.Warnf("Removing unreadable dynamic listener certificate cache %s: %v", s.Path, err)
		if rmErr := os.Remove(s.Path); rmErr != nil && !os.IsNotExist(rmErr) {
			logrus.Warnf("Failed to remove unreadable dynamic listener certificate cache %s: %v", s.Path, rmErr)
		}
		return nil, nil
	}
	return secret, nil
}

func (s *RecoveringStorage) Update(secret *corev1.Secret) error {
	return s.Inner.Update(secret)
}
