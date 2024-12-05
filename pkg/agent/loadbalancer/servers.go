package loadbalancer

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
)

type HealthCheckFunc func() HealthCheckResult

// HealthCheckResult indicates the status of a server health check poll.
// For health-checks that poll in the background, Unknown should be returned
// if a poll has not occurred since the last check.
type HealthCheckResult int

const (
	HealthCheckResultUnknown HealthCheckResult = iota
	HealthCheckResultFailed
	HealthCheckResultOK
)

// serverList tracks potential backend servers for use by a loadbalancer.
type serverList struct {
	// This mutex protects access to the server list. All direct access to the list should be protected by it.
	mutex   sync.Mutex
	servers []*server
}

// setServers updates the server list to contain only the selected addresses.
func (sl *serverList) setAddresses(serviceName string, addresses []string) bool {
	newAddresses := sets.New(addresses...)
	curAddresses := sets.New(sl.getAddresses()...)
	if newAddresses.Equal(curAddresses) {
		return false
	}

	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	var closeAllFuncs []func()
	var defaultServer *server
	if i := slices.IndexFunc(sl.servers, func(s *server) bool { return s.isDefault }); i != -1 {
		defaultServer = sl.servers[i]
	}

	// add new servers
	for addedAddress := range newAddresses.Difference(curAddresses) {
		if defaultServer != nil && defaultServer.address == addedAddress {
			// make default server go through the same health check promotions as a new server when added
			logrus.Infof("Server %s->%s from add to load balancer %s", defaultServer, stateUnchecked, serviceName)
			defaultServer.state = stateUnchecked
			defaultServer.lastTransition = time.Now()
		} else {
			s := newServer(addedAddress, false)
			logrus.Infof("Adding server to load balancer %s: %s", serviceName, s.address)
			sl.servers = append(sl.servers, s)
		}
	}

	// remove old servers
	for removedAddress := range curAddresses.Difference(newAddresses) {
		if defaultServer != nil && defaultServer.address == removedAddress {
			// demote the default server down to standby, instead of deleting it
			defaultServer.state = stateStandby
			closeAllFuncs = append(closeAllFuncs, defaultServer.closeAll)
		} else {
			sl.servers = slices.DeleteFunc(sl.servers, func(s *server) bool {
				if s.address == removedAddress {
					logrus.Infof("Removing server from load balancer %s: %s", serviceName, s.address)
					// set state to invalid to prevent server from making additional connections
					s.state = stateInvalid
					closeAllFuncs = append(closeAllFuncs, s.closeAll)
					// remove metrics
					loadbalancerState.DeleteLabelValues(serviceName, s.address)
					loadbalancerConnections.DeleteLabelValues(serviceName, s.address)
					return true
				}
				return false
			})
		}
	}

	slices.SortFunc(sl.servers, compareServers)

	// Close all connections to servers that were removed
	for _, closeAll := range closeAllFuncs {
		closeAll()
	}

	return true
}

// getAddresses returns the addresses of all servers.
// If the default server is in standby state, indicating it is only present
// because it is the default, it is not returned in this list.
func (sl *serverList) getAddresses() []string {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	addresses := make([]string, 0, len(sl.servers))
	for _, s := range sl.servers {
		if s.isDefault && s.state == stateStandby {
			continue
		}
		addresses = append(addresses, s.address)
	}
	return addresses
}

// setDefault sets the server with the provided address as the default server.
// The default flag is cleared on all other servers, and if the server was previously
// only kept in the list because it was the default, it is removed.
func (sl *serverList) setDefaultAddress(serviceName, address string) {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	// deal with existing default first
	sl.servers = slices.DeleteFunc(sl.servers, func(s *server) bool {
		if s.isDefault && s.address != address {
			s.isDefault = false
			if s.state == stateStandby {
				s.state = stateInvalid
				defer s.closeAll()
				return true
			}
		}
		return false
	})

	// update or create server with selected address
	if i := slices.IndexFunc(sl.servers, func(s *server) bool { return s.address == address }); i != -1 {
		sl.servers[i].isDefault = true
	} else {
		sl.servers = append(sl.servers, newServer(address, true))
	}

	logrus.Infof("Updated load balancer %s default server: %s", serviceName, address)
	slices.SortFunc(sl.servers, compareServers)
}

// getDefault returns the address of the default server.
func (sl *serverList) getDefaultAddress() string {
	if s := sl.getDefaultServer(); s != nil {
		return s.address
	}
	return ""
}

// getDefault returns the default server.
func (sl *serverList) getDefaultServer() *server {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	if i := slices.IndexFunc(sl.servers, func(s *server) bool { return s.isDefault }); i != -1 {
		return sl.servers[i]
	}
	return nil
}

