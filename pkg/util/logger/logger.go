package logger

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
	"k8s.io/klog/v2"
)

// implicit interface check
var _ logr.LogSink = &LogrusSink{}

// mapLevel maps logr log verbosities to logrus log levels
// logr does not have "log levels", but Info prints at verbosity 0
// while logrus's LevelInfo is unit32(4). This means:
// * panic/fatal/warn are unused,
// * 0 is info
// * 1 is debug
// * >=2 are trace
func mapLevel(level int) logrus.Level {
	if level >= 2 {
		return logrus.TraceLevel
	}
	return logrus.Level(level + 4)
}

// mapKV maps a list of keys and values to logrus Fields
func mapKV(kvs []any) logrus.Fields {
	fields := logrus.Fields{}
	for i := 0; i < len(kvs); i += 2 {
		k, ok := kvs[i].(string)
		if !ok {
			k = fmt.Sprint(kvs[i])
		}
		if len(kvs) > i+1 {
			fields[k] = kvs[i+1]
		} else {
			fields[k] = ""
		}
	}
	return fields
}

// LogrusSink wraps logrus the Logger/Entry types for use as a logr LogSink.
type LogrusSink struct {
	e  *logrus.Entry
	ri logr.RuntimeInfo
}

func NewLogrusSink(l *logrus.Logger) *LogrusSink {
	if l == nil {
		l = logrus.StandardLogger()
	}
	return &LogrusSink{e: logrus.NewEntry(l)}
}

func (ls *LogrusSink) AsLogr() logr.Logger {
	return logr.New(ls)
}

func (ls *LogrusSink) Init(ri logr.RuntimeInfo) {
	ls.ri = ri
}

func (ls *LogrusSink) Enabled(level int) bool {
	return ls.e.Logger.IsLevelEnabled(mapLevel(level))
}

func (ls *LogrusSink) Info(level int, msg string, kvs ...any) {
	ls.e.WithFields(mapKV(kvs)).Log(mapLevel(level), msg)
}

func (ls *LogrusSink) Error(err error, msg string, kvs ...any) {
	ls.e.WithError(err).WithFields(mapKV(kvs)).Error(msg)
}

func (ls *LogrusSink) WithValues(kvs ...any) logr.LogSink {
	return &LogrusSink{
		e:  ls.e.WithFields(mapKV(kvs)),
		ri: ls.ri,
	}
}

func (ls *LogrusSink) WithName(name string) logr.LogSink {
	if base, ok := ls.e.Data["logger"]; ok {
		name = fmt.Sprintf("%s/%s", base, name)
	}
	return ls.WithValues("logger", name)
}

func NewContextWithName(ctx context.Context, name string) context.Context {
	return klog.NewContext(ctx, klog.FromContext(ctx).WithName(name))
}
