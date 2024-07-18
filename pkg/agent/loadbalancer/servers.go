package loadbalancer

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/k3s-io/k3s/pkg/version"
	http_dialer "github.com/mwitkow/go-http-dialer"
	"github.com/pkg/errors"
	"golang.org/x/net/http/httpproxy"
	"golang.org/x/net/proxy"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
)

var defaultDialer proxy.Dialer = &net.Dialer{}

// SetHTTPProxy configures a proxy-enabled dialer to be used for all loadbalancer connections,
// if the agent has been configured to allow use of a HTTP proxy, and the environment has been configured
// to indicate use of a HTTP proxy for the server URL.
func SetHTTPProxy(address string) error {
	// Check if env variable for proxy is set
	if useProxy, _ := strconv.ParseBool(os.Getenv(version.ProgramUpper + "_AGENT_HTTP_PROXY_ALLOWED")); !useProxy || address == "" {
		return nil
	}

	serverURL, err := url.Parse(address)
	if err != nil {
		return errors.Wrapf(err, "failed to parse address %s", address)
	}

	// Call this directly instead of using the cached environment used by http.ProxyFromEnvironment to allow for testing
	proxyFromEnvironment := httpproxy.FromEnvironment().ProxyFunc()
	proxyURL, err := proxyFromEnvironment(serverURL)
	if err != nil {
		return errors.Wrapf(err, "failed to get proxy for address %s", address)
	}
	if proxyURL == nil {
		logrus.Debug(version.ProgramUpper + "_AGENT_HTTP_PROXY_ALLOWED is true but no proxy is configured for URL " + serverURL.String())
		return nil
	}

	dialer, err := proxyDialer(proxyURL)
	if err != nil {
		return errors.Wrapf(err, "failed to create proxy dialer for %s", proxyURL)
	}

	defaultDialer = dialer
	logrus.Debugf("Using proxy %s for agent connection to %s", proxyURL, serverURL)
	return nil
}

func (lb *LoadBalancer) setServers(serverAddresses []string) bool {
	serverAddresses, hasOriginalServer := sortServers(serverAddresses, lb.defaultServerAddress)
	if len(serverAddresses) == 0 {
		return false
	}

	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	newAddresses := sets.NewString(serverAddresses...)
	curAddresses := sets.NewString(lb.ServerAddresses...)
	if newAddresses.Equal(curAddresses) {
		return false
	}

	for addedServer := range newAddresses.Difference(curAddresses) {
		logrus.Infof("Adding server to load balancer %s: %s", lb.serviceName, addedServer)
		lb.servers[addedServer] = &server{
			address:     addedServer,
			connections: make(map[net.Conn]struct{}),
			healthCheck: func() bool { return true },
		}
	}

	for removedServer := range curAddresses.Difference(newAddresses) {
		server := lb.servers[removedServer]
		if server != nil {
			logrus.Infof("Removing server from load balancer %s: %s", lb.serviceName, removedServer)
			// Defer closing connections until after the new server list has been put into place.
			// Closing open connections ensures that anything stuck retrying on a stale server is forced
			// over to a valid endpoint.
			defer server.closeAll()
			// Don't delete the default server from the server map, in case we need to fall back to it.
			if removedServer != lb.defaultServerAddress {
				delete(lb.servers, removedServer)
			}
		}
	}

	lb.ServerAddresses = serverAddresses
	lb.randomServers = append([]string{}, lb.ServerAddresses...)
	rand.Shuffle(len(lb.randomServers), func(i, j int) {
		lb.randomServers[i], lb.randomServers[j] = lb.randomServers[j], lb.randomServers[i]
	})
	if !hasOriginalServer {
		lb.randomServers = append(lb.randomServers, lb.defaultServerAddress)
	}
	lb.currentServerAddress = lb.randomServers[0]
	lb.nextServerIndex = 1

	return true
}

// nextServer attempts to get the next server in the loadbalancer server list.
// If another goroutine has already updated the current server address to point at
// a different address than just failed, nothing is changed. Otherwise, a new server address
// is stored to the currentServerAddress field, and returned for use.
// This function must always be called by a goroutine that holds a read lock on the loadbalancer mutex.
func (lb *LoadBalancer) nextServer(failedServer string) (string, error) {
	// note: these fields are not protected by the mutex, so we clamp the index value and update
	// the index/current address using local variables, to avoid time-of-check vs time-of-use
	// race conditions caused by goroutine A incrementing it in between the time goroutine B
	// validates its value, and uses it as a list index.
	currentServerAddress := lb.currentServerAddress
	nextServerIndex := lb.nextServerIndex

	if len(lb.randomServers) == 0 {
		return "", errors.New("No servers in load balancer proxy list")
	}
	if len(lb.randomServers) == 1 {
		return currentServerAddress, nil
	}
	if failedServer != currentServerAddress {
		return currentServerAddress, nil
	}
	if nextServerIndex >= len(lb.randomServers) {
		nextServerIndex = 0
	}

	currentServerAddress = lb.randomServers[nextServerIndex]
	nextServerIndex++

	lb.currentServerAddress = currentServerAddress
	lb.nextServerIndex = nextServerIndex

	return currentServerAddress, nil
}

