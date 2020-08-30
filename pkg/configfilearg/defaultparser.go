package configfilearg

import (
	"github.com/rancher/k3s/pkg/version"
	"github.com/sirupsen/logrus"
)

func MustParse(args []string) []string {
	parser := &Parser{
		After:         []string{"server", "agent"},
		FlagNames:     []string{"--config", "-c"},
		EnvName:       version.ProgramUpper + "_CONFIG_FILE",
		DefaultConfig: "/etc/rancher/" + version.Program + "/config.yaml",
	}
	result, err := parser.Parse(args)
	if err != nil {
		logrus.Fatal(err)
	}
	return result
}
