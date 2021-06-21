package loadbalancer

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/rancher/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	"inet.af/tcpproxy"
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
	Listener              net.Listener
}

const RandomPort = 0

var (
	SupervisorServiceName = version.Program + "-agent-load-balancer"
	APIServerServiceName  = version.Program + "-api-server-agent-load-balancer"
	ETCDServerServiceName = version.Program + "-etcd-server-load-balancer"
)

func New(ctx context.Context, dataDir, serviceName, serverURL string, lbServerPort int) (_lb *LoadBalancer, _err error) {
	config := net.ListenConfig{Control: reusePort}
	listener, err := config.Listen(ctx, "tcp", "127.0.0.1:"+strconv.Itoa(lbServerPort))
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

	originalServerAddress, localServerURL, err := parseURL(serverURL, localAddress)
	if err != nil {
		return nil, err
	}

	if serverURL == localServerURL {
		logrus.Debugf("Initial server URL for load balancer points at local server URL - starting with empty original server address")
		originalServerAddress = ""
	}

	lb := &LoadBalancer{
		dialer:                &net.Dialer{},
		configFile:            filepath.Join(dataDir, "etc", serviceName+".json"),
		localAddress:          localAddress,
		localServerURL:        localServerURL,
		originalServerAddress: originalServerAddress,
		ServerURL:             serverURL,
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
