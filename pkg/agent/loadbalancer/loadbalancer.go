package loadbalancer

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"sync"

	"github.com/google/tcpproxy"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/sirupsen/logrus"
)

type LoadBalancer struct {
	mutex  sync.Mutex
	dialer *net.Dialer
	proxy  *tcpproxy.Proxy

	configFile            string
	localAddress          string
	localServerURL        string
	originalServerAddress string
	ServerURL             string
	ServerAddresses       []string
	randomServers         []string
	currentServerAddress  string
	nextServerIndex       int
}

const (
	serviceName = "k3s-agent-load-balancer"
)

func Setup(ctx context.Context, cfg cmds.Agent) (_lb *LoadBalancer, _err error) {
	if cfg.DisableLoadBalancer {
		return nil, nil
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	defer func() {
		if _err != nil {
			logrus.Warnf("Error starting load balancer: %s", _err)
			if listener != nil {
				listener.Close()
			}
		}
	}()
	if err != nil {
		return nil, err
	}
	localAddress := listener.Addr().String()

	originalServerAddress, localServerURL, err := parseURL(cfg.ServerURL, localAddress)
	if err != nil {
		return nil, err
	}

	lb := &LoadBalancer{
		dialer:                &net.Dialer{},
		configFile:            filepath.Join(cfg.DataDir, "etc", serviceName+".json"),
		localAddress:          localAddress,
		localServerURL:        localServerURL,
		originalServerAddress: originalServerAddress,
		ServerURL:             cfg.ServerURL,
	}

	lb.setServers([]string{lb.originalServerAddress})

	lb.proxy = &tcpproxy.Proxy{
		ListenFunc: func(string, string) (net.Listener, error) {
			return listener, nil
		},
	}
	lb.proxy.AddRoute(serviceName, &tcpproxy.DialProxy{
		Addr:        serviceName,
		DialContext: lb.dialContext,
		OnDialError: onDialError,
	})

	if err := lb.updateConfig(); err != nil {
		return nil, err
	}
	if err := lb.proxy.Start(); err != nil {
		return nil, err
	}
	logrus.Infof("Running load balancer %s -> %v", lb.localAddress, lb.randomServers)

	return lb, nil
}

func (lb *LoadBalancer) Update(serverAddresses []string) {
	if lb == nil {
		return
	}
	if !lb.setServers(serverAddresses) {
		return
	}
	logrus.Infof("Updating load balancer server addresses -> %v", lb.randomServers)

	if err := lb.writeConfig(); err != nil {
		logrus.Warnf("Error updating load balancer config: %s", err)
	}
}

func (lb *LoadBalancer) LoadBalancerServerURL() string {
	if lb == nil {
		return ""
	}
	return lb.localServerURL
}

func (lb *LoadBalancer) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	startIndex := lb.nextServerIndex
	for {
		targetServer := lb.currentServerAddress

		conn, err := lb.dialer.DialContext(ctx, network, targetServer)
		if err == nil {
			return conn, nil
		}
		logrus.Debugf("Dial error from load balancer: %s", err)

		newServer, err := lb.nextServer(targetServer)
		if err != nil {
			return nil, err
		}
		if targetServer != newServer {
			logrus.Debugf("Dial server in load balancer failed over to %s", newServer)
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		maxIndex := len(lb.randomServers)
		if startIndex > maxIndex {
			startIndex = maxIndex
		}
		if lb.nextServerIndex == startIndex {
			return nil, errors.New("all servers failed")
		}
	}
}

func onDialError(src net.Conn, dstDialErr error) {
	logrus.Debugf("Incoming conn %v, error dialing load balancer servers: %v", src.RemoteAddr().String(), dstDialErr)
	src.Close()
}
