package logging

import (
	"fmt"
	"testing"
)

// Func is a function that can be used for logging.
type Func func(Level, string, ...interface{})

// Test returns a logging function that forwards messages to the test logger.
func Test(t *testing.T) Func {
	return func(l Level, format string, a ...interface{}) {
		format = fmt.Sprintf("%s: %s", l.String(), format)
		t.Logf(format, a...)
	}
}

// Stdout returns a logging function that prints log messages on standard
// output.
func Stdout() Func {
	return func(l Level, format string, a ...interface{}) {
		format = fmt.Sprintf("%s: %s\n", l.String(), format)
		fmt.Printf(format, a...)
	}
}
