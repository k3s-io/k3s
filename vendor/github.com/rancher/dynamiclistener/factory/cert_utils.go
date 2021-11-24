package factory

import (
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math"
	"math/big"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	CertificateBlockType               = "CERTIFICATE"
	defaultNewSignedCertExpirationDays = 365
)

func NewSelfSignedCACert(key crypto.Signer, cn string, org ...string) (*x509.Certificate, error) {
	now := time.Now()
	tmpl := x509.Certificate{
		BasicConstraintsValid: true,
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		NotAfter:              now.Add(time.Hour * 24 * 365 * 10).UTC(),
		NotBefore:             now.UTC(),
		SerialNumber:          new(big.Int).SetInt64(0),
		Subject: pkix.Name{
			CommonName:   cn,
			Organization: org,
		},
	}

	certDERBytes, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, key.Public(), key)
	if err != nil {
		return nil, err
	}

	logrus.Infof("generated self-signed CA certificate %s: notBefore=%s notAfter=%s",
		tmpl.Subject, tmpl.NotBefore, tmpl.NotAfter)

	return x509.ParseCertificate(certDERBytes)
}

func NewSignedClientCert(signer crypto.Signer, caCert *x509.Certificate, caKey crypto.Signer, cn string) (*x509.Certificate, error) {
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, err
	}

	parent := x509.Certificate{
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		NotAfter:     time.Now().Add(time.Hour * 24 * 365).UTC(),
		NotBefore:    caCert.NotBefore,
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: cn,
		},
	}

	parts := strings.Split(cn, ",o=")
	if len(parts) > 1 {
		parent.Subject.CommonName = parts[0]
		parent.Subject.Organization = parts[1:]
	}

	cert, err := x509.CreateCertificate(rand.Reader, &parent, caCert, signer.Public(), caKey)
	if err != nil {
		return nil, err
	}

	return x509.ParseCertificate(cert)
}

func NewSignedCert(signer crypto.Signer, caCert *x509.Certificate, caKey crypto.Signer, cn string, orgs []string,
	domains []string, ips []net.IP) (*x509.Certificate, error) {

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, err
	}

	expirationDays := defaultNewSignedCertExpirationDays
	envExpirationDays := os.Getenv("CATTLE_NEW_SIGNED_CERT_EXPIRATION_DAYS")
	if envExpirationDays != "" {
		if envExpirationDaysInt, err := strconv.Atoi(envExpirationDays); err != nil {
			logrus.Infof("[NewSignedCert] expiration days from ENV (%s) could not be converted to int (falling back to default value: %d)", envExpirationDays, defaultNewSignedCertExpirationDays)
		} else {
			expirationDays = envExpirationDaysInt
		}
	}

	parent := x509.Certificate{
		DNSNames:     domains,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  ips,
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		NotAfter:     time.Now().Add(time.Hour * 24 * time.Duration(expirationDays)).UTC(),
		NotBefore:    caCert.NotBefore,
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   cn,
			Organization: orgs,
		},
	}

	cert, err := x509.CreateCertificate(rand.Reader, &parent, caCert, signer.Public(), caKey)
	if err != nil {
		return nil, err
	}

	parsedCert, err := x509.ParseCertificate(cert)
	if err == nil {
		logrus.Infof("certificate %s signed by %s: notBefore=%s notAfter=%s",
			parsedCert.Subject, caCert.Subject, parsedCert.NotBefore, parsedCert.NotAfter)
	}
	return parsedCert, err
}

func ParseCertPEM(pemCerts []byte) (*x509.Certificate, error) {
	var pemBlock *pem.Block
	for {
		pemBlock, pemCerts = pem.Decode(pemCerts)
		if pemBlock == nil {
			break
		}

		if pemBlock.Type == CertificateBlockType {
			return x509.ParseCertificate(pemBlock.Bytes)
		}
	}

	return nil, fmt.Errorf("pem does not include a valid x509 cert")
}
