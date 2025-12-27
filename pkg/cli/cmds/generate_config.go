package cmds

import (
	"github.com/urfave/cli/v2"
)

const GenerateConfigCommand = "generate-config"

type GenerateConfig struct {
	Output     string
	ConfigType string
	FromConfig string
}

var (
	GenerateConfigConfig = GenerateConfig{}
	GenerateConfigFlags  = []cli.Flag{
		DebugFlag,
		&cli.StringFlag{
			Name:        "output",
			Aliases:     []string{"o"},
			Usage:       "Output file path (default: stdout)",
			Destination: &GenerateConfigConfig.Output,
		},
		&cli.StringFlag{
			Name:        "type",
			Aliases:     []string{"t"},
			Usage:       "Config type to generate (server or agent)",
			Value:       "server",
			Destination: &GenerateConfigConfig.ConfigType,
		},
		&cli.StringFlag{
			Name:        "from-config",
			Usage:       "Read existing config file to show current values",
			Destination: &GenerateConfigConfig.FromConfig,
		},
	}
)

func NewGenerateConfigCommand(action func(*cli.Context) error) *cli.Command {
	return &cli.Command{
		Name:            GenerateConfigCommand,
		Usage:           "Generate an example k3s config file",
		SkipFlagParsing: false,
		Action:          action,
		Flags:           GenerateConfigFlags,
	}
}
