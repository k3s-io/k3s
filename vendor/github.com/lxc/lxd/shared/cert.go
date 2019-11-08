// http://golang.org/src/pkg/crypto/tls/generate_cert.go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package shared

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"time"
)

// KeyPairAndCA returns a CertInfo object with a reference to the key pair and
// (optionally) CA certificate located in the given directory and having the
// given name prefix
//
// The naming conversion for the various files is:
//
// <prefix>.crt -> public key
// <prefix>.key -> private key
// <prefix>.ca -> CA certificate
//
// If no public/private key files are found, a new key pair will be generated
// and saved on disk.
//
// If a CA certificate is found, it will be returned as well as second return
// value (otherwise it will be nil).
func KeyPairAndCA(dir, prefix string, kind CertKind) (*CertInfo, error) {
	certFilename := filepath.Join(dir, prefix+".crt")
	keyFilename := filepath.Join(dir, prefix+".key")

	// Ensure that the certificate exists, or create a new one if it does
	// not.
	err := FindOrGenCert(certFilename, keyFilename, kind == CertClient)
	if err != nil {
		return nil, err
	}

	// Load the certificate.
	keypair, err := tls.LoadX509KeyPair(certFilename, keyFilename)
	if err != nil {
		return nil, err
	}

	// If available, load the CA data as well.
	caFilename := filepath.Join(dir, prefix+".ca")
	var ca *x509.Certificate
	if PathExists(caFilename) {
		ca, err = ReadCert(caFilename)
		if err != nil {
			return nil, err
		}
	}

	info := &CertInfo{
		keypair: keypair,
		ca:      ca,
	}
	return info, nil
}

// CertInfo captures TLS certificate information about a certain public/private
// keypair and an optional CA certificate.
//
// Given LXD's support for PKI setups, these two bits of information are
// normally used and passed around together, so this structure helps with that
// (see doc/security.md for more details).
type CertInfo struct {
	keypair tls.Certificate
	ca      *x509.Certificate
}

// KeyPair returns the public/private key pair.
func (c *CertInfo) KeyPair() tls.Certificate {
	return c.keypair
}

// CA returns the CA certificate.
func (c *CertInfo) CA() *x509.Certificate {
	return c.ca
}

// PublicKey is a convenience to encode the underlying public key to ASCII.
func (c *CertInfo) PublicKey() []byte {
	data := c.KeyPair().Certificate[0]
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: data})
}

// PrivateKey is a convenience to encode the underlying private key.
func (c *CertInfo) PrivateKey() []byte {
	ecKey, ok := c.KeyPair().PrivateKey.(*ecdsa.PrivateKey)
	if ok {
		data, err := x509.MarshalECPrivateKey(ecKey)
		if err != nil {
			return nil
		}

		return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: data})
	}

	rsaKey, ok := c.KeyPair().PrivateKey.(*rsa.PrivateKey)
	if ok {
		data := x509.MarshalPKCS1PrivateKey(rsaKey)
		return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: data})
	}

	return nil
}

// Fingerprint returns the fingerprint of the public key.
func (c *CertInfo) Fingerprint() string {
	fingerprint, err := CertFingerprintStr(string(c.PublicKey()))
	// Parsing should never fail, since we generated the cert ourselves,
	// but let's check the error for good measure.
	if err != nil {
		panic("invalid public key material")
	}
	return fingerprint
}

// CertKind defines the kind of certificate to generate from scratch in
// KeyPairAndCA when it's not there.
//
// The two possible kinds are client and server, and they differ in the
// ext-key-usage bitmaps. See GenerateMemCert for more details.
type CertKind int

// Possible kinds of certificates.
const (
	CertClient CertKind = iota
	CertServer
)

// TestingKeyPair returns CertInfo object initialized with a test keypair. It's
// meant to be used only by tests.
func TestingKeyPair() *CertInfo {
	keypair, err := tls.X509KeyPair(testCertPEMBlock, testKeyPEMBlock)
	if err != nil {
		panic(fmt.Sprintf("invalid X509 keypair material: %v", err))
	}
	cert := &CertInfo{
		keypair: keypair,
	}
	return cert
}

// TestingAltKeyPair returns CertInfo object initialized with a test keypair
// which differs from the one returned by TestCertInfo. It's meant to be used
// only by tests.
func TestingAltKeyPair() *CertInfo {
	keypair, err := tls.X509KeyPair(testAltCertPEMBlock, testAltKeyPEMBlock)
	if err != nil {
		panic(fmt.Sprintf("invalid X509 keypair material: %v", err))
	}
	cert := &CertInfo{
		keypair: keypair,
	}
	return cert
}

