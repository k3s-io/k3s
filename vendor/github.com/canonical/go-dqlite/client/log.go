package client

import (
	"fmt"
	"log"
	"os"

	"github.com/canonical/go-dqlite/internal/logging"
)

// LogFunc is a function that can be used for logging.
type LogFunc = logging.Func

// LogLevel defines the logging level.
type LogLevel = logging.Level

// Available logging levels.
const (
	LogDebug = logging.Debug
	LogInfo  = logging.Info
	LogWarn  = logging.Warn
	LogError = logging.Error
)

// DefaultLogFunc emits messages using the stdlib's logger.
func DefaultLogFunc(l LogLevel, format string, a ...interface{}) {
	logger := log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)
	format = fmt.Sprintf("[%s]: %s", l.String(), format)
	logger.Printf(format, a...)
}
