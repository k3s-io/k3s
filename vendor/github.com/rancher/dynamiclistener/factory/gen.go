package factory

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"net"
	"sort"
	"strings"

	v1 "k8s.io/api/core/v1"
)

const (
	cnPrefix = "listener.cattle.io/cn-"
	static   = "listener.cattle.io/static"
	hashKey  = "listener.cattle.io/hash"
)

type TLS struct {
	CACert       *x509.Certificate
	CAKey        crypto.Signer
	CN           string
	Organization []string
}

func collectCNs(secret *v1.Secret) (domains []string, ips []net.IP, hash string, err error) {
	var (
		cns    []string
		digest = sha256.New()
	)
	for k, v := range secret.Annotations {
		if strings.HasPrefix(k, cnPrefix) {
			cns = append(cns, v)
		}
	}

	sort.Strings(cns)

	for _, v := range cns {
		digest.Write([]byte(v))
		ip := net.ParseIP(v)
		if ip == nil {
			domains = append(domains, v)
		} else {
			ips = append(ips, ip)
		}
	}

	hash = hex.EncodeToString(digest.Sum(nil))
	return
}

func (t *TLS) AddCN(secret *v1.Secret, cn ...string) (*v1.Secret, bool, error) {
	var (
		err error
	)

	if !NeedsUpdate(secret, cn...) {
		return secret, false, nil
	}

	secret = populateCN(secret, cn...)

	privateKey, err := getPrivateKey(secret)
	if err != nil {
		return nil, false, err
	}

	domains, ips, hash, err := collectCNs(secret)
	if err != nil {
		return nil, false, err
	}

	newCert, err := t.newCert(domains, ips, privateKey)
	if err != nil {
		return nil, false, err
	}

	certBytes, keyBytes, err := Marshal(newCert, privateKey)
	if err != nil {
		return nil, false, err
	}

	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	secret.Data[v1.TLSCertKey] = certBytes
	secret.Data[v1.TLSPrivateKeyKey] = keyBytes
	secret.Annotations[hashKey] = hash

	return secret, true, nil
}

func (t *TLS) newCert(domains []string, ips []net.IP, privateKey *ecdsa.PrivateKey) (*x509.Certificate, error) {
	return NewSignedCert(privateKey, t.CACert, t.CAKey, t.CN, t.Organization, domains, ips)
}

func populateCN(secret *v1.Secret, cn ...string) *v1.Secret {
	secret = secret.DeepCopy()
	if secret.Annotations == nil {
		secret.Annotations = map[string]string{}
	}
	for _, cn := range cn {
		secret.Annotations[cnPrefix+cn] = cn
	}
	return secret
}

func NeedsUpdate(secret *v1.Secret, cn ...string) bool {
	if secret.Annotations[static] == "true" {
		return false
	}

	for _, cn := range cn {
		if secret.Annotations[cnPrefix+cn] == "" {
			return true
		}
	}

	return false
}

func getPrivateKey(secret *v1.Secret) (*ecdsa.PrivateKey, error) {
	keyBytes := secret.Data[v1.TLSPrivateKeyKey]
	if len(keyBytes) == 0 {
		return NewPrivateKey()
	}

	privateKey, err := ParseECPrivateKeyPEM(keyBytes)
	if err == nil {
		return privateKey, nil
	}

	return NewPrivateKey()
}

func Marshal(x509Cert *x509.Certificate, privateKey *ecdsa.PrivateKey) ([]byte, []byte, error) {
	certBlock := pem.Block{
		Type:  CertificateBlockType,
		Bytes: x509Cert.Raw,
	}

	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, nil, err
	}

	keyBlock := pem.Block{
		Type:  ECPrivateKeyBlockType,
		Bytes: keyBytes,
	}

	return pem.EncodeToMemory(&certBlock), pem.EncodeToMemory(&keyBlock), nil
}

func NewPrivateKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}
