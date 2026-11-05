package logger

import (
	"errors"
	"fmt"

	"github.com/go-logr/logr"
)

// implicit interface check
var _ logr.LogSink = &MessageFilter{}
var _ logr.LogSink = &ErrorFilter{}

type MessageFilter struct {
	ls       logr.LogSink
	messages []string
}

func (mf *MessageFilter) Init(_ logr.RuntimeInfo) {}

func (mf *MessageFilter) Enabled(level int) bool {
	return mf.ls.Enabled(level)
}

func (mf *MessageFilter) Info(level int, msg string, kvs ...any) {
	for _, fmsg := range mf.messages {
		if msg == fmsg {
			return
		}
	}
	mf.ls.Info(level, msg, kvs...)
}

func (mf *MessageFilter) Error(err error, msg string, kvs ...any) {
	for _, fmsg := range mf.messages {
		if msg == fmsg {
			return
		}
	}
	mf.ls.Error(err, msg, kvs...)
}

func (mf *MessageFilter) WithValues(kvs ...any) logr.LogSink {
	return &MessageFilter{
		ls:       mf.ls.WithValues(kvs),
		messages: mf.messages,
	}
}

func (mf *MessageFilter) WithName(name string) logr.LogSink {
	return &MessageFilter{
		ls:       mf.ls.WithName(name),
		messages: mf.messages,
	}
}

func WithMessageFilter(messages ...string) LogOption {
	return func(l logr.Logger) logr.Logger {
		fmt.Printf("MessageFilter messages=%v\n", messages)
		return l.WithSink(&MessageFilter{ls: l.GetSink(), messages: messages})
	}
}

type ErrorFilter struct {
	ls     logr.LogSink
	errors []error
}

func (ef *ErrorFilter) Init(_ logr.RuntimeInfo) {}

func (ef *ErrorFilter) Enabled(level int) bool {
	return ef.ls.Enabled(level)
}

func (ef *ErrorFilter) Info(level int, msg string, kvs ...any) {
	ef.ls.Info(level, msg, kvs...)
}

func (ef *ErrorFilter) Error(err error, msg string, kvs ...any) {
	for _, ferr := range ef.errors {
		if errors.Is(err, ferr) {
			return
		}
	}
	ef.ls.Error(err, msg, kvs...)
}

func (ef *ErrorFilter) WithValues(kvs ...any) logr.LogSink {
	return &ErrorFilter{
		ls:     ef.ls.WithValues(kvs),
		errors: ef.errors,
	}
}

func (ef *ErrorFilter) WithName(name string) logr.LogSink {
	return &ErrorFilter{
		ls:     ef.ls.WithName(name),
		errors: ef.errors,
	}
}

func WithErrorFilter(errs ...error) LogOption {
	return func(l logr.Logger) logr.Logger {
		return l.WithSink(&ErrorFilter{ls: l.GetSink(), errors: errs})
	}
}