/*
 * Generate a list of names for which the certificate will be valid.
 * This will include the hostname and ip address
 */
func mynames() ([]string, error) {
	h, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	ret := []string{h}

	ifs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifs {
		if IsLoopback(&iface) {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}

		for _, addr := range addrs {
			ret = append(ret, addr.String())
		}
	}

	return ret, nil
}

func FindOrGenCert(certf string, keyf string, certtype bool) error {
	if PathExists(certf) && PathExists(keyf) {
		return nil
	}

	/* If neither stat succeeded, then this is our first run and we
	 * need to generate cert and privkey */
	err := GenCert(certf, keyf, certtype)
	if err != nil {
		return err
	}

	return nil
}

// GenCert will create and populate a certificate file and a key file
func GenCert(certf string, keyf string, certtype bool) error {
	/* Create the basenames if needed */
	dir := path.Dir(certf)
	err := os.MkdirAll(dir, 0750)
	if err != nil {
		return err
	}
	dir = path.Dir(keyf)
	err = os.MkdirAll(dir, 0750)
	if err != nil {
		return err
	}

	certBytes, keyBytes, err := GenerateMemCert(certtype)
	if err != nil {
		return err
	}

	certOut, err := os.Create(certf)
	if err != nil {
		return fmt.Errorf("Failed to open %s for writing: %v", certf, err)
	}
	certOut.Write(certBytes)
	certOut.Close()

	keyOut, err := os.OpenFile(keyf, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("Failed to open %s for writing: %v", keyf, err)
	}
	keyOut.Write(keyBytes)
	keyOut.Close()
	return nil
}

// GenerateMemCert creates client or server certificate and key pair,
// returning them as byte arrays in memory.
func GenerateMemCert(client bool) ([]byte, []byte, error) {
	privk, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to generate key: %v", err)
	}

	hosts, err := mynames()
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to get my hostname: %v", err)
	}

	validFrom := time.Now()
	validTo := validFrom.Add(10 * 365 * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to generate serial number: %v", err)
	}

	userEntry, err := user.Current()
	var username string
	if err == nil {
		username = userEntry.Username
		if username == "" {
			username = "UNKNOWN"
		}
	} else {
		username = "UNKNOWN"
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "UNKNOWN"
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"linuxcontainers.org"},
			CommonName:   fmt.Sprintf("%s@%s", username, hostname),
		},
		NotBefore: validFrom,
		NotAfter:  validTo,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	if client {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	} else {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	}

	for _, h := range hosts {
		if ip, _, err := net.ParseCIDR(h); err == nil {
			if !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() {
				template.IPAddresses = append(template.IPAddresses, ip)
			}
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privk.PublicKey, privk)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to create certificate: %v", err)
	}

	data, err := x509.MarshalECPrivateKey(privk)
	if err != nil {
		return nil, nil, err
	}

	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	key := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: data})

	return cert, key, nil
}

func ReadCert(fpath string) (*x509.Certificate, error) {
	cf, err := ioutil.ReadFile(fpath)
	if err != nil {
		return nil, err
	}

	certBlock, _ := pem.Decode(cf)
	if certBlock == nil {
		return nil, fmt.Errorf("Invalid certificate file")
	}

	return x509.ParseCertificate(certBlock.Bytes)
}

func CertFingerprint(cert *x509.Certificate) string {
	return fmt.Sprintf("%x", sha256.Sum256(cert.Raw))
}

func CertFingerprintStr(c string) (string, error) {
	pemCertificate, _ := pem.Decode([]byte(c))
	if pemCertificate == nil {
		return "", fmt.Errorf("invalid certificate")
	}

	cert, err := x509.ParseCertificate(pemCertificate.Bytes)
	if err != nil {
		return "", err
	}

	return CertFingerprint(cert), nil
}

func GetRemoteCertificate(address string) (*x509.Certificate, error) {
	// Setup a permissive TLS config
	tlsConfig, err := GetTLSConfig("", "", "", nil)
	if err != nil {
		return nil, err
	}

	tlsConfig.InsecureSkipVerify = true

	// Support disabling of strict ciphers
	if IsTrue(os.Getenv("LXD_INSECURE_TLS")) {
		tlsConfig.CipherSuites = nil
	}

	tr := &http.Transport{
		TLSClientConfig: tlsConfig,
		Dial:            RFC3493Dialer,
		Proxy:           ProxyFromEnvironment,
	}

	// Connect
	client := &http.Client{Transport: tr}
	resp, err := client.Get(address)
	if err != nil {
		return nil, err
	}

	// Retrieve the certificate
	if resp.TLS == nil || len(resp.TLS.PeerCertificates) == 0 {
		return nil, fmt.Errorf("Unable to read remote TLS certificate")
	}

	return resp.TLS.PeerCertificates[0], nil
}

