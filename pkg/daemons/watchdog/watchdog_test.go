package watchdog

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/k3s-io/k3s/pkg/daemons/health"
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

// withWatchdogEnv sets WATCHDOG_USEC for the duration of the test so the
// notify loop actually ticks.
func withWatchdogEnv(t *testing.T, usec string) {
	t.Helper()
	prev, had := os.LookupEnv("WATCHDOG_USEC")
	if err := os.Setenv("WATCHDOG_USEC", usec); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	t.Cleanup(func() {
		if had {
			os.Setenv("WATCHDOG_USEC", prev)
		} else {
			os.Unsetenv("WATCHDOG_USEC")
		}
	})
}

func Test_UnitWatchdogNoSocketReturnsImmediately(t *testing.T) {
	g := health.NewGroup()
	g.Add(health.Func{ComponentName: "x", Probe: func(_ context.Context) error { return nil }})
	done := make(chan struct{})
	go func() { Run(context.Background(), "", g); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return when socketPath was empty")
	}
}

func Test_UnitWatchdogEmptyGroupReturnsImmediately(t *testing.T) {
	socket, _ := startNotifyListener(t)
	withWatchdogEnv(t, "200000") // 200ms

	done := make(chan struct{})
	go func() { Run(context.Background(), socket, health.NewGroup()); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return when group was empty")
	}
}

func Test_UnitWatchdogNoEnvReturnsImmediately(t *testing.T) {
	socket, _ := startNotifyListener(t)
	// Make sure WATCHDOG_USEC is unset.
	prev, had := os.LookupEnv("WATCHDOG_USEC")
	os.Unsetenv("WATCHDOG_USEC")
	t.Cleanup(func() {
		if had {
			os.Setenv("WATCHDOG_USEC", prev)
		}
	})

	g := health.NewGroup()
	g.Add(health.Func{ComponentName: "x", Probe: func(_ context.Context) error { return nil }})
	done := make(chan struct{})
	go func() { Run(context.Background(), socket, g); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return when WATCHDOG_USEC was unset")
	}
}

func Test_UnitWatchdogPingsWhenHealthy(t *testing.T) {
	socket, msgs := startNotifyListener(t)
	withWatchdogEnv(t, "100000")

	g := health.NewGroup()
	g.Add(health.Func{ComponentName: "ok", Probe: func(_ context.Context) error { return nil }})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go Run(ctx, socket, g)

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
	withWatchdogEnv(t, "100000")

	var calls atomic.Int32
	g := health.NewGroup()
	g.Add(health.Func{ComponentName: "bad", Probe: func(_ context.Context) error {
		calls.Add(1)
		return errors.New("unhealthy")
	}})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go Run(ctx, socket, g)

	select {
	case got := <-msgs:
		t.Fatalf("did not expect any WATCHDOG=1 ping, got %q", got)
	case <-time.After(500 * time.Millisecond):
	}
	if calls.Load() == 0 {
		t.Fatal("expected checker to have been invoked at least once")
	}
}

func Test_UnitWatchdogStopsOnContextCancel(t *testing.T) {
	socket, _ := startNotifyListener(t)
	withWatchdogEnv(t, "100000")

	g := health.NewGroup()
	g.Add(health.Func{ComponentName: "ok", Probe: func(_ context.Context) error { return nil }})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { Run(ctx, socket, g); close(done) }()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after ctx cancellation")
	}
}
