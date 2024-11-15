package loadbalancer

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

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
	listener, err := net.Listen("tcp", "127.0.0.1:0")
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
	fmt.Fprintf(conn, "ping\n")
	result, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result), nil
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
