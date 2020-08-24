package cmds

import (
	"github.com/rancher/spur/cli"
)

var (
	DefaultConfig = "/etc/rancher/k3s/config.yaml"
	ConfigFlag    = cli.StringFlag{
		Name:    "config",
		Aliases: []string{"c"},
		Usage:   "(config) Load configuration from `FILE`",
		EnvVars: []string{"K3S_CONFIG_FILE"},
		Value:   DefaultConfig,
	}
)
