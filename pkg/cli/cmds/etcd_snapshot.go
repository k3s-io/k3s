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
				Name:        "etcd-snapshot-name",
				Usage:       "(db) Set the name of etcd snapshots. Default: etcd-snapshot-<unix-timestamp>",
				Destination: &ServerConfig.EtcdSnapshotName,
			},
			&cli.IntFlag{
				Name:        "etcd-snapshot-retention",
				Usage:       "(db) Number of snapshots to retain",
				Destination: &ServerConfig.EtcdSnapshotRetention,
				Value:       defaultSnapshotRentention,
			},
			&cli.StringFlag{
				Name:        "etcd-snapshot-dir",
				Usage:       "(db) Directory to save db snapshots. (Default location: ${data-dir}/db/snapshots)",
				Destination: &ServerConfig.EtcdSnapshotDir,
			},
		},
	}
}
