package loadbalancer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/k3s-io/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	"inet.af/tcpproxy"
)

type LoadBalancer struct {
	mutex  sync.Mutex
	dialer *net.Dialer
	proxy  *tcpproxy.Proxy

	serviceName          string
	configFile           string
	localAddress         string
	localServerURL       string
	defaultServerAddress string
	ServerURL            string
	ServerAddresses      []string
	randomServers        []string
	currentServerAddress string
	nextServerIndex      int
	Listener             net.Listener
}

const RandomPort = 0

var (
	SupervisorServiceName = version.Program + "-agent-load-balancer"
	APIServerServiceName  = version.Program + "-api-server-agent-load-balancer"
	ETCDServerServiceName = version.Program + "-etcd-server-load-balancer"
)

func New(ctx context.Context, dataDir, serviceName, serverURL string, lbServerPort int, isIPv6 bool) (_lb *LoadBalancer, _err error) {
	config := net.ListenConfig{Control: reusePort}
	var localAddress string
	if isIPv6 {
		localAddress = fmt.Sprintf("[::1]:%d", lbServerPort)
	} else {
		localAddress = fmt.Sprintf("127.0.0.1:%d", lbServerPort)
	}
	listener, err := config.Listen(ctx, "tcp", localAddress)
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

	// if lbServerPort was 0, the port was assigned by the OS when bound - see what we ended up with.
	localAddress = listener.Addr().String()

	defaultServerAddress, localServerURL, err := parseURL(serverURL, localAddress)
	if err != nil {
		return nil, err
	}

	if serverURL == localServerURL {
		logrus.Debugf("Initial server URL for load balancer %s points at local server URL - starting with empty default server address", serviceName)
		defaultServerAddress = ""
	}

	lb := &LoadBalancer{
		serviceName:          serviceName,
		dialer:               &net.Dialer{},
		configFile:           filepath.Join(dataDir, "etc", serviceName+".json"),
		localAddress:         localAddress,
		localServerURL:       localServerURL,
		defaultServerAddress: defaultServerAddress,
		ServerURL:            serverURL,
	}

	lb.setServers([]string{lb.defaultServerAddress})

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
	logrus.Infof("Running load balancer %s %s -> %v", serviceName, lb.localAddress, lb.randomServers)

	return lb, nil
}

func (lb *LoadBalancer) SetDefault(serverAddress string) {
	logrus.Infof("Updating load balancer %s default server address -> %s", lb.serviceName, serverAddress)
	lb.defaultServerAddress = serverAddress
}

func (lb *LoadBalancer) Update(serverAddresses []string) {
	if lb == nil {
		return
	}
	if !lb.setServers(serverAddresses) {
		return
	}
	logrus.Infof("Updating load balancer %s server addresses -> %v", lb.serviceName, lb.randomServers)

	if err := lb.writeConfig(); err != nil {
		logrus.Warnf("Error updating load balancer %s config: %s", lb.serviceName, err)
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
		logrus.Debugf("Dial error from load balancer %s: %s", lb.serviceName, err)

		newServer, err := lb.nextServer(targetServer)
		if err != nil {
			return nil, err
		}
		if targetServer != newServer {
			logrus.Debugf("Dial server in load balancer %s failed over to %s", lb.serviceName, newServer)
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

// ResetLoadBalancer will delete the local state file for the load balancer on disk
func ResetLoadBalancer(dataDir, serviceName string) error {
	stateFile := filepath.Join(dataDir, "etc", serviceName+".json")
	if err := os.Remove(stateFile); err != nil {
		logrus.Warn(err)
	}
	return nil
}
