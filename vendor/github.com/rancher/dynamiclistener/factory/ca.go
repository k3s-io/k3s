package factory

import (
	"crypto/ecdsa"
	"crypto/x509"
	"io/ioutil"
	"os"
)

func GenCA() (*x509.Certificate, *ecdsa.PrivateKey, error) {
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

func LoadOrGenCA() (*x509.Certificate, *ecdsa.PrivateKey, error) {
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

func loadCA() (*x509.Certificate, *ecdsa.PrivateKey, error) {
	return LoadCerts("./certs/ca.pem", "./certs/ca.key")
}

func LoadCerts(certFile, keyFile string) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	caPem, err := ioutil.ReadFile(certFile)
	if err != nil {
		return nil, nil, err
	}
	caKey, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return nil, nil, err
	}

	key, err := ParseECPrivateKeyPEM(caKey)
	if err != nil {
		return nil, nil, err
	}

	cert, err := ParseCertPEM(caPem)
	if err != nil {
		return nil, nil, err
	}

	return cert, key, nil
}
