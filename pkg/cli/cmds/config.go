package cmds

import (
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/urfave/cli/v2"
)

var (
	// ConfigFlag is here to show to the user, but the actually processing is done by configfileargs before
	// call urfave
	ConfigFlag = &cli.StringFlag{
		Name:    "config",
		Aliases: []string{"c"},
		Usage:   "(config) Load configuration from `FILE`",
		EnvVars: []string{version.ProgramUpper + "_CONFIG_FILE"},
		Value:   "/etc/rancher/" + version.Program + "/config.yaml",
	}
)