var testCertPEMBlock = []byte(`-----BEGIN CERTIFICATE-----
MIIFzjCCA7igAwIBAgIRAKnCQRdpkZ86oXYOd9hGrPgwCwYJKoZIhvcNAQELMB4x
HDAaBgNVBAoTE2xpbnV4Y29udGFpbmVycy5vcmcwHhcNMTUwNzE1MDQ1NjQ0WhcN
MjUwNzEyMDQ1NjQ0WjAeMRwwGgYDVQQKExNsaW51eGNvbnRhaW5lcnMub3JnMIIC
IjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAyViJkCzoxa1NYilXqGJog6xz
lSm4xt8KIzayc0JdB9VxEdIVdJqUzBAUtyCS4KZ9MbPmMEOX9NbBASL0tRK58/7K
Scq99Kj4XbVMLU1P/y5aW0ymnF0OpKbG6unmgAI2k/duRlbYHvGRdhlswpKl0Yst
l8i2kXOK0Rxcz90FewcEXGSnIYW21sz8YpBLfIZqOx6XEV36mOdi3MLrhUSAhXDw
Pay33Y7NonCQUBtiO7BT938cqI14FJrWdKon1UnODtzONcVBLTWtoe7D41+mx7EE
Taq5OPxBSe0DD6KQcPOZ7ZSJEhIqVKMvzLyiOJpyShmhm4OuGNoAG6jAuSij/9Kc
aLU4IitcrvFOuAo8M9OpiY9ZCR7Gb/qaPAXPAxE7Ci3f9DDNKXtPXDjhj3YG01+h
fNXMW3kCkMImn0A/+mZUMdCL87GWN2AN3Do5qaIc5XVEt1gp+LVqJeMoZ/lAeZWT
IbzcnkneOzE25m+bjw3r3WlR26amhyrWNwjGzRkgfEpw336kniX/GmwaCNgdNk+g
5aIbVxIHO0DbgkDBtdljR3VOic4djW/LtUIYIQ2egnPPyRR3fcFI+x5EQdVQYUXf
jpGIwovUDyG0Lkam2tpdeEXvLMZr8+Lhzu+H6vUFSj3cz6gcw/Xepw40FOkYdAI9
LYB6nwpZLTVaOqZCJ2ECAwEAAaOCAQkwggEFMA4GA1UdDwEB/wQEAwIAoDATBgNV
HSUEDDAKBggrBgEFBQcDATAMBgNVHRMBAf8EAjAAMIHPBgNVHREEgccwgcSCCVVi
dW50dVByb4IRMTAuMTY3LjE2MC4xODMvMjSCHzIwMDE6MTVjMDo2NzM1OmVlMDA6
OmU6ZTMxMy8xMjiCKWZkNTc6Yzg3ZDpmMWVlOmVlMDA6MjFkOjdkZmY6ZmUwOToz
NzUzLzY0gikyMDAxOjE1YzA6NjczNTplZTAwOjIxZDo3ZGZmOmZlMDk6Mzc1My82
NIIbZmU4MDo6MjFkOjdkZmY6ZmUwOTozNzUzLzY0ghAxOTIuMTY4LjEyMi4xLzI0
MAsGCSqGSIb3DQEBCwOCAgEAmcJUSBH7cLw3auEEV1KewtdqY1ARVB/pafAtbe9F
7ZKBbxUcS7cP3P1hRs5FH1bH44bIJKHxckctNUPqvC+MpXSryKinQ5KvGPNjGdlW
6EPlQr23btizC6hRdQ6RjEkCnQxhyTLmQ9n78nt47hjA96rFAhCUyfPdv9dI4Zux
bBTJekhCx5taamQKoxr7tql4Y2TchVlwASZvOfar8I0GxBRFT8w9IjckOSLoT9/s
OhlvXpeoxxFT7OHwqXEXdRUvw/8MGBo6JDnw+J/NGDBw3Z0goebG4FMT//xGSHia
czl3A0M0flk4/45L7N6vctwSqi+NxVaJRKeiYPZyzOO9K/d+No+WVBPwKmyP8icQ
b7FGTelPJOUolC6kmoyM+vyaNUoU4nz6lgOSHAtuqGNDWZWuX/gqzZw77hzDIgkN
qisOHZWPVlG/iUh1JBkbglBaPeaa3zf0XwSdgwwf4v8Z+YtEiRqkuFgQY70eQKI/
CIkj1p0iW5IBEsEAGUGklz4ZwqJwH3lQIqDBzIgHe3EP4cXaYsx6oYhPSDdHLPv4
HMZhl05DP75CEkEWRD0AIaL7SHdyuYUmCZ2zdrMI7TEDrAqcUuPbYpHcdJ2wnYmi
2G8XHJibfu4PCpIm1J8kPL8rqpdgW3moKR8Mp0HJQOH4tSBr1Ep7xNLP1wg6PIe+
p7U=
-----END CERTIFICATE-----
`)

