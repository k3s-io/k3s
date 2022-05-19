package cmds

import (
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/urfave/cli"
)

var (
	// ConfigFlag is here to show to the user, but the actually processing is done by configfileargs before
	// call urfave
	ConfigFlag = cli.StringFlag{
		Name:   "config,c",
		Usage:  "(config) Load configuration from `FILE`",
		EnvVar: version.ProgramUpper + "_CONFIG_FILE",
		Value:  "/etc/rancher/" + version.Program + "/config.yaml",
	}
)
