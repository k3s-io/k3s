package util

import (
	"crypto/x509"

	certutil "github.com/rancher/dynamiclistener/cert"
)

// EncodeCertsPEM is a wrapper around the EncodeCertPEM function to return the
// PEM encoding of a cert and chain, instead of just a single cert.
func EncodeCertsPEM(cert *x509.Certificate, caCerts []*x509.Certificate) []byte {
	pemBytes := certutil.EncodeCertPEM(cert)
	for _, caCert := range caCerts {
		pemBytes = append(pemBytes, certutil.EncodeCertPEM(caCert)...)
	}
	return pemBytes
}
