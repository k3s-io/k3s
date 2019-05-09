package proxy

import (
	"crypto/tls"
	"net/http"

	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/proxy"
	"github.com/sirupsen/logrus"
)

func Run(config *config.Node) error {
	proxy, err := proxy.NewSimpleProxy(config.ServerAddress, config.CACerts, true)
	if err != nil {
		return err
	}

	listener, err := tls.Listen("tcp", config.LocalAddress, &tls.Config{
		Certificates: []tls.Certificate{
			*config.Certificate,
		},
	})

	if err != nil {
		return errors.Wrap(err, "Failed to start tls listener")
	}

	go func() {
		err := http.Serve(listener, proxy)
		logrus.Fatalf("TLS proxy stopped: %v", err)
	}()

	return nil
}
