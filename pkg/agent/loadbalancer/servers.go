package loadbalancer

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/k3s-io/k3s/pkg/version"
	http_dialer "github.com/mwitkow/go-http-dialer"
	"golang.org/x/net/proxy"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

var defaultDialer proxy.Dialer = &net.Dialer{}

func init() {
	// Check if env variable for proxy is set
	address := os.Getenv(version.ProgramUpper + "_URL")

	if useProxy, _ := strconv.ParseBool(os.Getenv(version.ProgramUpper + "_AGENT_HTTP_PROXY_ALLOWED")); !useProxy {
		return
	}

	req, err := http.NewRequest("GET", "https://"+address, nil)
	if err != nil {
		logrus.Errorf("Error creating request for address %s: %v", address, err)
	}
	proxyURL, err := http.ProxyFromEnvironment(req)
	if err != nil {
		logrus.Errorf("Error getting the proxy for address %s: %v", address, err)
	}

	if dialer, err := proxyDialer(proxyURL); err != nil {
		logrus.Errorf("Error creating the proxyDialer for  %s: %v", address, err)
	} else {
		defaultDialer = dialer
	}
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
		lb.servers[addedServer] = &server{connections: make(map[net.Conn]struct{})}
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

func (lb *LoadBalancer) nextServer(failedServer string) (string, error) {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	if len(lb.randomServers) == 0 {
		return "", errors.New("No servers in load balancer proxy list")
	}
	if len(lb.randomServers) == 1 {
		return lb.currentServerAddress, nil
	}
	if failedServer != lb.currentServerAddress {
		return lb.currentServerAddress, nil
	}
	if lb.nextServerIndex >= len(lb.randomServers) {
		lb.nextServerIndex = 0
	}

	lb.currentServerAddress = lb.randomServers[lb.nextServerIndex]
	lb.nextServerIndex++

	return lb.currentServerAddress, nil
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

	logrus.Debugf("Closing %d connections to load balancer server", len(s.connections))
	for conn := range s.connections {
		// Close the connection in a goroutine so that we don't hold the lock while doing so.
		go conn.Close()
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
