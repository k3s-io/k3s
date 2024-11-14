package loadbalancer

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

type testServer struct {
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

func (s *testServer) address() string {
	return s.listener.Addr().String()
}

func ping(conn net.Conn) (string, error) {
	fmt.Fprintf(conn, "ping\n")
	result, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result), nil
}

// Test_UnitFailOver creates a LB using a default server (ie fixed registration endpoint)
// and then adds a new server (a node). The node server is then closed, and it is confirmed
// that new connections use the default server.
func Test_UnitFailOver(t *testing.T) {
	tmpDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	defaultServer, err := createServer(ctx, "default")
	if err != nil {
		t.Fatalf("createServer(default) failed: %v", err)
	}

	node1Server, err := createServer(ctx, "node1")
	if err != nil {
		t.Fatalf("createServer(node1) failed: %v", err)
	}

	node2Server, err := createServer(ctx, "node2")
	if err != nil {
		t.Fatalf("createServer(node2) failed: %v", err)
	}

	// start the loadbalancer with the default server as the only server
	lb, err := New(ctx, tmpDir, SupervisorServiceName, "http://"+defaultServer.address(), RandomPort, false)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	parsedURL, err := url.Parse(lb.LoadBalancerServerURL())
	if err != nil {
		t.Fatalf("url.Parse failed: %v", err)
	}
	localAddress := parsedURL.Host

	// add the node as a new server address.
	lb.Update([]string{node1Server.address()})

	// make sure connections go to the node
	conn1, err := net.Dial("tcp", localAddress)
	if err != nil {
		t.Fatalf("net.Dial failed: %v", err)
	}
	if result, err := ping(conn1); err != nil {
		t.Fatalf("ping(conn1) failed: %v", err)
	} else if result != "node1:ping" {
		t.Fatalf("Unexpected ping(conn1) result: %v", result)
	}

	t.Log("conn1 tested OK")

	// set failing health check for node 1
	lb.SetHealthCheck(node1Server.address(), func() bool { return false })

	// Server connections are checked every second, now that node 1 is failed
	// the connections to it should be closed.
	time.Sleep(2 * time.Second)

	if _, err := ping(conn1); err == nil {
		t.Fatal("Unexpected successful ping on closed connection conn1")
	}

	t.Log("conn1 closed on failure OK")

	// make sure connection still goes to the first node - it is failing health checks but so
	// is the default endpoint, so it should be tried first with health checks disabled,
	// before failing back to the default.
	conn2, err := net.Dial("tcp", localAddress)
	if err != nil {
		t.Fatalf("net.Dial failed: %v", err)

	}
	if result, err := ping(conn2); err != nil {
		t.Fatalf("ping(conn2) failed: %v", err)
	} else if result != "node1:ping" {
		t.Fatalf("Unexpected ping(conn2) result: %v", result)
	}

	t.Log("conn2 tested OK")

	// make sure the health checks don't close the connection we just made -
	// connections should only be closed when it transitions from health to unhealthy.
	time.Sleep(2 * time.Second)

	if result, err := ping(conn2); err != nil {
		t.Fatalf("ping(conn2) failed: %v", err)
	} else if result != "node1:ping" {
		t.Fatalf("Unexpected ping(conn2) result: %v", result)
	}

	t.Log("conn2 tested OK again")

	// shut down the first node server to force failover to the default
	node1Server.close()

	// make sure new connections go to the default, and existing connections are closed
	conn3, err := net.Dial("tcp", localAddress)
	if err != nil {
		t.Fatalf("net.Dial failed: %v", err)

	}
	if result, err := ping(conn3); err != nil {
		t.Fatalf("ping(conn3) failed: %v", err)
	} else if result != "default:ping" {
		t.Fatalf("Unexpected ping(conn3) result: %v", result)
	}

	t.Log("conn3 tested OK")

	if _, err := ping(conn2); err == nil {
		t.Fatal("Unexpected successful ping on closed connection conn2")
	}

	t.Log("conn2 closed on failure OK")

	// add the second node as a new server address.
	lb.Update([]string{node2Server.address()})

	// make sure connection now goes to the second node,
	// and connections to the default are closed.
	conn4, err := net.Dial("tcp", localAddress)
	if err != nil {
		t.Fatalf("net.Dial failed: %v", err)

	}
	if result, err := ping(conn4); err != nil {
		t.Fatalf("ping(conn4) failed: %v", err)
	} else if result != "node2:ping" {
		t.Fatalf("Unexpected ping(conn4) result: %v", result)
	}

	t.Log("conn4 tested OK")

	// Server connections are checked every second, now that we have a healthy
	// server, connections to the default server should be closed
	time.Sleep(2 * time.Second)

	if _, err := ping(conn3); err == nil {
		t.Fatal("Unexpected successful ping on connection conn3")
	}

	t.Log("conn3 closed on failure OK")
}

// Test_UnitFailFast confirms that connnections to invalid addresses fail quickly
func Test_UnitFailFast(t *testing.T) {
	tmpDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverURL := "http://127.0.0.1:0/"
	lb, err := New(ctx, tmpDir, SupervisorServiceName, serverURL, RandomPort, false)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	conn, err := net.Dial("tcp", lb.localAddress)
	if err != nil {
		t.Fatalf("net.Dial failed: %v", err)
	}

	done := make(chan error)
	go func() {
		_, err = ping(conn)
		done <- err
	}()
	timeout := time.After(10 * time.Millisecond)

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Unexpected successful ping from invalid address")
		}
	case <-timeout:
		t.Fatal("Test timed out")
	}
}

// Test_UnitFailUnreachable confirms that connnections to unreachable addresses do fail
// within the expected duration
func Test_UnitFailUnreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow test in short mode.")
	}
	tmpDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverAddr := "192.0.2.1:6443"
	lb, err := New(ctx, tmpDir, SupervisorServiceName, "http://"+serverAddr, RandomPort, false)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Set failing health check to reduce retries
	lb.SetHealthCheck(serverAddr, func() bool { return false })

	conn, err := net.Dial("tcp", lb.localAddress)
	if err != nil {
		t.Fatalf("net.Dial failed: %v", err)
	}

	done := make(chan error)
	go func() {
		_, err = ping(conn)
		done <- err
	}()
	timeout := time.After(11 * time.Second)

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Unexpected successful ping from unreachable address")
		}
	case <-timeout:
		t.Fatal("Test timed out")
	}
}