// getServers returns a copy of the servers list that can be safely iterated over without holding a lock
func (sl *serverList) getServers() []*server {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	return slices.Clone(sl.servers)
}

// getServer returns the first server with the specified address
func (sl *serverList) getServer(address string) *server {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	if i := slices.IndexFunc(sl.servers, func(s *server) bool { return s.address == address }); i != -1 {
		return sl.servers[i]
	}
	return nil
}

// setHealthCheck updates the health check function for a server, replacing the
// current function.
func (sl *serverList) setHealthCheck(address string, healthCheck HealthCheckFunc) error {
	if s := sl.getServer(address); s != nil {
		s.healthCheck = healthCheck
		return nil
	}
	return fmt.Errorf("no server found for %s", address)
}

// recordSuccess records a successful check of a server, either via health-check or dial.
// The server's state is adjusted accordingly.
func (sl *serverList) recordSuccess(srv *server, r reason) {
	var new_state state
	switch srv.state {
	case stateFailed, stateUnchecked:
		// dialed or health checked OK once, improve to recovering
		new_state = stateRecovering
	case stateRecovering:
		if r == reasonHealthCheck {
			// was recovering due to successful dial or first health check, can now improve
			if len(srv.connections) > 0 {
				// server accepted connections while recovering, attempt to go straight to active
				new_state = stateActive
			} else {
				// no connections, just make it preferred
				new_state = statePreferred
			}
		}
	case stateHealthy:
		if r == reasonDial {
			// improve from healthy to active by being dialed
			new_state = stateActive
		}
	case statePreferred:
		if r == reasonDial {
			// improve from healthy to active by being dialed
			new_state = stateActive
		} else {
			if time.Now().Sub(srv.lastTransition) > time.Minute {
				// has been preferred for a while without being dialed, demote to healthy
				new_state = stateHealthy
			}
		}
	}

	// no-op if state did not change
	if new_state == stateInvalid {
		return
	}

	// handle active transition and sort the server list while holding the lock
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	// handle states of other servers when attempting to make this one active
	if new_state == stateActive {
		for _, s := range sl.servers {
			if srv.address == s.address {
				continue
			}
			switch s.state {
			case stateFailed, stateStandby, stateRecovering, stateHealthy:
				// close connections to other non-active servers whenever we have a new active server
				defer s.closeAll()
			case stateActive:
				if len(s.connections) > len(srv.connections) {
					// if there is a currently active server that has more connections than we do,
					// close our connections and go to preferred instead
					new_state = statePreferred
					defer srv.closeAll()
				} else {
					// otherwise, close its connections and demote it to preferred
					s.state = statePreferred
					defer s.closeAll()
				}
			}
		}
	}

	// ensure some other routine didn't already make the transition
	if srv.state == new_state {
		return
	}

	logrus.Infof("Server %s->%s from successful %s", srv, new_state, r)
	srv.state = new_state
	srv.lastTransition = time.Now()

	slices.SortFunc(sl.servers, compareServers)
}

// recordFailure records a failed check of a server, either via health-check or dial.
// The server's state is adjusted accordingly.
func (sl *serverList) recordFailure(srv *server, r reason) {
	var new_state state
	switch srv.state {
	case stateUnchecked, stateRecovering:
		if r == reasonDial {
			// only demote from unchecked or recovering if a dial fails, health checks may
			// continue to fail despite it being dialable. just leave it where it is
			// and don't close any connections.
			new_state = stateFailed
		}
	case stateHealthy, statePreferred, stateActive:
		// should not have any connections when in any state other than active or
		// recovering, but close them all anyway to force failover.
		defer srv.closeAll()
		new_state = stateFailed
	}

	// no-op if state did not change
	if new_state == stateInvalid {
		return
	}

	// sort the server list while holding the lock
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	// ensure some other routine didn't already make the transition
	if srv.state == new_state {
		return
	}

	logrus.Infof("Server %s->%s from failed %s", srv, new_state, r)
	srv.state = new_state
	srv.lastTransition = time.Now()

	slices.SortFunc(sl.servers, compareServers)
}

// state is possible server health states, in increasing order of preference.
// The server list is kept sorted in descending order by this state value.
type state int

const (
	stateInvalid    state = iota
	stateFailed           // failed a health check or dial
	stateStandby          // reserved for use by default server if not in server list
	stateUnchecked        // just added, has not been health checked
	stateRecovering       // successfully health checked once, or dialed when failed
	stateHealthy          // normal state
	statePreferred        // recently transitioned from recovering; should be preferred as others may go down for maintenance
	stateActive           // currently active server
)

