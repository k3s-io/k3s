package cmds

import (
	"github.com/rancher/k3s/pkg/version"
	"github.com/urfave/cli"
)

const EtcdSnapshotCommand = "etcd-snapshot"

func NewEtcdSnapshotCommand(action func(*cli.Context) error) cli.Command {
	return cli.Command{
		Name:            EtcdSnapshotCommand,
		Usage:           "Trigger an immediate etcd snapshot",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Action:          action,
		Flags: []cli.Flag{
			DebugFlag,
			LogFile,
			AlsoLogToStderr,
			cli.StringFlag{
				Name:        "data-dir,d",
				Usage:       "(data) Folder to hold state default /var/lib/rancher/" + version.Program + " or ${HOME}/.rancher/" + version.Program + " if not root",
				Destination: &ServerConfig.DataDir,
			},
			&cli.StringFlag{
				Name:        "name",
				Usage:       "(db) Set the base name of the etcd on-demand snapshot (appended with UNIX timestamp).",
				Destination: &ServerConfig.EtcdSnapshotName,
				Value:       "on-demand",
			},
			&cli.StringFlag{
				Name:        "dir",
				Usage:       "(db) Directory to save etcd on-demand snapshot. (default: ${data-dir}/db/snapshots)",
				Destination: &ServerConfig.EtcdSnapshotDir,
			},
		},
	}
}
