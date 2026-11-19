package loadbalancer

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	//revive:disable:dot-imports
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sirupsen/logrus"
)

func Test_UnitLoadBalancer(t *testing.T) {
	_, reporterConfig := GinkgoConfiguration()
	reporterConfig.Verbose = testing.Verbose()
	RegisterFailHandler(Fail)
	RunSpecs(t, "LoadBalancer Suite", reporterConfig)
}

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

type testServer struct {
	address  string
	listener net.Listener
	conns    []net.Conn
	prefix   string
}

func createServer(ctx context.Context, prefix string) (*testServer, error) {
	return createServerAt(ctx, prefix, "127.0.0.1:0")
}

// createServerAt creates a server that listens on the specified address.
func createServerAt(ctx context.Context, prefix, address string) (*testServer, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	s := &testServer{
		prefix:   prefix,
		listener: listener,
		address:  listener.Addr().String(),
	}
	go s.serve()
	go func() {
		<-ctx.Done()
		s.close()
	}()
	return s, nil
}

func (s *testServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.conns = append(s.conns, conn)
		go s.echo(conn)
	}
}

func (s *testServer) close() {
	logrus.Printf("testServer %s closing", s.prefix)
	s.address = ""
	s.listener.Close()
	for _, conn := range s.conns {
		conn.Close()
	}
}

func (s *testServer) echo(conn net.Conn) {
	for {
		result, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			return
		}
		conn.Write([]byte(s.prefix + ":" + result))
	}
}

func ping(conn net.Conn) (string, error) {
	fmt.Fprint(conn, "ping\n")
	result, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result), nil
}

// dial opens a connection to the address, and returns it along with the name of the server that it reached.
// The connection is closed when the current spec finishes.
func dial(address string) (net.Conn, string) {
	GinkgoHelper()

	conn, err := net.Dial("tcp", address)
	Expect(err).NotTo(HaveOccurred(), "net.Dial failed")
	DeferCleanup(func() { conn.Close() })

	// fail instead of blocking if the server stops responding
	Expect(conn.SetDeadline(time.Now().Add(time.Minute))).To(Succeed(), "SetDeadline failed")

	result, err := ping(conn)
	Expect(err).NotTo(HaveOccurred(), "ping failed")
	return conn, strings.TrimSuffix(result, ":ping")
}

// firstDialed returns the addresses of the servers that are dialed first, sampled over many new connections.
func firstDialed(lb *LoadBalancer) []string {
	var addresses []string
	for range 100 {
		if servers := lb.servers.getDialOrder(); len(servers) > 0 && !slices.Contains(addresses, servers[0].address) {
			addresses = append(addresses, servers[0].address)
		}
	}
	return addresses
}

// serverState returns the current state of the server with the specified address.
func serverState(lb *LoadBalancer, address string) state {
	lb.servers.mutex.Lock()
	defer lb.servers.mutex.Unlock()

	for _, s := range lb.servers.servers {
		if s.address == address {
			return s.state
		}
	}
	return stateInvalid
}

