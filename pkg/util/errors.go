package util

import (
	"errors"
	"fmt"
	"path"
	"runtime"
	"slices"
	"strings"
)

var ErrCommandNoArgs = errors.New("this command does not take any arguments")
var ErrUnsupportedPlatform = errors.New("unsupported platform")

func ErrWithStack(message string) error {
	stack := []string{}
	pcs := make([]uintptr, 32)
	runtime.Callers(2, pcs)
	frames := runtime.CallersFrames(pcs)
	for {
		frame, more := frames.Next()
		if !strings.HasPrefix(frame.Function, "runtime.") {
			stack = append(stack, fmt.Sprintf("%s(%s:%d)", frame.Function, path.Base(frame.File), frame.Line))
		}
		if !more {
			break
		}
	}
	slices.Reverse(stack)
	return errors.New(message + " at " + strings.Join(stack, "->"))
}
