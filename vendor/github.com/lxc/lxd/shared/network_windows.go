// +build windows

package shared

import (
	"crypto/x509"
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var once sync.Once
var systemRoots *x509.CertPool

func systemCertPool() (*x509.CertPool, error) {
	once.Do(initSystemRoots)
	if systemRoots == nil {
		return nil, fmt.Errorf("Bad system root pool")
	}
	return systemRoots, nil
}

func initSystemRoots() {
	const CRYPT_E_NOT_FOUND = 0x80092004

	store, err := windows.CertOpenSystemStore(0, windows.StringToUTF16Ptr("ROOT"))
	if err != nil {
		systemRoots = nil
		return
	}
	defer windows.CertCloseStore(store, 0)

	roots := x509.NewCertPool()
	var cert *windows.CertContext
	for {
		cert, err = windows.CertEnumCertificatesInStore(store, cert)
		if err != nil {
			if errno, ok := err.(windows.Errno); ok {
				if errno == CRYPT_E_NOT_FOUND {
					break
				}
			}
			systemRoots = nil
			return
		}
		if cert == nil {
			break
		}
		// Copy the buf, since ParseCertificate does not create its own copy.
		buf := (*[1 << 20]byte)(unsafe.Pointer(cert.EncodedCert))[:]
		buf2 := make([]byte, cert.Length)
		copy(buf2, buf)
		if c, err := x509.ParseCertificate(buf2); err == nil {
			roots.AddCert(c)
		}
	}
	systemRoots = roots
}
