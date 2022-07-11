package cmds

import (
	"time"

	"github.com/k3s-io/k3s/pkg/version"
	"github.com/urfave/cli"
)

const EtcdSnapshotCommand = "etcd-snapshot"

var EtcdSnapshotFlags = []cli.Flag{
	DebugFlag,
	ConfigFlag,
	LogFile,
	AlsoLogToStderr,
	cli.StringFlag{
		Name:        "node-name",
		Usage:       "(agent/node) Node name",
		EnvVar:      version.ProgramUpper + "_NODE_NAME",
		Destination: &AgentConfig.NodeName,
	},
	DataDirFlag,
	&cli.StringFlag{
		Name:        "dir,etcd-snapshot-dir",
		Usage:       "(db) Directory to save etcd on-demand snapshot. (default: ${data-dir}/db/snapshots)",
		Destination: &ServerConfig.EtcdSnapshotDir,
	},
	&cli.StringFlag{
		Name:        "name",
		Usage:       "(db) Set the base name of the etcd on-demand snapshot (appended with UNIX timestamp).",
		Destination: &ServerConfig.EtcdSnapshotName,
		Value:       "on-demand",
	},
	&cli.BoolFlag{
		Name:        "snapshot-compress,etcd-snapshot-compress",
		Usage:       "(db) Compress etcd snapshot",
		Destination: &ServerConfig.EtcdSnapshotCompress,
	},
	&cli.BoolFlag{
		Name:        "s3,etcd-s3",
		Usage:       "(db) Enable backup to S3",
		Destination: &ServerConfig.EtcdS3,
	},
	&cli.StringFlag{
		Name:        "s3-endpoint,etcd-s3-endpoint",
		Usage:       "(db) S3 endpoint url",
		Destination: &ServerConfig.EtcdS3Endpoint,
		Value:       "s3.amazonaws.com",
	},
	&cli.StringFlag{
		Name:        "s3-endpoint-ca,etcd-s3-endpoint-ca",
		Usage:       "(db) S3 custom CA cert to connect to S3 endpoint",
		Destination: &ServerConfig.EtcdS3EndpointCA,
	},
	&cli.BoolFlag{
		Name:        "s3-skip-ssl-verify,etcd-s3-skip-ssl-verify",
		Usage:       "(db) Disables S3 SSL certificate validation",
		Destination: &ServerConfig.EtcdS3SkipSSLVerify,
	},
	&cli.StringFlag{
		Name:        "s3-access-key,etcd-s3-access-key",
		Usage:       "(db) S3 access key",
		EnvVar:      "AWS_ACCESS_KEY_ID",
		Destination: &ServerConfig.EtcdS3AccessKey,
	},
	&cli.StringFlag{
		Name:        "s3-secret-key,etcd-s3-secret-key",
		Usage:       "(db) S3 secret key",
		EnvVar:      "AWS_SECRET_ACCESS_KEY",
		Destination: &ServerConfig.EtcdS3SecretKey,
	},
	&cli.StringFlag{
		Name:        "s3-bucket,etcd-s3-bucket",
		Usage:       "(db) S3 bucket name",
		Destination: &ServerConfig.EtcdS3BucketName,
	},
	&cli.StringFlag{
		Name:        "s3-region,etcd-s3-region",
		Usage:       "(db) S3 region / bucket location (optional)",
		Destination: &ServerConfig.EtcdS3Region,
		Value:       "us-east-1",
	},
	&cli.StringFlag{
		Name:        "s3-folder,etcd-s3-folder",
		Usage:       "(db) S3 folder",
		Destination: &ServerConfig.EtcdS3Folder,
	},
	&cli.BoolFlag{
		Name:        "s3-insecure,etcd-s3-insecure",
		Usage:       "(db) Disables S3 over HTTPS",
		Destination: &ServerConfig.EtcdS3Insecure,
	},
	&cli.DurationFlag{
		Name:        "s3-timeout,etcd-s3-timeout",
		Usage:       "(db) S3 timeout",
		Destination: &ServerConfig.EtcdS3Timeout,
		Value:       5 * time.Minute,
	},
}

func NewEtcdSnapshotCommand(action func(*cli.Context) error, subcommands []cli.Command) cli.Command {
	return cli.Command{
		Name:            EtcdSnapshotCommand,
		Usage:           "Trigger an immediate etcd snapshot",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Action:          action,
		Subcommands:     subcommands,
		Flags:           EtcdSnapshotFlags,
	}
}

func NewEtcdSnapshotSubcommands(delete, list, prune, save func(ctx *cli.Context) error) []cli.Command {
	return []cli.Command{
		{
			Name:            "delete",
			Usage:           "Delete given snapshot(s)",
			SkipFlagParsing: false,
			SkipArgReorder:  true,
			Action:          delete,
			Flags:           EtcdSnapshotFlags,
		},
		{
			Name:            "ls",
			Aliases:         []string{"list", "l"},
			Usage:           "List snapshots",
			SkipFlagParsing: false,
			SkipArgReorder:  true,
			Action:          list,
			Flags: append(EtcdSnapshotFlags, &cli.StringFlag{
				Name:        "o,output",
				Usage:       "(db) List format. Default: standard. Optional: json",
				Destination: &ServerConfig.EtcdListFormat,
			}),
		},
		{
			Name:            "prune",
			Usage:           "Remove snapshots that match the name prefix that exceed the configured retention count",
			SkipFlagParsing: false,
			SkipArgReorder:  true,
			Action:          prune,
			Flags: append(EtcdSnapshotFlags, &cli.IntFlag{
				Name:        "snapshot-retention",
				Usage:       "(db) Number of snapshots to retain.",
				Destination: &ServerConfig.EtcdSnapshotRetention,
				Value:       defaultSnapshotRentention,
			}),
		},
		{
			Name:            "save",
			Usage:           "Trigger an immediate etcd snapshot",
			SkipFlagParsing: false,
			SkipArgReorder:  true,
			Action:          save,
			Flags:           EtcdSnapshotFlags,
		},
	}
}
