package configfilearg

import (
	"path/filepath"
	"slices"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var DefaultParser = &Parser{
	After:         []string{"server", "agent", "etcd-snapshot:1"},
	ConfigFlags:   []string{"--config", "-c"},
	EnvName:       version.ProgramUpper + "_CONFIG_FILE",
	DefaultConfig: "/etc/rancher/" + version.Program + "/config.yaml",
	ValidFlags:    map[string][]cli.Flag{"server": cmds.ServerFlags, "etcd-snapshot": cmds.EtcdSnapshotFlags},
}

func MustParse(args []string) []string {
	result, err := DefaultParser.Parse(args)
	if err != nil {
		logrus.Fatal(err)
	}
	return result
}

func MustFindString(args []string, target string, commandsWithoutOverride ...string) string {
	overrideFlags := []string{"--help", "-h", "--version", "-v"}
	// Check to see if the command or subcommand being executed supports override flags.
	// Some subcommands such as `k3s ctr` or just `ctr` need to be extracted out even to
	// provide version or help text, and we cannot short-circuit loading the config file. For
	// these commands, treat failure to load the config file as a warning instead of a fatal.
	if len(args) > 0 && filepath.Base(args[0]) == version.Program {
		args = args[1:]
	}
	if len(args) > 0 && slices.Contains(commandsWithoutOverride, filepath.Base(args[0])) {
		overrideFlags = nil
	}

	parser := &Parser{
		OverrideFlags: overrideFlags,
		EnvName:       version.ProgramUpper + "_CONFIG_FILE",
		DefaultConfig: "/etc/rancher/" + version.Program + "/config.yaml",
	}
	result, err := parser.FindString(args, target)
	if err != nil {
		if len(overrideFlags) > 0 {
			logrus.Fatal(err)
		} else {
			logrus.Warn(err)
		}
	}
	return result
}
