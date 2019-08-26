package tls

import (
	"crypto/tls"

	"github.com/coreos/etcd/pkg/transport"
)

type Config struct {
	CAFile   string
	CertFile string
	KeyFile  string
}

func (c Config) ClientConfig() (*tls.Config, error) {
	if c.CertFile == "" && c.KeyFile == "" && c.CAFile == "" {
		return nil, nil
	}

	info := &transport.TLSInfo{
		CertFile: c.CertFile,
		KeyFile:  c.KeyFile,
		CAFile:   c.CAFile,
	}
	tlsConfig, err := info.ClientConfig()
	if err != nil {
		return nil, err
	}

	return tlsConfig, nil
}
