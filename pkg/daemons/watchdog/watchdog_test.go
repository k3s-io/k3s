package watchdog

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"k8s.io/apiserver/pkg/server/healthz"
)

// startNotifyListener opens a unix datagram socket at a temporary path and
// returns the path plus a channel that receives every datagram written to it.
// The listener is cleaned up when the test ends.
func startNotifyListener(t *testing.T) (string, <-chan string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "notify.sock")
	conn, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: path, Net: "unixgram"})
	if err != nil {
		t.Fatalf("listen unixgram: %v", err)
	}
	t.Cleanup(func() {
		conn.Close()
		_ = os.Remove(path)
	})

	out := make(chan string, 16)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, _, err := conn.ReadFromUnix(buf)
			if err != nil {
				close(out)
				return
			}
			out <- string(buf[:n])
		}
	}()
	return path, out
}

// fakeChecker returns a healthz.HealthChecker that calls fn and exposes a
// counter of invocations.
type fakeChecker struct {
	name  string
	fn    func() error
	calls atomic.Int32
}

func (f *fakeChecker) Name() string                   { return f.name }
func (f *fakeChecker) Check(_ *http.Request) error    { f.calls.Add(1); return f.fn() }

func ok(name string) *fakeChecker {
	return &fakeChecker{name: name, fn: func() error { return nil }}
}

func bad(name string, err error) *fakeChecker {
	return &fakeChecker{name: name, fn: func() error { return err }}
}

func Test_UnitWatchdogNoSocketReturnsImmediately(t *testing.T) {
	done := make(chan struct{})
	go func() {
		Run(context.Background(), "", time.Second, []healthz.HealthChecker{ok("x")})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return when socketPath was empty")
	}
}

func Test_UnitWatchdogZeroIntervalReturnsImmediately(t *testing.T) {
	socket, _ := startNotifyListener(t)
	done := make(chan struct{})
	go func() {
		Run(context.Background(), socket, 0, []healthz.HealthChecker{ok("x")})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return when interval was zero")
	}
}

func Test_UnitWatchdogEmptyCheckersReturnsImmediately(t *testing.T) {
	socket, _ := startNotifyListener(t)
	done := make(chan struct{})
	go func() {
		Run(context.Background(), socket, time.Second, nil)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return when checkers was empty")
	}
}

func Test_UnitWatchdogPingsWhenHealthy(t *testing.T) {
	socket, msgs := startNotifyListener(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go Run(ctx, socket, 100*time.Millisecond, []healthz.HealthChecker{ok("ok")})

	select {
	case got := <-msgs:
		if got != "WATCHDOG=1" {
			t.Errorf("expected WATCHDOG=1, got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive WATCHDOG=1 within 2s")
	}
}

func Test_UnitWatchdogWithholdsPingWhenUnhealthy(t *testing.T) {
	socket, msgs := startNotifyListener(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	checker := bad("bad", errors.New("unhealthy"))
	go Run(ctx, socket, 100*time.Millisecond, []healthz.HealthChecker{checker})

	select {
	case got := <-msgs:
		t.Fatalf("did not expect any WATCHDOG=1 ping, got %q", got)
	case <-time.After(500 * time.Millisecond):
	}
	if checker.calls.Load() == 0 {
		t.Fatal("expected checker to have been invoked at least once")
	}
}

func Test_UnitWatchdogStopsOnContextCancel(t *testing.T) {
	socket, _ := startNotifyListener(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		Run(ctx, socket, 100*time.Millisecond, []healthz.HealthChecker{ok("ok")})
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after ctx cancellation")
	}
}