var _ = Describe("LoadBalancer", func() {
	// creates a LB using a default server (ie fixed registration endpoint)
	// and then adds a new server (a node). The node server is then closed, and it is confirmed
	// that new connections use the default server.
	When("loadbalancer is running", Ordered, func() {
		ctx, cancel := context.WithCancel(context.Background())
		var defaultServer, node1Server, node2Server *testServer
		var conn1, conn2, conn3, conn4 net.Conn
		var lb *LoadBalancer
		var err error

		BeforeAll(func() {
			tmpDir := GinkgoT().TempDir()

			defaultServer, err = createServer(ctx, "default")
			Expect(err).NotTo(HaveOccurred(), "createServer(default) failed")

			node1Server, err = createServer(ctx, "node1")
			Expect(err).NotTo(HaveOccurred(), "createServer(node1) failed")

			node2Server, err = createServer(ctx, "node2")
			Expect(err).NotTo(HaveOccurred(), "createServer(node2) failed")

			// start the loadbalancer with the default server as the only server
			lb, err = New(ctx, tmpDir, SupervisorServiceName, "http://"+defaultServer.address, RandomPort, false)
			Expect(err).NotTo(HaveOccurred(), "New() failed")
		})

		AfterAll(func() {
			cancel()
		})

		It("adds node1 as a server", func() {
			// add the node as a new server address.
			lb.Update([]string{node1Server.address})
			lb.SetHealthCheck(node1Server.address, func() HealthCheckResult { return HealthCheckResultOK })

			By(fmt.Sprintf("Added node1 server: %v", lb.servers.getServers()))

			// wait for state to change
			Eventually(func() state {
				if s := lb.servers.getServer(node1Server.address); s != nil {
					return s.state
				}
				return stateInvalid
			}, 5, 1).Should(Equal(statePreferred))
		})

		It("connects to node1", func() {
			// make sure connections go to the node
			conn1, err = net.Dial("tcp", lb.localAddress)
			Expect(err).NotTo(HaveOccurred(), "net.Dial failed")
			Expect(ping(conn1)).To(Equal("node1:ping"), "Unexpected ping(conn1) result")

			By("conn1 tested OK")
		})

		It("changes node1 state to failed", func() {
			// set failing health check for node 1
			lb.SetHealthCheck(node1Server.address, func() HealthCheckResult { return HealthCheckResultFailed })

			// wait for state to change
			Eventually(func() state {
				if s := lb.servers.getServer(node1Server.address); s != nil {
					return s.state
				}
				return stateInvalid
			}, 5, 1).Should(Equal(stateFailed))
		})

		It("disconnects from node1", func() {
			// Server connections are checked every second, now that node 1 is failed
			// the connections to it should be closed.
			Expect(ping(conn1)).Error().To(HaveOccurred(), "Unexpected successful ping on closed connection conn1")

			By("conn1 closed on failure OK")

			// connections shoould go to the default now that node 1 is failed
			conn2, err = net.Dial("tcp", lb.localAddress)
			Expect(err).NotTo(HaveOccurred(), "net.Dial failed")
			Expect(ping(conn2)).To(Equal("default:ping"), "Unexpected ping(conn2) result")

			By("conn2 tested OK")
		})

		It("does not close connections unexpectedly", func() {
			// make sure the health checks don't close the connection we just made -
			// connections should only be closed when it transitions from health to unhealthy.
			time.Sleep(2 * time.Second)

			Expect(ping(conn2)).To(Equal("default:ping"), "Unexpected ping(conn2) result")

			By("conn2 tested OK again")
		})

		It("closes connections when dial fails", func() {
			// shut down the first node server to force failover to the default
			node1Server.close()

			// make sure new connections go to the default, and existing connections are closed
			conn3, err = net.Dial("tcp", lb.localAddress)
			Expect(err).NotTo(HaveOccurred(), "net.Dial failed")

			Expect(ping(conn3)).To(Equal("default:ping"), "Unexpected ping(conn3) result")

			By("conn3 tested OK")
		})

		It("replaces node2 as a server", func() {
			// add the second node as a new server address.
			lb.Update([]string{node2Server.address})
			lb.SetHealthCheck(node2Server.address, func() HealthCheckResult { return HealthCheckResultOK })

			By(fmt.Sprintf("Added node2 server: %v", lb.servers.getServers()))

			// wait for state to change
			Eventually(func() state {
				if s := lb.servers.getServer(node2Server.address); s != nil {
					return s.state
				}
				return stateInvalid
			}, 5, 1).Should(Equal(statePreferred))
		})

		It("connects to node2", func() {
			// make sure connection now goes to the second node,
			// and connections to the default are closed.
			conn4, err = net.Dial("tcp", lb.localAddress)
			Expect(err).NotTo(HaveOccurred(), "net.Dial failed")

			Expect(ping(conn4)).To(Equal("node2:ping"), "Unexpected ping(conn3) result")

			By("conn4 tested OK")
		})

		It("does not close connections unexpectedly", func() {
			// Server connections are checked every second, now that we have a healthy
			// server, connections to the default server should be closed
			time.Sleep(2 * time.Second)

			Expect(ping(conn2)).Error().To(HaveOccurred(), "Unexpected successful ping on closed connection conn1")

			By("conn2 closed on failure OK")

			Expect(ping(conn3)).Error().To(HaveOccurred(), "Unexpected successful ping on closed connection conn1")

			By("conn3 closed on failure OK")
		})

		It("adds default as a server", func() {
			// add the default as a full server
			lb.Update([]string{node2Server.address, defaultServer.address})
			lb.SetHealthCheck(defaultServer.address, func() HealthCheckResult { return HealthCheckResultOK })

			// wait for state to change
			Eventually(func() state {
				if s := lb.servers.getServer(defaultServer.address); s != nil {
					return s.state
				}
				return stateInvalid
			}, 5, 1).Should(Equal(statePreferred))

			By(fmt.Sprintf("Default server added: %v", lb.servers.getServers()))
		})

		It("returns the default server in the address list", func() {
			// confirm that both servers are listed in the address list
			Expect(lb.ServerAddresses()).To(ConsistOf(node2Server.address, defaultServer.address))

			// confirm that the default is still listed as default
			Expect(lb.servers.getDefaultAddress()).To(Equal(defaultServer.address), "default server is not default")
		})

		It("does not return the default server in the address list after removing it", func() {
			// remove the default as a server
			lb.Update([]string{node2Server.address})
			By(fmt.Sprintf("Default removed: %v", lb.servers.getServers()))

			// confirm that it is not listed as a server
			Expect(lb.ServerAddresses()).To(ConsistOf(node2Server.address))

			// but is still listed as the default
			Expect(lb.servers.getDefaultAddress()).To(Equal(defaultServer.address), "default server is not default")
		})

		It("removes default server when no longer default", func() {
			// set node2 as the default
			lb.SetDefault(node2Server.address)
			By(fmt.Sprintf("Default set: %v", lb.servers.getServers()))

			// confirm that it is still listed as a server
			Expect(lb.ServerAddresses()).To(ConsistOf(node2Server.address))

			// and is listed as the default
			Expect(lb.servers.getDefaultAddress()).To(Equal(node2Server.address), "node2 server is not default")
		})

		It("sets all three servers", func() {
			// set node2 as the default
			lb.SetDefault(defaultServer.address)
			By(fmt.Sprintf("Default set: %v", lb.servers.getServers()))

			lb.Update([]string{node1Server.address, node2Server.address, defaultServer.address})
			lb.SetHealthCheck(node1Server.address, func() HealthCheckResult { return HealthCheckResultOK })
			lb.SetHealthCheck(node2Server.address, func() HealthCheckResult { return HealthCheckResultOK })
			lb.SetHealthCheck(defaultServer.address, func() HealthCheckResult { return HealthCheckResultOK })

			// wait for state to change
			Eventually(func() state {
				if s := lb.servers.getServer(defaultServer.address); s != nil {
					return s.state
				}
				return stateInvalid
			}, 5, 1).Should(Equal(statePreferred))

			By(fmt.Sprintf("All servers set: %v", lb.servers.getServers()))

			// confirm that it is still listed as a server
			Expect(lb.ServerAddresses()).To(ConsistOf(node1Server.address, node2Server.address, defaultServer.address))

			// and is listed as the default
			Expect(lb.servers.getDefaultAddress()).To(Equal(defaultServer.address), "default server is not default")
		})
	})

	// confirms that the random policy spreads new connections across healthy servers,
	// and that connections are only closed when their own server fails.
	When("the random policy is set", Ordered, func() {
		ctx, cancel := context.WithCancel(context.Background())
		var defaultServer, node1Server, node2Server *testServer
		var lb *LoadBalancer
		var err error

		BeforeAll(func() {
			tmpDir := GinkgoT().TempDir()

			defaultServer, err = createServer(ctx, "default")
			Expect(err).NotTo(HaveOccurred(), "createServer(default) failed")

			node1Server, err = createServer(ctx, "node1")
			Expect(err).NotTo(HaveOccurred(), "createServer(node1) failed")

			node2Server, err = createServer(ctx, "node2")
			Expect(err).NotTo(HaveOccurred(), "createServer(node2) failed")

			lb, err = New(ctx, tmpDir, SupervisorServiceName, "http://"+defaultServer.address, RandomPort, false)
			Expect(err).NotTo(HaveOccurred(), "New() failed")
			lb.SetPolicy(PolicyRandom)

			lb.Update([]string{node1Server.address, node2Server.address, defaultServer.address})
			for _, s := range []*testServer{defaultServer, node1Server, node2Server} {
				lb.SetHealthCheck(s.address, func() HealthCheckResult { return HealthCheckResultOK })
			}

			// wait for all servers to pass health checks
			for _, s := range []*testServer{defaultServer, node1Server, node2Server} {
				Eventually(func() state { return serverState(lb, s.address) }, 5, 1).Should(Equal(statePreferred))
			}
		})

		AfterAll(func() {
			cancel()
		})

		It("dials all healthy servers", func() {
			Expect(firstDialed(lb)).To(ConsistOf(defaultServer.address, node1Server.address, node2Server.address), "Unexpected servers dialed first")
		})

		It("only closes connections to a server that fails", func() {
			// each server becomes active when it is first dialed, which should not close connections to the others
			conns := map[string]net.Conn{}
			for range 50 {
				if len(conns) == 3 {
					break
				}
				conn, name := dial(lb.localAddress)
				if _, ok := conns[name]; !ok {
					conns[name] = conn
				}
			}
			Expect(conns).To(HaveLen(3), "Not all servers received connections")
			for name, conn := range conns {
				Expect(ping(conn)).To(Equal(name+":ping"), "Unexpected ping result")
			}

			lb.SetHealthCheck(node1Server.address, func() HealthCheckResult { return HealthCheckResultFailed })
			Eventually(func() error {
				_, err := ping(conns["node1"])
				return err
			}, 5, 1).Should(HaveOccurred(), "Unexpected successful ping on connection to failed server")

			Expect(ping(conns["default"])).To(Equal("default:ping"), "Unexpected ping result")
			Expect(ping(conns["node2"])).To(Equal("node2:ping"), "Unexpected ping result")
		})

		It("only dials the failed server as a fallback", func() {
			Expect(firstDialed(lb)).To(ConsistOf(defaultServer.address, node2Server.address), "Unexpected servers dialed first")

			servers := lb.servers.getDialOrder()
			Expect(servers).To(HaveLen(3), "Unexpected server count")
			Expect(servers[2].address).To(Equal(node1Server.address), "Failed server is not the last fallback")
		})

		It("dials a recovered server", func() {
			lb.SetHealthCheck(node1Server.address, func() HealthCheckResult { return HealthCheckResultOK })
			Eventually(func() []string { return firstDialed(lb) }, 5, 1).Should(ContainElement(node1Server.address), "Recovered server is not dialed first")
		})
	})

	// confirms that with the random policy, a server that passes health checks but does not
	// accept connections is kept out of rotation until it accepts connections again.
	When("the random policy is set and a healthy server stops accepting connections", Ordered, func() {
		ctx, cancel := context.WithCancel(context.Background())
		var defaultServer, node1Server *testServer
		var defaultAddress, node1Address string
		var lb *LoadBalancer
		var err error

		BeforeAll(func() {
			tmpDir := GinkgoT().TempDir()

			defaultServer, err = createServer(ctx, "default")
			Expect(err).NotTo(HaveOccurred(), "createServer(default) failed")
			defaultAddress = defaultServer.address

			node1Server, err = createServer(ctx, "node1")
			Expect(err).NotTo(HaveOccurred(), "createServer(node1) failed")
			node1Address = node1Server.address

			lb, err = New(ctx, tmpDir, SupervisorServiceName, "http://"+defaultAddress, RandomPort, false)
			Expect(err).NotTo(HaveOccurred(), "New() failed")
			lb.SetPolicy(PolicyRandom)

			// health checks pass for the whole test
			lb.Update([]string{node1Address, defaultAddress})
			for _, address := range []string{defaultAddress, node1Address} {
				lb.SetHealthCheck(address, func() HealthCheckResult { return HealthCheckResultOK })
			}

			for _, address := range []string{defaultAddress, node1Address} {
				Eventually(func() state { return serverState(lb, address) }, 5, 1).Should(Equal(statePreferred))
			}
		})

		AfterAll(func() {
			cancel()
		})

		It("stops dialing the server first after a dial to it fails", func() {
			node1Server.close()

			// a connection that tries the closed server first should still reach the other server
			Eventually(func(g Gomega) {
				conn, err := net.Dial("tcp", lb.localAddress)
				g.Expect(err).NotTo(HaveOccurred(), "net.Dial failed")
				defer conn.Close()
				g.Expect(conn.SetDeadline(time.Now().Add(5*time.Second))).To(Succeed(), "SetDeadline failed")
				g.Expect(ping(conn)).To(Equal("default:ping"), "Unexpected ping result")
				g.Expect(firstDialed(lb)).To(ConsistOf(defaultAddress), "Unexpected servers dialed first")
			}, 5, "100ms").Should(Succeed())
		})

		It("does not dial the server first while only its health check passes", func() {
			Consistently(func() []string { return firstDialed(lb) }, 4, "500ms").Should(ConsistOf(defaultAddress), "Unexpected servers dialed first")
		})

		It("dials the server first once it accepts connections again", func() {
			Eventually(func() error {
				node1Server, err = createServerAt(ctx, "node1", node1Address)
				return err
			}, 5, 1).Should(Succeed(), "createServerAt(node1) failed")

			Eventually(func() []string { return firstDialed(lb) }, 5, 1).Should(ConsistOf(defaultAddress, node1Address), "Unexpected servers dialed first")
		})
	})

	// confirms that the random policy still closes connections to a default server that is
	// only a fallback, once a listed server has taken a connection.
	When("the random policy is set and the default server is a fallback", Ordered, func() {
		ctx, cancel := context.WithCancel(context.Background())
		var defaultServer, node1Server *testServer
		var lb *LoadBalancer
		var err error

		BeforeAll(func() {
			tmpDir := GinkgoT().TempDir()

			defaultServer, err = createServer(ctx, "default")
			Expect(err).NotTo(HaveOccurred(), "createServer(default) failed")

			node1Server, err = createServer(ctx, "node1")
			Expect(err).NotTo(HaveOccurred(), "createServer(node1) failed")

			lb, err = New(ctx, tmpDir, SupervisorServiceName, "http://"+defaultServer.address, RandomPort, false)
			Expect(err).NotTo(HaveOccurred(), "New() failed")
			lb.SetPolicy(PolicyRandom)
		})

		AfterAll(func() {
			cancel()
		})

		It("closes the fallback connection once a listed server is dialed", func() {
			conn1, name := dial(lb.localAddress)
			Expect(name).To(Equal("default"), "Unexpected server for conn1")

			lb.Update([]string{node1Server.address})
			lb.SetHealthCheck(node1Server.address, func() HealthCheckResult { return HealthCheckResultOK })
			Eventually(func() state { return serverState(lb, node1Server.address) }, 5, 1).Should(Equal(statePreferred))

			_, name = dial(lb.localAddress)
			Expect(name).To(Equal("node1"), "Unexpected server for conn2")

			Eventually(func() error {
				_, err := ping(conn1)
				return err
			}, 5, 1).Should(HaveOccurred(), "Unexpected successful ping on fallback connection conn1")
		})
	})

	// confirms that the loadbalancer will not dial itself
	When("the default server is the loadbalancer", Ordered, func() {
		ctx, cancel := context.WithCancel(context.Background())
		var defaultServer *testServer
		var lb *LoadBalancer
		var err error

		BeforeAll(func() {
			tmpDir := GinkgoT().TempDir()

			defaultServer, err = createServer(ctx, "default")
			Expect(err).NotTo(HaveOccurred(), "createServer(default) failed")
			address := defaultServer.address
			defaultServer.close()
			_, port, _ := net.SplitHostPort(address)
			intPort, _ := strconv.Atoi(port)

			lb, err = New(ctx, tmpDir, SupervisorServiceName, "http://"+address, intPort, false)
			Expect(err).NotTo(HaveOccurred(), "New() failed")
		})

		AfterAll(func() {
			cancel()
		})

		It("fails immediately", func() {
			conn, err := net.Dial("tcp", lb.localAddress)
			Expect(err).NotTo(HaveOccurred(), "net.Dial failed")

			_, err = ping(conn)
			Expect(err).To(HaveOccurred(), "Unexpected successful ping on failed connection")
		})
	})

	// confirms that connnections to invalid addresses fail quickly
	When("there are no valid addresses", Ordered, func() {
		ctx, cancel := context.WithCancel(context.Background())
		var lb *LoadBalancer
		var err error

		BeforeAll(func() {
			tmpDir := GinkgoT().TempDir()
			lb, err = New(ctx, tmpDir, SupervisorServiceName, "http://127.0.0.1:0/", RandomPort, false)
			Expect(err).NotTo(HaveOccurred(), "New() failed")
		})

		AfterAll(func() {
			cancel()
		})

		It("fails fast", func() {
			conn, err := net.Dial("tcp", lb.localAddress)
			Expect(err).NotTo(HaveOccurred(), "net.Dial failed")

			done := make(chan error)
			go func() {
				_, err = ping(conn)
				done <- err
			}()
			timeout := time.After(10 * time.Millisecond)

			select {
			case err := <-done:
				if err == nil {
					Fail("Unexpected successful ping from invalid address")
				}
			case <-timeout:
				Fail("Test timed out")
			}
		})
	})

	// confirms that connnections to unreachable addresses do fail within the
	// expected duration
	When("the server is unreachable", Ordered, func() {
		ctx, cancel := context.WithCancel(context.Background())
		var lb *LoadBalancer
		var err error

		BeforeAll(func() {
			tmpDir := GinkgoT().TempDir()
			lb, err = New(ctx, tmpDir, SupervisorServiceName, "http://192.0.2.1:6443", RandomPort, false)
			Expect(err).NotTo(HaveOccurred(), "New() failed")
		})

		AfterAll(func() {
			cancel()
		})

		It("fails with the correct timeout", func() {
			conn, err := net.Dial("tcp", lb.localAddress)
			Expect(err).NotTo(HaveOccurred(), "net.Dial failed")

			done := make(chan error)
			go func() {
				_, err = ping(conn)
				done <- err
			}()
			timeout := time.After(11 * time.Second)

			select {
			case err := <-done:
				if err == nil {
					Fail("Unexpected successful ping from unreachable address")
				}
			case <-timeout:
				Fail("Test timed out")
			}
		})
	})
})
