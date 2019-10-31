package dqlite

import (
	"github.com/canonical/go-dqlite/client"
	"github.com/sirupsen/logrus"
)

func log() client.LogFunc {
	return func(level client.LogLevel, s string, i ...interface{}) {
		switch level {
		case client.LogDebug:
			logrus.Debugf(s, i...)
		case client.LogError:
			logrus.Errorf(s, i...)
		case client.LogInfo:
			logrus.Infof(s, i...)
		case client.LogWarn:
			logrus.Warnf(s, i...)
		}
	}
}
