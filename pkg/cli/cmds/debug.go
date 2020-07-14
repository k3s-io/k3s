package cmds

import (
	"github.com/rancher/k3s/pkg/version"
	"github.com/rancher/spur/cli"
	"github.com/sirupsen/logrus"
)

var (
	Debug     = false
	DebugFlag = cli.BoolFlag{
		Name:        "debug",
		Usage:       "(logging) Turn on debug logs",
		Destination: &Debug,
		EnvVars:     []string{version.ProgramUpper + "_DEBUG"},
	}
)

func DebugContext(f func(*cli.Context) error) func(ctx *cli.Context) error {
	return func(ctx *cli.Context) error {
		if f != nil {
			if err := f(ctx); err != nil {
				return err
			}
		}
		if Debug {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}
}