var testKeyPEMBlock = []byte(`-----BEGIN RSA PRIVATE KEY-----
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
-----END RSA PRIVATE KEY-----
`)

var testAltCertPEMBlock = []byte(`-----BEGIN CERTIFICATE-----
MIICEzCCAXygAwIBAgIQMIMChMLGrR+QvmQvpwAU6zANBgkqhkiG9w0BAQsFADAS
MRAwDgYDVQQKEwdBY21lIENvMCAXDTcwMDEwMTAwMDAwMFoYDzIwODQwMTI5MTYw
MDAwWjASMRAwDgYDVQQKEwdBY21lIENvMIGfMA0GCSqGSIb3DQEBAQUAA4GNADCB
iQKBgQDuLnQAI3mDgey3VBzWnB2L39JUU4txjeVE6myuDqkM/uGlfjb9SjY1bIw4
iA5sBBZzHi3z0h1YV8QPuxEbi4nW91IJm2gsvvZhIrCHS3l6afab4pZBl2+XsDul
rKBxKKtD1rGxlG4LjncdabFn9gvLZad2bSysqz/qTAUStTvqJQIDAQABo2gwZjAO
BgNVHQ8BAf8EBAMCAqQwEwYDVR0lBAwwCgYIKwYBBQUHAwEwDwYDVR0TAQH/BAUw
AwEB/zAuBgNVHREEJzAlggtleGFtcGxlLmNvbYcEfwAAAYcQAAAAAAAAAAAAAAAA
AAAAATANBgkqhkiG9w0BAQsFAAOBgQCEcetwO59EWk7WiJsG4x8SY+UIAA+flUI9
tyC4lNhbcF2Idq9greZwbYCqTTTr2XiRNSMLCOjKyI7ukPoPjo16ocHj+P3vZGfs
h1fIw3cSS2OolhloGw/XM6RWPWtPAlGykKLciQrBru5NAPvCMsb/I1DAceTiotQM
fblo6RBxUQ==
-----END CERTIFICATE-----`)

var testAltKeyPEMBlock = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIICXgIBAAKBgQDuLnQAI3mDgey3VBzWnB2L39JUU4txjeVE6myuDqkM/uGlfjb9
SjY1bIw4iA5sBBZzHi3z0h1YV8QPuxEbi4nW91IJm2gsvvZhIrCHS3l6afab4pZB
l2+XsDulrKBxKKtD1rGxlG4LjncdabFn9gvLZad2bSysqz/qTAUStTvqJQIDAQAB
AoGAGRzwwir7XvBOAy5tM/uV6e+Zf6anZzus1s1Y1ClbjbE6HXbnWWF/wbZGOpet
3Zm4vD6MXc7jpTLryzTQIvVdfQbRc6+MUVeLKwZatTXtdZrhu+Jk7hx0nTPy8Jcb
uJqFk541aEw+mMogY/xEcfbWd6IOkp+4xqjlFLBEDytgbIECQQDvH/E6nk+hgN4H
qzzVtxxr397vWrjrIgPbJpQvBsafG7b0dA4AFjwVbFLmQcj2PprIMmPcQrooz8vp
jy4SHEg1AkEA/v13/5M47K9vCxmb8QeD/asydfsgS5TeuNi8DoUBEmiSJwma7FXY
fFUtxuvL7XvjwjN5B30pNEbc6Iuyt7y4MQJBAIt21su4b3sjXNueLKH85Q+phy2U
fQtuUE9txblTu14q3N7gHRZB4ZMhFYyDy8CKrN2cPg/Fvyt0Xlp/DoCzjA0CQQDU
y2ptGsuSmgUtWj3NM9xuwYPm+Z/F84K6+ARYiZ6PYj013sovGKUFfYAqVXVlxtIX
qyUBnu3X9ps8ZfjLZO7BAkEAlT4R5Yl6cGhaJQYZHOde3JEMhNRcVFMO8dJDaFeo
f9Oeos0UUothgiDktdQHxdNEwLjQf7lJJBzV+5OtwswCWA==
-----END RSA PRIVATE KEY-----`)
