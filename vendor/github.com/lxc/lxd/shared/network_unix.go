// +build !windows

package shared

import (
	"crypto/x509"
	"io/ioutil"
)

func systemCertPool() (*x509.CertPool, error) {
	// Get the system pool
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}

	// Attempt to load the system's pool too (for snaps)
	if PathExists("/var/lib/snapd/hostfs/etc/ssl/certs/ca-certificates.crt") {
		snapCerts, err := ioutil.ReadFile("/var/lib/snapd/hostfs/etc/ssl/certs/ca-certificates.crt")
		if err == nil {
			pool.AppendCertsFromPEM(snapCerts)
		}
	}

	return pool, nil
}
