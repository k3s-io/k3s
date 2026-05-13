// Package watchdog implements the k3s side of the systemd watchdog protocol.
//
// k3s strips NOTIFY_SOCKET from the process environment early in startup so
// that embedded components (kubelet, etcd, kine, etc.) cannot send READY=1 or
// WATCHDOG=1 to systemd on behalf of the whole process. That is intentional:
// the kubelet by itself does not know whether etcd, the API server, the CRI
// runtime, and the other in-process components are alive, so letting it ping
// the watchdog would mask whole-process failures.
//
// READY=1 is still sent the usual way via systemd.SdNotify by the server /
// agent startup code, which temporarily restores NOTIFY_SOCKET and then
// unsets it again. This package owns the periodic WATCHDOG=1 pings: callers
// pass in the cached NOTIFY_SOCKET value they captured before it was
// stripped, plus a health.Group covering every component that must be alive
// for the process to be considered healthy. WATCHDOG=1 is only sent while
// every Checker in the group passes; otherwise the loop stays quiet and
// systemd will restart the unit after WatchdogSec.
package watchdog

import (
	"context"
	"errors"
	"net"
	"time"

	systemd "github.com/coreos/go-systemd/v22/daemon"
	"github.com/k3s-io/k3s/pkg/daemons/health"
	"github.com/sirupsen/logrus"
)

func Run(ctx context.Context, socketPath string, group *health.Group) {
	if socketPath == "" {
		return
	}
	if group == nil || group.Len() == 0 {
		logrus.Warn("systemd watchdog: no health checks registered, notifier disabled")
		return
	}
	interval, err := systemd.SdWatchdogEnabled(false)
	if err != nil {
		logrus.Warnf("systemd watchdog: failed to read WATCHDOG_USEC: %v", err)
		return
	}
	if interval == 0 {
		logrus.Debug("systemd watchdog: not enabled by unit, notifier disabled")
		return
	}

	tick := interval / 2
	logrus.Infof("systemd watchdog: pinging every %s (WatchdogSec=%s), monitoring components %v",
		tick, interval, group.Names())

	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if name, err := group.CheckAll(ctx); err != nil {
				logrus.Warnf("systemd watchdog: %q is unhealthy, withholding WATCHDOG=1: %v", name, err)
				continue
			}
			if err := notify(socketPath, systemd.SdNotifyWatchdog); err != nil {
				logrus.Warnf("systemd watchdog: failed to send WATCHDOG=1: %v", err)
			}
		}
	}
}

// notify writes a single state line as a datagram to the systemd notify
// socket. The socket is SOCK_DGRAM, so each call is a self-contained
// message and we do not need to manage a persistent connection.
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