func (s state) String() string {
	switch s {
	case stateInvalid:
		return "INVALID"
	case stateFailed:
		return "FAILED"
	case stateStandby:
		return "STANDBY"
	case stateUnchecked:
		return "UNCHECKED"
	case stateRecovering:
		return "RECOVERING"
	case stateHealthy:
		return "HEALTHY"
	case statePreferred:
		return "PREFERRED"
	case stateActive:
		return "ACTIVE"
	default:
		return "UNKNOWN"
	}
}

// reason specifies the reason for a successful or failed health report
type reason int

const (
	reasonDial reason = iota
	reasonHealthCheck
)

func (r reason) String() string {
	switch r {
	case reasonDial:
		return "dial"
	case reasonHealthCheck:
		return "health check"
	default:
		return "unknown reason"
	}
}

// server tracks the connections to a server, so that they can be closed when the server is removed.
type server struct {
	// This mutex protects access to the connections map. All direct access to the map should be protected by it.
	mutex          sync.Mutex
	address        string
	isDefault      bool
	state          state
	lastTransition time.Time
	healthCheck    HealthCheckFunc
	connections    map[net.Conn]struct{}
}

// newServer creates a new server, with a default health check
// and default/state fields appropriate for whether or not
// the server is a full server, or just a fallback default.
func newServer(address string, isDefault bool) *server {
	state := stateUnchecked
	if isDefault {
		state = stateStandby
	}
	return &server{
		address:        address,
		isDefault:      isDefault,
		state:          state,
		lastTransition: time.Now(),
		healthCheck:    func() HealthCheckResult { return HealthCheckResultUnknown },
		connections:    make(map[net.Conn]struct{}),
	}
}

func (s *server) String() string {
	format := "%s@%s"
	if s.isDefault {
		format += "*"
	}
	return fmt.Sprintf(format, s.address, s.state)
}

// dialContext dials a new connection to the server using the environment's proxy settings, and adds its wrapped connection to the map
func (s *server) dialContext(ctx context.Context, network string) (net.Conn, error) {
	if s.state == stateInvalid {
		return nil, fmt.Errorf("server %s is stopping", s.address)
	}

	conn, err := defaultDialer.Dial(network, s.address)
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

// closeAll closes all connections to the server, and removes their entries from the map
func (s *server) closeAll() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if l := len(s.connections); l > 0 {
		logrus.Infof("Closing %d connections to load balancer server %s", len(s.connections), s)
		for conn := range s.connections {
			// Close the connection in a goroutine so that we don't hold the lock while doing so.
			go conn.Close()
		}
	}
}

// serverConn wraps a net.Conn so that it can be removed from the server's connection map when closed.
type serverConn struct {
	server *server
	net.Conn
}

// Close removes the connection entry from the server's connection map, and
// closes the wrapped connection.
func (sc *serverConn) Close() error {
	sc.server.mutex.Lock()
	defer sc.server.mutex.Unlock()

	delete(sc.server.connections, sc)
	return sc.Conn.Close()
}

// runHealthChecks periodically health-checks all servers and updates metrics
func (sl *serverList) runHealthChecks(ctx context.Context, serviceName string) {
	wait.Until(func() {
		for _, s := range sl.getServers() {
			switch s.healthCheck() {
			case HealthCheckResultOK:
				sl.recordSuccess(s, reasonHealthCheck)
			case HealthCheckResultFailed:
				sl.recordFailure(s, reasonHealthCheck)
			}
			if s.state != stateInvalid {
				loadbalancerState.WithLabelValues(serviceName, s.address).Set(float64(s.state))
				loadbalancerConnections.WithLabelValues(serviceName, s.address).Set(float64(len(s.connections)))
			}
		}
	}, time.Second, ctx.Done())
	logrus.Debugf("Stopped health checking for load balancer %s", serviceName)
}

// dialContext attemps to dial a connection to a server from the server list.
// Success or failure is recorded to ensure that server state is updated appropriately.
func (sl *serverList) dialContext(ctx context.Context, network, _ string) (net.Conn, error) {
	for _, s := range sl.getServers() {
		dialTime := time.Now()
		conn, err := s.dialContext(ctx, network)
		if err == nil {
			sl.recordSuccess(s, reasonDial)
			return conn, nil
		}
		logrus.Debugf("Dial error from server %s after %s: %s", s, time.Now().Sub(dialTime), err)
		sl.recordFailure(s, reasonDial)
	}
	return nil, errors.New("all servers failed")
}

// compareServers is a comparison function that can be used to sort the server list
// so that servers with a more preferred state, or higher number of connections, are ordered first.
func compareServers(a, b *server) int {
	c := cmp.Compare(b.state, a.state)
	if c == 0 {
		return cmp.Compare(len(b.connections), len(a.connections))
	}
	return c
}
