// Package signal provides helper functions for dealing with signals across
// various operating systems.
//
// Forked from https://github.com/moby/moby/tree/37defbfd9b968f38e8e15dfa5f06d9f878bd65ba/pkg/signal
package signal

import (
	"os"
	"os/signal"
)

// CatchAll catches all signals and relays them to the specified channel.
func CatchAll(sigc chan os.Signal) {
	var handledSigs []os.Signal
	for _, s := range SignalMap {
		handledSigs = append(handledSigs, s)
	}
	signal.Notify(sigc, handledSigs...)
}

// StopCatch stops catching the signals and closes the specified channel.
func StopCatch(sigc chan os.Signal) {
	signal.Stop(sigc)
	close(sigc)
}
