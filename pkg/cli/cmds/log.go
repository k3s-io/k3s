package cmds

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/docker/docker/pkg/reexec"
	"github.com/natefinch/lumberjack"
	"github.com/rancher/k3s/pkg/version"
	"github.com/rancher/spur/cli"
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

func InitLogging(action func(*cli.Context) error) func(*cli.Context) error {
	return func(ctx *cli.Context) error {
		var (
			err    error
			reExec bool
		)
		logSetupOnce.Do(func() {
			if LogConfig.LogFile != "" && os.Getenv("_K3S_LOG_REEXEC_") == "" {
				reExec = true
				err = runWithLogging()
				return
			}
			if err = checkUnixTimestamp(); err != nil {
				return
			}
			setupLogging()
		})
		if reExec || err != nil {
			return err
		}
		if action != nil {
			return action(ctx)
		}
		return nil
	}
}

func checkUnixTimestamp() error {
	timeNow := time.Now()
	// check if time before 01/01/1980
	if timeNow.Before(time.Unix(315532800, 0)) {
		return fmt.Errorf("server time isn't set properly: %v", timeNow)
	}
	return nil
}

func runWithLogging() error {
	var (
		l io.Writer
	)
	l = &lumberjack.Logger{
		Filename:   LogConfig.LogFile,
		MaxSize:    50,
		MaxBackups: 3,
		MaxAge:     28,
		Compress:   true,
	}
	if LogConfig.AlsoLogToStderr {
		l = io.MultiWriter(l, os.Stderr)
	}

	args := append([]string{version.Program}, os.Args[1:]...)
	cmd := reexec.Command(args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "_K3S_LOG_REEXEC_=true")
	cmd.Stderr = l
	cmd.Stdout = l
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func setupLogging() {
	flag.Set("v", strconv.Itoa(LogConfig.VLevel))
	flag.Set("vmodule", LogConfig.VModule)
	flag.Set("alsologtostderr", strconv.FormatBool(Debug))
	flag.Set("logtostderr", strconv.FormatBool(!Debug))
}
