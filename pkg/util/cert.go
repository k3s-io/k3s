package util

import (
	"crypto/x509"
	"time"

	certutil "github.com/rancher/dynamiclistener/cert"
)

// cert usage constants
const (
	CertUsageCertSign   = "CertSign"
	CertUsageServerAuth = "ServerAuth"
	CertUsageClientAuth = "ClientAuth"
	CertUsageUnknown    = "Unknown"
)

// cert status constants
const (
	CertStatusOK          = "OK"
	CertStatusWarning     = "WARNING"
	CertStatusExpired     = "EXPIRED"
	CertStatusNotYetValid = "NOT YET VALID"
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

// GetCertUsages returns a slice of strings representing the certificate usages
func GetCertUsages(cert *x509.Certificate) []string {
	usages := []string{}
	if cert.KeyUsage&x509.KeyUsageCertSign != 0 {
		usages = append(usages, CertUsageCertSign)
	}
	for _, eku := range cert.ExtKeyUsage {
		switch eku {
		case x509.ExtKeyUsageServerAuth:
			usages = append(usages, CertUsageServerAuth)
		case x509.ExtKeyUsageClientAuth:
			usages = append(usages, CertUsageClientAuth)
		}
	}
	if len(usages) == 0 {
		usages = append(usages, CertUsageUnknown)
	}
	return usages
}

// GetCertStatus determines the status of a certificate based on its validity period
func GetCertStatus(cert *x509.Certificate, now time.Time, warn time.Time) string {
	if now.Before(cert.NotBefore) {
		return CertStatusNotYetValid
	} else if now.After(cert.NotAfter) {
		return CertStatusExpired
	} else if warn.After(cert.NotAfter) {
		return CertStatusWarning
	}
	return CertStatusOK
}
