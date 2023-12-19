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

	"github.com/k3s-io/k3s/pkg/cli/cmds"
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

func createServer(prefix string) (*testServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s := &testServer{
		prefix:   prefix,
		listener: listener,
	}
	go s.serve()
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

func Test_UnitFailOver(t *testing.T) {
	tmpDir := t.TempDir()

	ogServe, err := createServer("og")
	if err != nil {
		t.Fatalf("createServer(og) failed: %v", err)
	}

	lbServe, err := createServer("lb")
	if err != nil {
		t.Fatalf("createServer(lb) failed: %v", err)
	}

	cfg := cmds.Agent{
		ServerURL: fmt.Sprintf("http://%s/", ogServe.listener.Addr().String()),
		DataDir:   tmpDir,
	}

	lb, err := New(context.TODO(), cfg.DataDir, SupervisorServiceName, cfg.ServerURL, RandomPort, false)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	parsedURL, err := url.Parse(lb.LoadBalancerServerURL())
	if err != nil {
		t.Fatalf("url.Parse failed: %v", err)
	}
	localAddress := parsedURL.Host

	lb.Update([]string{lbServe.listener.Addr().String()})

	conn1, err := net.Dial("tcp", localAddress)
	if err != nil {
		t.Fatalf("net.Dial failed: %v", err)
	}
	result1, err := ping(conn1)
	if err != nil {
		t.Fatalf("ping(conn1) failed: %v", err)
	}
	if result1 != "lb:ping" {
		t.Fatalf("Unexpected ping result: %v", result1)
	}

	lbServe.close()

	_, err = ping(conn1)
	if err == nil {
		t.Fatal("Unexpected successful ping on closed connection conn1")
	}

	conn2, err := net.Dial("tcp", localAddress)
	if err != nil {
		t.Fatalf("net.Dial failed: %v", err)

	}
	result2, err := ping(conn2)
	if err != nil {
		t.Fatalf("ping(conn2) failed: %v", err)
	}
	if result2 != "og:ping" {
		t.Fatalf("Unexpected ping result: %v", result2)
	}
}

func Test_UnitFailFast(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := cmds.Agent{
		ServerURL: "http://127.0.0.1:0/",
		DataDir:   tmpDir,
	}

	lb, err := New(context.TODO(), cfg.DataDir, SupervisorServiceName, cfg.ServerURL, RandomPort, false)
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
