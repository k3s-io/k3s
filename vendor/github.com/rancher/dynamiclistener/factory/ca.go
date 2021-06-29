package factory

import (
	"crypto"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/rancher/dynamiclistener/cert"
)

func GenCA() (*x509.Certificate, crypto.Signer, error) {
	caKey, err := NewPrivateKey()
	if err != nil {
		return nil, nil, err
	}

	caCert, err := NewSelfSignedCACert(caKey, "dynamiclistener-ca", "dynamiclistener-org")
	if err != nil {
		return nil, nil, err
	}

	return caCert, caKey, nil
}

func LoadOrGenCA() (*x509.Certificate, crypto.Signer, error) {
	cert, key, err := loadCA()
	if err == nil {
		return cert, key, nil
	}

	cert, key, err = GenCA()
	if err != nil {
		return nil, nil, err
	}

	certBytes, keyBytes, err := Marshal(cert, key)
	if err != nil {
		return nil, nil, err
	}

	if err := os.MkdirAll("./certs", 0700); err != nil {
		return nil, nil, err
	}

	if err := ioutil.WriteFile("./certs/ca.pem", certBytes, 0600); err != nil {
		return nil, nil, err
	}

	if err := ioutil.WriteFile("./certs/ca.key", keyBytes, 0600); err != nil {
		return nil, nil, err
	}

	return cert, key, nil
}

func loadCA() (*x509.Certificate, crypto.Signer, error) {
	return LoadCerts("./certs/ca.pem", "./certs/ca.key")
}

func LoadCA(caPem, caKey []byte) (*x509.Certificate, crypto.Signer, error) {
	key, err := cert.ParsePrivateKeyPEM(caKey)
	if err != nil {
		return nil, nil, err
	}
	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, nil, fmt.Errorf("key is not a crypto.Signer")
	}

	cert, err := ParseCertPEM(caPem)
	if err != nil {
		return nil, nil, err
	}

	return cert, signer, nil
}

func LoadCerts(certFile, keyFile string) (*x509.Certificate, crypto.Signer, error) {
	caPem, err := ioutil.ReadFile(certFile)
	if err != nil {
		return nil, nil, err
	}
	caKey, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return nil, nil, err
	}

	return LoadCA(caPem, caKey)
}
