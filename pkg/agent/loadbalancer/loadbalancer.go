package loadbalancer

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/inetaf/tcpproxy"
	"github.com/k3s-io/k3s/pkg/util/metrics"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/sirupsen/logrus"
)

// LoadBalancer holds data for a local listener which forwards connections to a
// pool of remote servers. It is not a proper load-balancer in that it does not
// actually balance connections, but instead fails over to a new server only
// when a connection attempt to the currently selected server fails.
type LoadBalancer struct {
	serviceName  string
	configFile   string
	scheme       string
	localAddress string
	servers      serverList
	proxy        *tcpproxy.Proxy
}

const RandomPort = 0

var (
	SupervisorServiceName = version.Program + "-agent-load-balancer"
	APIServerServiceName  = version.Program + "-api-server-agent-load-balancer"
	ETCDServerServiceName = version.Program + "-etcd-server-load-balancer"
)

// New contstructs a new LoadBalancer instance. The default server URL, and
// currently active servers, are stored in a file within the dataDir.
func New(ctx context.Context, dataDir, serviceName, defaultServerURL string, lbServerPort int, isIPv6 bool) (_lb *LoadBalancer, _err error) {
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

	serverURL, err := url.Parse(defaultServerURL)
	if err != nil {
		return nil, err
	}

	// Set explicit port from scheme
	if serverURL.Port() == "" {
		if strings.ToLower(serverURL.Scheme) == "http" {
			serverURL.Host += ":80"
		}
		if strings.ToLower(serverURL.Scheme) == "https" {
			serverURL.Host += ":443"
		}
	}

	lb := &LoadBalancer{
		serviceName:  serviceName,
		configFile:   filepath.Join(dataDir, "etc", serviceName+".json"),
		scheme:       serverURL.Scheme,
		localAddress: listener.Addr().String(),
	}

	// if starting pointing at ourselves, don't set a default server address,
	// which will cause all dials to fail until servers are added.
	if serverURL.Host == lb.localAddress {
		logrus.Debugf("Initial server URL for load balancer %s points at local server URL - starting with empty default server address", serviceName)
	} else {
		lb.servers.setDefaultAddress(lb.serviceName, serverURL.Host)
	}

	lb.proxy = &tcpproxy.Proxy{
		ListenFunc: func(string, string) (net.Listener, error) {
			return listener, nil
		},
	}
	lb.proxy.AddRoute(serviceName, &tcpproxy.DialProxy{
		Addr:        serviceName,
		OnDialError: onDialError,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			start := time.Now()
			conn, err := lb.servers.dialContext(ctx, network, address)
			metrics.ObserveWithStatus(loadbalancerDials, start, err, serviceName)
			return conn, err
		},
	})

	if err := lb.updateConfig(); err != nil {
		return nil, err
	}
	if err := lb.proxy.Start(); err != nil {
		return nil, err
	}
	logrus.Infof("Running load balancer %s %s -> %v [default: %s]", serviceName, lb.localAddress, lb.servers.getAddresses(), lb.servers.getDefaultAddress())

	go lb.servers.runHealthChecks(ctx, lb.serviceName)

	return lb, nil
}

// Update updates the list of server addresses to contain only the listed servers.
func (lb *LoadBalancer) Update(serverAddresses []string) {
	if !lb.servers.setAddresses(lb.serviceName, serverAddresses) {
		return
	}

	logrus.Infof("Updated load balancer %s server addresses -> %v [default: %s]", lb.serviceName, lb.servers.getAddresses(), lb.servers.getDefaultAddress())

	if err := lb.writeConfig(); err != nil {
		logrus.Warnf("Error updating load balancer %s config: %s", lb.serviceName, err)
	}
}

// SetDefault sets the selected address as the default / fallback address
func (lb *LoadBalancer) SetDefault(serverAddress string) {
	lb.servers.setDefaultAddress(lb.serviceName, serverAddress)

	if err := lb.writeConfig(); err != nil {
		logrus.Warnf("Error updating load balancer %s config: %s", lb.serviceName, err)
	}
}

// SetHealthCheck adds a health-check callback to an address, replacing the default no-op function.
func (lb *LoadBalancer) SetHealthCheck(address string, healthCheck HealthCheckFunc) {
	if err := lb.servers.setHealthCheck(address, healthCheck); err != nil {
		logrus.Errorf("Failed to set health check for load balancer %s: %v", lb.serviceName, err)
	} else {
		logrus.Debugf("Set health check for load balancer %s: %s", lb.serviceName, address)
	}
}

func (lb *LoadBalancer) LocalURL() string {
	return lb.scheme + "://" + lb.localAddress
}

func (lb *LoadBalancer) ServerAddresses() []string {
	return lb.servers.getAddresses()
}

func onDialError(src net.Conn, dstDialErr error) {
	logrus.Debugf("Incoming conn %s, error dialing load balancer servers: %v", src.RemoteAddr(), dstDialErr)
	src.Close()
}

// ResetLoadBalancer will delete the local state file for the load balancer on disk
func ResetLoadBalancer(dataDir, serviceName string) {
	stateFile := filepath.Join(dataDir, "etc", serviceName+".json")
	if err := os.Remove(stateFile); err != nil && !os.IsNotExist(err) {
		logrus.Warn(err)
	}
}