// dialContext dials a new connection using the environment's proxy settings, and adds its wrapped connection to the map
func (s *server) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := defaultDialer.Dial(network, address)
	if err != nil {
		return nil, err
	}

	// Wrap the connection and add it to the server's connection map
	s.mutex.Lock()
	defer s.mutex.Unlock()

	wrappedConn := &serverConn{server: s, Conn: conn}
	s.connections[wrappedConn] = struct{}{}
	return wrappedConn, nil
}

// proxyDialer creates a new proxy.Dialer that routes connections through the specified proxy.
func proxyDialer(proxyURL *url.URL) (proxy.Dialer, error) {
	if proxyURL.Scheme == "http" || proxyURL.Scheme == "https" {
		// Create a new HTTP proxy dialer
		httpProxyDialer := http_dialer.New(proxyURL)
		return httpProxyDialer, nil
	} else if proxyURL.Scheme == "socks5" {
		// For SOCKS5 proxies, use the proxy package's FromURL
		return proxy.FromURL(proxyURL, proxy.Direct)
	}
	return nil, fmt.Errorf("unsupported proxy scheme: %s", proxyURL.Scheme)
}

// closeAll closes all connections to the server, and removes their entries from the map
func (s *server) closeAll() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if l := len(s.connections); l > 0 {
		logrus.Infof("Closing %d connections to load balancer server %s", len(s.connections), s.address)
		for conn := range s.connections {
			// Close the connection in a goroutine so that we don't hold the lock while doing so.
			go conn.Close()
		}
	}
}

// Close removes the connection entry from the server's connection map, and
// closes the wrapped connection.
func (sc *serverConn) Close() error {
	sc.server.mutex.Lock()
	defer sc.server.mutex.Unlock()

	delete(sc.server.connections, sc)
	return sc.Conn.Close()
}

// SetDefault sets the selected address as the default / fallback address
func (lb *LoadBalancer) SetDefault(serverAddress string) {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	_, hasOriginalServer := sortServers(lb.ServerAddresses, lb.defaultServerAddress)
	// if the old default server is not currently in use, remove it from the server map
	if server := lb.servers[lb.defaultServerAddress]; server != nil && !hasOriginalServer {
		defer server.closeAll()
		delete(lb.servers, lb.defaultServerAddress)
	}
	// if the new default server doesn't have an entry in the map, add one
	if _, ok := lb.servers[serverAddress]; !ok {
		lb.servers[serverAddress] = &server{
			address:     serverAddress,
			healthCheck: func() bool { return true },
			connections: make(map[net.Conn]struct{}),
		}
	}

	lb.defaultServerAddress = serverAddress
	logrus.Infof("Updated load balancer %s default server address -> %s", lb.serviceName, serverAddress)
}

// SetHealthCheck adds a health-check callback to an address, replacing the default no-op function.
func (lb *LoadBalancer) SetHealthCheck(address string, healthCheck func() bool) {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	if server := lb.servers[address]; server != nil {
		logrus.Debugf("Added health check for load balancer %s: %s", lb.serviceName, address)
		server.healthCheck = healthCheck
	} else {
		logrus.Errorf("Failed to add health check for load balancer %s: no server found for %s", lb.serviceName, address)
	}
}

// runHealthChecks periodically health-checks all servers. Any servers that fail the health-check will have their
// connections closed, to force clients to switch over to a healthy server.
func (lb *LoadBalancer) runHealthChecks(ctx context.Context) {
	previousStatus := map[string]bool{}
	wait.Until(func() {
		lb.mutex.RLock()
		defer lb.mutex.RUnlock()
		for address, server := range lb.servers {
			status := server.healthCheck()
			if status == false && previousStatus[address] == true {
				// Only close connections when the server transitions from healthy to unhealthy;
				// we don't want to re-close all the connections every time as we might be ignoring
				// health checks due to all servers being marked unhealthy.
				defer server.closeAll()
			}
			previousStatus[address] = status
		}
	}, time.Second, ctx.Done())
	logrus.Debugf("Stopped health checking for load balancer %s", lb.serviceName)
}
