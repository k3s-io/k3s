package proxy

import (
	"github.com/google/tcpproxy"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/sirupsen/logrus"
)

func Run(config *config.Node) error {
	logrus.Infof("Starting proxy %s -> %s", config.LocalAddress, config.ServerAddress)
	var proxy tcpproxy.Proxy
	proxy.AddRoute(config.LocalAddress, tcpproxy.To(config.ServerAddress))
	go func() {
		err := proxy.Run()
		logrus.Fatalf("TLS proxy stopped: %v", err)
	}()
	return nil
}
