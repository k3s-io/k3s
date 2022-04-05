package loadbalancer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
)

type server struct {
	listener net.Listener
	conns    []net.Conn
	prefix   string
}

func createServer(prefix string) (*server, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s := &server{
		prefix:   prefix,
		listener: listener,
	}
	go s.serve()
	return s, nil
}

func (s *server) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.conns = append(s.conns, conn)
		go s.echo(conn)
	}
}

func (s *server) close() {
	s.listener.Close()
	for _, conn := range s.conns {
		conn.Close()
	}
}

func (s *server) echo(conn net.Conn) {
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

func assertEqual(t *testing.T, a interface{}, b interface{}) {
	if a != b {
		t.Fatalf("[ %v != %v ]", a, b)
	}
}

func assertNotEqual(t *testing.T, a interface{}, b interface{}) {
	if a == b {
		t.Fatalf("[ %v == %v ]", a, b)
	}
}

func Test_UnitFailOver(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "lb-test")
	if err != nil {
		assertEqual(t, err, nil)
	}
	defer os.RemoveAll(tmpDir)

	ogServe, err := createServer("og")
	if err != nil {
		assertEqual(t, err, nil)
	}

	lbServe, err := createServer("lb")
	if err != nil {
		assertEqual(t, err, nil)
	}

	cfg := cmds.Agent{
		ServerURL: fmt.Sprintf("http://%s/", ogServe.listener.Addr().String()),
		DataDir:   tmpDir,
	}

	lb, err := New(context.TODO(), cfg.DataDir, SupervisorServiceName, cfg.ServerURL, RandomPort, false)
	if err != nil {
		assertEqual(t, err, nil)
	}

	parsedURL, err := url.Parse(lb.LoadBalancerServerURL())
	if err != nil {
		assertEqual(t, err, nil)
	}
	localAddress := parsedURL.Host

	lb.Update([]string{lbServe.listener.Addr().String()})

	conn1, err := net.Dial("tcp", localAddress)
	if err != nil {
		assertEqual(t, err, nil)
	}
	result1, err := ping(conn1)
	if err != nil {
		assertEqual(t, err, nil)
	}
	assertEqual(t, result1, "lb:ping")

	lbServe.close()

	_, err = ping(conn1)
	assertNotEqual(t, err, nil)

	conn2, err := net.Dial("tcp", localAddress)
	if err != nil {
		assertEqual(t, err, nil)
	}
	result2, err := ping(conn2)
	if err != nil {
		assertEqual(t, err, nil)
	}
	assertEqual(t, result2, "og:ping")
}

func Test_UnitFailFast(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "lb-test")
	if err != nil {
		assertEqual(t, err, nil)
	}
	defer os.RemoveAll(tmpDir)

	cfg := cmds.Agent{
		ServerURL: "http://127.0.0.1:0/",
		DataDir:   tmpDir,
	}

	lb, err := New(context.TODO(), cfg.DataDir, SupervisorServiceName, cfg.ServerURL, RandomPort, false)
	if err != nil {
		assertEqual(t, err, nil)
	}

	conn, err := net.Dial("tcp", lb.localAddress)
	if err != nil {
		assertEqual(t, err, nil)
	}

	done := make(chan error)
	go func() {
		_, err = ping(conn)
		done <- err
	}()
	timeout := time.After(10 * time.Millisecond)

	select {
	case err := <-done:
		assertNotEqual(t, err, nil)
	case <-timeout:
		t.Fatal(errors.New("time out"))
	}
}
