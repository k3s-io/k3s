package cmds

import (
	"flag"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

type Log struct {
	VLevel          int
	VModule         string
	LogFile         string
	AlsoLogToStderr bool
}

var (
	LogConfig Log

	VLevel = cli.IntFlag{
		Name:        "v",
		Usage:       "(logging) Number for the log level verbosity",
		Destination: &LogConfig.VLevel,
	}
	VModule = cli.StringFlag{
		Name:        "vmodule",
		Usage:       "(logging) Comma-separated list of pattern=N settings for file-filtered logging",
		Destination: &LogConfig.VModule,
	}
	LogFile = cli.StringFlag{
		Name:        "log,l",
		Usage:       "(logging) Log to file",
		Destination: &LogConfig.LogFile,
	}
	AlsoLogToStderr = cli.BoolFlag{
		Name:        "alsologtostderr",
		Usage:       "(logging) Log to standard error as well as file (if set)",
		Destination: &LogConfig.AlsoLogToStderr,
	}

	logSetupOnce sync.Once
)

func InitLogging() error {
	var rErr error
	logSetupOnce.Do(func() {
		if err := forkIfLoggingOrReaping(); err != nil {
			rErr = err
			return
		}

		if err := checkUnixTimestamp(); err != nil {
			rErr = err
			return
		}

		setupLogging()
	})
	return rErr
}

func checkUnixTimestamp() error {
	timeNow := time.Now()
	// check if time before 01/01/1980
	if timeNow.Before(time.Unix(315532800, 0)) {
		return fmt.Errorf("server time isn't set properly: %v", timeNow)
	}
	return nil
}

func setupLogging() {
	flag.Set("v", strconv.Itoa(LogConfig.VLevel))
	flag.Set("vmodule", LogConfig.VModule)
	flag.Set("alsologtostderr", strconv.FormatBool(Debug))
	flag.Set("logtostderr", strconv.FormatBool(!Debug))
	if Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
}
