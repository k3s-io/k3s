package signals

import (
	"context"
	"os"
	"os/signal"

	"github.com/sirupsen/logrus"
)

var onlyOneSignalHandler = make(chan struct{})
var signalHandler chan os.Signal
var shutdownHandler chan error

// SetupSignalHandler registers for SIGTERM and SIGINT. A context is returned
// which is cancelled on one of these signals. If a second signal is caught, the program
// is terminated with exit code 1.
func SetupSignalContext() context.Context {
	close(onlyOneSignalHandler) // panics when called twice

	signalHandler = make(chan os.Signal, 2)
	shutdownHandler = make(chan error, 1)

	ctx, cancel := context.WithCancel(context.Background())
	signal.Notify(signalHandler, shutdownSignals...)
	go func() {
		select {
		case s := <-signalHandler:
			logrus.Debugf("Signal received: %s", s)
		case e := <-shutdownHandler:
			if e != nil {
				logrus.Errorf("Shutdown request received: %q", e)
			} else {
				logrus.Infof("Shutdown request received")
			}
		}
		cancel()
		s := <-signalHandler
		logrus.Infof("Second shutdown signal received: %s, exiting...", s)

		//revive:disable-next-line:deep-exit
		os.Exit(1)
	}()

	return ctx
}

// RequestShutdown emulates a received event that is considered as shutdown signal
// This returns whether a handler was notified
func RequestShutdown(err error) bool {
	if shutdownHandler != nil {
		select {
		case shutdownHandler <- err:
			return true
		default:
		}
	}
	return false
}
