package dqlite

import (
	"fmt"
	"os"

	"github.com/canonical/go-dqlite/internal/bindings"
	"github.com/canonical/go-dqlite/internal/protocol"
	"github.com/pkg/errors"
)

// ConfigMultiThread sets the threading mode of SQLite to Multi-thread.
//
// By default go-dqlite configures SQLite to Single-thread mode, because the
// dqlite engine itself is single-threaded, and enabling Multi-thread or
// Serialized modes would incur in a performance penality.
//
// If your Go process also uses SQLite directly (e.g. using the
// github.com/mattn/go-sqlite3 bindings) you might need to switch to
// Multi-thread mode in order to be thread-safe.
//
// IMPORTANT: It's possible to successfully change SQLite's threading mode only
// if no SQLite APIs have been invoked yet (e.g. no database has been opened
// yet). Therefore you'll typically want to call ConfigMultiThread() very early
// in your process setup. Alternatively you can set the GO_DQLITE_MULTITHREAD
// environment variable to 1 at process startup, in order to prevent go-dqlite
// from setting Single-thread mode at all.
func ConfigMultiThread() error {
	if err := bindings.ConfigMultiThread(); err != nil {
		if err, ok := err.(protocol.Error); ok && err.Code == 21 /* SQLITE_MISUSE */ {
			return fmt.Errorf("SQLite is already initialized")
		}
		return errors.Wrap(err, "unknown error")
	}
	return nil
}

func init() {
	// Don't enable single thread mode by default if GO_DQLITE_MULTITHREAD
	// is set.
	if os.Getenv("GO_DQLITE_MULTITHREAD") == "1" {
		return
	}
	err := bindings.ConfigSingleThread()
	if err != nil {
		panic(errors.Wrap(err, "set single thread mode"))
	}
}
