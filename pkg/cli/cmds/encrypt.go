package cmds

import (
	"github.com/urfave/cli"
)

func NewEncryptCommand(action func(*cli.Context) error, subcommands []cli.Command) cli.Command {
	return cli.Command{
		Name:            "encrypt",
		Usage:           "(experimental) Enable secrets encryption at rest",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Action:          action,
		Subcommands:     subcommands,
	}
}

func NewEncryptSubcommands(prepare, rotate, reencrypt func(ctx *cli.Context) error) []cli.Command {
	return []cli.Command{
		{
			Name:            "prepare",
			Usage:           "(experimental) Prepare for encryption keys rotation",
			SkipFlagParsing: false,
			SkipArgReorder:  true,
			Action:          prepare,
			// Flags:           EtcdSnapshotFlags,
		},
		{
			Name:            "rotate",
			Usage:           "(experimental) Rotate secrets encryption keys",
			SkipFlagParsing: false,
			SkipArgReorder:  true,
			Action:          rotate,
		},
		{
			Name:            "reencrypt",
			Usage:           "(experimental) Reencrypt all data with new encryption key",
			SkipFlagParsing: false,
			SkipArgReorder:  true,
			Action:          reencrypt,
		},
	}
}
