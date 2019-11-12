// +build !logdebug

package logger

import (
	"fmt"
)

// Logger is the main logging interface
type Logger interface {
	Debug(msg string, ctx ...interface{})
	Info(msg string, ctx ...interface{})
	Warn(msg string, ctx ...interface{})
	Error(msg string, ctx ...interface{})
	Crit(msg string, ctx ...interface{})
}

// Log contains the logger used by all the logging functions
var Log Logger

type nullLogger struct{}

func (nl nullLogger) Debug(msg string, ctx ...interface{}) {}
func (nl nullLogger) Info(msg string, ctx ...interface{})  {}
func (nl nullLogger) Warn(msg string, ctx ...interface{})  {}
func (nl nullLogger) Error(msg string, ctx ...interface{}) {}
func (nl nullLogger) Crit(msg string, ctx ...interface{})  {}

func init() {
	Log = nullLogger{}
}

// Debug logs a message (with optional context) at the DEBUG log level
func Debug(msg string, ctx ...interface{}) {
	if Log != nil {
		Log.Debug(msg, ctx...)
	}
}

// Info logs a message (with optional context) at the INFO log level
func Info(msg string, ctx ...interface{}) {
	if Log != nil {
		Log.Info(msg, ctx...)
	}
}

// Warn logs a message (with optional context) at the WARNING log level
func Warn(msg string, ctx ...interface{}) {
	if Log != nil {
		Log.Warn(msg, ctx...)
	}
}

// Error logs a message (with optional context) at the ERROR log level
func Error(msg string, ctx ...interface{}) {
	if Log != nil {
		Log.Error(msg, ctx...)
	}
}

// Crit logs a message (with optional context) at the CRITICAL log level
func Crit(msg string, ctx ...interface{}) {
	if Log != nil {
		Log.Crit(msg, ctx...)
	}
}

// Infof logs at the INFO log level using a standard printf format string
func Infof(format string, args ...interface{}) {
	if Log != nil {
		Log.Info(fmt.Sprintf(format, args...))
	}
}

// Debugf logs at the DEBUG log level using a standard printf format string
func Debugf(format string, args ...interface{}) {
	if Log != nil {
		Log.Debug(fmt.Sprintf(format, args...))
	}
}

// Warnf logs at the WARNING log level using a standard printf format string
func Warnf(format string, args ...interface{}) {
	if Log != nil {
		Log.Warn(fmt.Sprintf(format, args...))
	}
}

// Errorf logs at the ERROR log level using a standard printf format string
func Errorf(format string, args ...interface{}) {
	if Log != nil {
		Log.Error(fmt.Sprintf(format, args...))
	}
}

// Critf logs at the CRITICAL log level using a standard printf format string
func Critf(format string, args ...interface{}) {
	if Log != nil {
		Log.Crit(fmt.Sprintf(format, args...))
	}
}
