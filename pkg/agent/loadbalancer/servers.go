package loadbalancer

import (
	"context"
	"errors"
	"math/rand"
	"net"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

var defaultDialer = &net.Dialer{}

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

// dialContext dials a new connection, and adds its wrapped connection to the map
func (s *server) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := defaultDialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	// don't lock until adding the connection to the map, otherwise we may block
	// while waiting for the dial to time out
	s.mutex.Lock()
	defer s.mutex.Unlock()

	conn = &serverConn{server: s, Conn: conn}
	s.connections[conn] = struct{}{}
	return conn, nil
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
