package sigproxy

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"

	"github.com/rootless-containers/rootlesskit/pkg/sigproxy/signal"
)

// ForwardAllSignals forwards signals.
// Based on https://github.com/docker/cli/blob/ef2f64abbd37edfa148f745fa0013731b5074d1b/cli/command/container/tty.go#L99-L126
func ForwardAllSignals(ctx context.Context, pid int) chan os.Signal {
	sigc := make(chan os.Signal, 128)
	signal.CatchAll(sigc)
	go func() {
		for s := range sigc {
			if s == unix.SIGCHLD || s == unix.SIGPIPE || s == unix.SIGURG {
				continue
			}
			us, ok := s.(unix.Signal)
			if !ok {
				logrus.Warnf("Unsupported signal %v", s)
				continue
			}
			if err := unix.Kill(pid, us); err != nil {
				logrus.WithError(err).Debugf("Error sending signal %v", s)
			}
		}
	}()
	return sigc
}
