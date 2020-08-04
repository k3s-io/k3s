package kubernetes

import (
	"crypto"
	"crypto/x509"

	"github.com/rancher/dynamiclistener/factory"
	v1controller "github.com/rancher/wrangler-api/pkg/generated/controllers/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func LoadOrGenCA(secrets v1controller.SecretClient, namespace, name string) (*x509.Certificate, crypto.Signer, error) {
	secret, err := getSecret(secrets, namespace, name)
	if err != nil {
		return nil, nil, err
	}
	return factory.LoadCA(secret.Data[v1.TLSCertKey], secret.Data[v1.TLSPrivateKeyKey])
}

func getSecret(secrets v1controller.SecretClient, namespace, name string) (*v1.Secret, error) {
	s, err := secrets.Get(namespace, name, metav1.GetOptions{})
	if !errors.IsNotFound(err) {
		return s, err
	}

	if err := createAndStore(secrets, namespace, name); err != nil {
		return nil, err
	}
	return secrets.Get(namespace, name, metav1.GetOptions{})
}

func createAndStore(secrets v1controller.SecretClient, namespace string, name string) error {
	ca, cert, err := factory.GenCA()
	if err != nil {
		return err
	}

	certPem, keyPem, err := factory.Marshal(ca, cert)
	if err != nil {
		return err
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			v1.TLSCertKey:       certPem,
			v1.TLSPrivateKeyKey: keyPem,
		},
		Type: v1.SecretTypeTLS,
	}

	secrets.Create(secret)
	return nil
}
