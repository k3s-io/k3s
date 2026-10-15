package proxy

import (
	"errors"
	"io"
	"net"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"
)

// resetConn simulates a peer reset: Read blocks until gate is closed and then fails, Write always fails.
type resetConn struct {
	gate <-chan struct{}
}

func (c resetConn) Read([]byte) (int, error)  { <-c.gate; return 0, syscall.ECONNRESET }
func (c resetConn) Write([]byte) (int, error) { return 0, syscall.EPIPE }
func (c resetConn) Close() error              { return nil }

func waitForGoroutines(t *testing.T, want int) int {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		n := runtime.NumGoroutine()
		if n <= want || time.Now().After(deadline) {
			return n
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestProxyNoLeakWhenBothDirectionsFail(t *testing.T) {
	before := runtime.NumGoroutine()

	for i := 0; i < 2000; i++ {
		gate := make(chan struct{})
		done := make(chan error, 1)
		go func() { done <- Proxy(resetConn{gate}, resetConn{gate}) }()
		close(gate)
		select {
		case err := <-done:
			if !errors.Is(err, syscall.ECONNRESET) {
				t.Fatalf("iteration %d: expected ECONNRESET, got %v", i, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("iteration %d: Proxy did not return", i)
		}
	}

	if after := waitForGoroutines(t, before); after > before {
		t.Fatalf("goroutine leak: before=%d after=%d", before, after)
	}
}

func TestProxyCopiesBothDirections(t *testing.T) {
	lClient, lServer := net.Pipe()
	rServer, rClient := net.Pipe()

	done := make(chan error, 1)
	go func() { done <- Proxy(lServer, rServer) }()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		lClient.Write([]byte("ping"))
	}()
	go func() {
		defer wg.Done()
		buf := make([]byte, 4)
		if _, err := io.ReadFull(rClient, buf); err != nil || string(buf) != "ping" {
			t.Errorf("l->r: got %q, %v", buf, err)
		}
		rClient.Write([]byte("pong"))
	}()
	wg.Wait()

	buf := make([]byte, 4)
	if _, err := io.ReadFull(lClient, buf); err != nil || string(buf) != "pong" {
		t.Fatalf("r->l: got %q, %v", buf, err)
	}

	lClient.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil error on clean close, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Proxy did not return after client closed")
	}
}

func TestProxyReturnsWhenOtherSideIdle(t *testing.T) {
	lClient, lServer := net.Pipe()
	rServer, rClient := net.Pipe()
	defer rClient.Close()

	done := make(chan error, 1)
	go func() { done <- Proxy(lServer, rServer) }()

	// rClient never writes, so the r->l copy is blocked in Read until Proxy closes rServer.
	lClient.Close()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Proxy did not return while the other direction was idle")
	}
}
