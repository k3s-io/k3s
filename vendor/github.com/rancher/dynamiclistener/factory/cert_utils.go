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
	"time"
)

const (
	CertificateBlockType = "CERTIFICATE"
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

	return x509.ParseCertificate(certDERBytes)
}

func NewSignedCert(signer crypto.Signer, caCert *x509.Certificate, caKey crypto.Signer, cn string, orgs []string,
	domains []string, ips []net.IP) (*x509.Certificate, error) {

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, err
	}

	parent := x509.Certificate{
		DNSNames:     domains,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  ips,
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		NotAfter:     time.Now().Add(time.Hour * 24 * 365).UTC(),
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

	return x509.ParseCertificate(cert)
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
