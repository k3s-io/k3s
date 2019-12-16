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

var (
	logger = log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)
)

// DefaultLogFunc emits messages using the stdlib's logger.
func DefaultLogFunc(l LogLevel, format string, a ...interface{}) {
	logger.Output(2, fmt.Sprintf("[%s]: %s", l.String(), fmt.Sprintf(format, a...)))
}
