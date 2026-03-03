package errors

import (
	"errors"
	"fmt"
	"path"
	"runtime"
	"slices"
	"strings"
)

// wrap stdlib errors functions to make it easier to import this package as a replacement
var As = errors.As
var Is = errors.Is
var New = errors.New

var ErrCommandNoArgs = New("this command does not take any arguments")
var ErrUnsupportedPlatform = New("unsupported platform")

func WithStack(err error) error {
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
	return fmt.Errorf("%w at %s", err, strings.Join(stack, "->"))
}

func WithMessage(err error, msg string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", msg, err)
}

func WithMessagef(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}
