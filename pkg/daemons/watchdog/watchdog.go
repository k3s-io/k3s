// Package watchdog implements the k3s side of the systemd notify / watchdog
// protocol.
//
// k3s strips NOTIFY_SOCKET (and WATCHDOG_USEC) from the process environment
// early in startup so embedded components — kubelet, etcd, kine, etc. —
// cannot ping systemd on behalf of the whole process. That is intentional:
// the kubelet by itself has no visibility into etcd, the apiserver, or the
// CRI runtime, so letting it ping the watchdog would mask whole-process
// failures.
//
// READY=1 is still sent the usual way via systemd.SdNotify in the server /
// agent startup code, which temporarily restores NOTIFY_SOCKET and then
// unsets it again. This package owns the periodic WATCHDOG=1 pings: callers
// pass in the cached NOTIFY_SOCKET path and WATCHDOG_USEC interval that they
// captured before stripping, plus the set of healthz.HealthCheckers covering
// every component that must be alive for the process to be considered
// healthy. WATCHDOG=1 is only sent while every check passes; otherwise the
// loop stays quiet and systemd will restart the unit after WatchdogSec.
package watchdog

import (
	"context"
	"errors"
	"net"
	"time"

	systemd "github.com/coreos/go-systemd/v22/daemon"
	"github.com/sirupsen/logrus"
	"k8s.io/apiserver/pkg/server/healthz"
)

func Run(ctx context.Context, socketPath string, interval time.Duration, checkers []healthz.HealthChecker) {
	if socketPath == "" {
		return
	}
	if interval <= 0 {
		logrus.Debug("systemd watchdog: not enabled by unit, notifier disabled")
		return
	}
	if len(checkers) == 0 {
		logrus.Warn("systemd watchdog: no health checks registered, notifier disabled")
		return
	}

	tick := interval / 2
	names := make([]string, len(checkers))
	for i, c := range checkers {
		names[i] = c.Name()
	}
	logrus.Infof("systemd watchdog: pinging every %s (WatchdogSec=%s), monitoring components %v",
		tick, interval, names)

	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if name, err := check(checkers); err != nil {
				logrus.Warnf("systemd watchdog: %q is unhealthy, withholding WATCHDOG=1: %v", name, err)
				continue
			}
			if err := notify(socketPath, systemd.SdNotifyWatchdog); err != nil {
				logrus.Warnf("systemd watchdog: failed to send WATCHDOG=1: %v", err)
			}
		}
	}
}

func check(checkers []healthz.HealthChecker) (string, error) {
	for _, c := range checkers {
		if err := c.Check(nil); err != nil {
			return c.Name(), err
		}
	}
	return "", nil
}

func notify(socketPath, state string) error {
	if socketPath == "" {
		return errors.New("watchdog: empty notify socket path")
	}
	addr := &net.UnixAddr{Name: socketPath, Net: "unixgram"}
	conn, err := net.DialUnix(addr.Net, nil, addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Write([]byte(state))
	return err
}
