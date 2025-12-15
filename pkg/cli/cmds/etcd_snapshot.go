package cmds

import (
	"time"

	"github.com/k3s-io/k3s/pkg/version"
	"github.com/urfave/cli/v2"
)

const EtcdSnapshotCommand = "etcd-snapshot"

var EtcdSnapshotFlags = []cli.Flag{
	DebugFlag,
	ConfigFlag,
	LogFile,
	AlsoLogToStderr,
	&cli.StringFlag{
		Name:        "node-name",
		Usage:       "(agent/node) Node name",
		EnvVars:     []string{version.ProgramUpper + "_NODE_NAME"},
		Destination: &AgentConfig.NodeName,
	},
	DataDirFlag,
	&cli.StringFlag{
		Name:        "etcd-token",
		Aliases:     []string{"t"},
		Usage:       "(cluster) Shared secret used to authenticate to etcd server",
		Destination: &ServerConfig.Token,
	},
	&cli.StringFlag{
		Name:        "etcd-server",
		Aliases:     []string{"s"},
		Usage:       "(cluster) Server with etcd role to connect to for snapshot management operations",
		Value:       "https://127.0.0.1:6443",
		Destination: &ServerConfig.ServerURL,
	},
	&cli.StringFlag{
		Name:        "dir",
		Aliases:     []string{"etcd-snapshot-dir"},
		Usage:       "(db) Directory to save etcd on-demand snapshot. (default: ${data-dir}/server/db/snapshots)",
		Destination: &ServerConfig.EtcdSnapshotDir,
	},
	&cli.StringFlag{
		Name:        "name",
		Usage:       "(db) Set the base name of the etcd on-demand snapshot, appended with UNIX timestamp",
		Destination: &ServerConfig.EtcdSnapshotName,
		Value:       "on-demand",
	},
	&cli.BoolFlag{
		Name:        "snapshot-compress",
		Aliases:     []string{"etcd-snapshot-compress"},
		Usage:       "(db) Compress etcd snapshot",
		Destination: &ServerConfig.EtcdSnapshotCompress,
	},
	&cli.IntFlag{
		Name:        "snapshot-retention,",
		Aliases:     []string{"etcd-snapshot-retention"},
		Usage:       "(db) Number of snapshots to retain.",
		Destination: &ServerConfig.EtcdSnapshotRetention,
		Value:       defaultSnapshotRentention,
	},
	&cli.BoolFlag{
		Name:        "s3",
		Aliases:     []string{"etcd-s3"},
		Usage:       "(db) Enable backup to S3",
		Destination: &ServerConfig.EtcdS3,
	},
	&cli.StringFlag{
		Name:        "s3-endpoint",
		Aliases:     []string{"etcd-s3-endpoint"},
		Usage:       "(db) S3 endpoint url",
		Destination: &ServerConfig.EtcdS3Endpoint,
		Value:       "s3.amazonaws.com",
	},
	&cli.StringFlag{
		Name:        "s3-endpoint-ca",
		Aliases:     []string{"etcd-s3-endpoint-ca"},
		Usage:       "(db) S3 custom CA cert to connect to S3 endpoint",
		Destination: &ServerConfig.EtcdS3EndpointCA,
	},
	&cli.BoolFlag{
		Name:        "s3-skip-ssl-verify",
		Aliases:     []string{"etcd-s3-skip-ssl-verify"},
		Usage:       "(db) Disables S3 SSL certificate validation",
		Destination: &ServerConfig.EtcdS3SkipSSLVerify,
	},
	&cli.StringFlag{
		Name:        "s3-access-key",
		Aliases:     []string{"etcd-s3-access-key"},
		Usage:       "(db) S3 access key",
		EnvVars:     []string{"AWS_ACCESS_KEY_ID"},
		Destination: &ServerConfig.EtcdS3AccessKey,
	},
	&cli.StringFlag{
		Name:        "s3-secret-key",
		Aliases:     []string{"etcd-s3-secret-key"},
		Usage:       "(db) S3 secret key",
		EnvVars:     []string{"AWS_SECRET_ACCESS_KEY"},
		Destination: &ServerConfig.EtcdS3SecretKey,
	},
	&cli.StringFlag{
		Name:        "s3-session-token",
		Aliases:     []string{"etcd-s3-session-token"},
		Usage:       "(db) S3 session token",
		EnvVars:     []string{"AWS_SESSION_TOKEN"},
		Destination: &ServerConfig.EtcdS3SessionToken,
	},
	&cli.StringFlag{
		Name:        "s3-bucket",
		Aliases:     []string{"etcd-s3-bucket"},
		Usage:       "(db) S3 bucket name",
		Destination: &ServerConfig.EtcdS3BucketName,
	},
	&cli.StringFlag{
		Name:        "s3-bucket-lookup-type",
		Aliases:     []string{"etcd-s3-bucket-lookup-type"},
		Usage:       "(db) S3 bucket lookup type, one of 'auto', 'dns', 'path'; default is 'auto' if not set",
		Destination: &ServerConfig.EtcdS3BucketLookupType,
	},
	&cli.StringFlag{
		Name:        "s3-region",
		Aliases:     []string{"etcd-s3-region"},
		Usage:       "(db) S3 region / bucket location (optional)",
		Destination: &ServerConfig.EtcdS3Region,
		Value:       "us-east-1",
	},
	&cli.IntFlag{
		Name:        "s3-retention",
		Aliases:     []string{"etcd-s3-retention"},
		Usage:       "(db) Number of s3 snapshots to retain.",
		Destination: &ServerConfig.EtcdS3Retention,
		Value:       defaultSnapshotRentention,
	},
	&cli.StringFlag{
		Name:        "s3-folder",
		Aliases:     []string{"etcd-s3-folder"},
		Usage:       "(db) S3 folder",
		Destination: &ServerConfig.EtcdS3Folder,
	},
	&cli.StringFlag{
		Name:        "s3-proxy",
		Aliases:     []string{"etcd-s3-proxy"},
		Usage:       "(db) Proxy server to use when connecting to S3, overriding any proxy-releated environment variables",
		Destination: &ServerConfig.EtcdS3Proxy,
	},
	&cli.StringFlag{
		Name:        "s3-config-secret",
		Aliases:     []string{"etcd-s3-config-secret"},
		Usage:       "(db) Name of secret in the kube-system namespace used to configure S3, if etcd-s3 is enabled and no other etcd-s3 options are set",
		Destination: &ServerConfig.EtcdS3ConfigSecret,
	},
	&cli.BoolFlag{
		Name:        "s3-insecure",
		Aliases:     []string{"etcd-s3-insecure"},
		Usage:       "(db) Disables S3 over HTTPS",
		Destination: &ServerConfig.EtcdS3Insecure,
	},
	&cli.DurationFlag{
		Name:        "s3-timeout",
		Aliases:     []string{"etcd-s3-timeout"},
		Usage:       "(db) S3 timeout",
		Destination: &ServerConfig.EtcdS3Timeout,
		Value:       5 * time.Minute,
	},
}

func NewEtcdSnapshotCommands(deleteFunc, listFunc, pruneFunc, saveFunc func(ctx *cli.Context) error) *cli.Command {
	return &cli.Command{
		Name:            EtcdSnapshotCommand,
		Usage:           "Manage etcd snapshots",
		SkipFlagParsing: false,
		Subcommands: []*cli.Command{
			{
				Name:            "save",
				Usage:           "Trigger an immediate etcd snapshot",
				SkipFlagParsing: false,
				Action:          saveFunc,
				Flags:           EtcdSnapshotFlags,
			},
			{
				Name:            "delete",
				Usage:           "Delete given snapshot(s)",
				SkipFlagParsing: false,
				Action:          deleteFunc,
				Flags:           EtcdSnapshotFlags,
			},
			{
				Name:            "ls",
				Aliases:         []string{"list", "l"},
				Usage:           "List snapshots",
				SkipFlagParsing: false,
				Action:          listFunc,
				Flags: append(EtcdSnapshotFlags, &cli.StringFlag{
					Name:        "output",
					Aliases:     []string{"o"},
					Usage:       "(db) List format. Default: standard. Optional: json",
					Destination: &ServerConfig.EtcdListFormat,
				}),
			},
			{
				Name:            "prune",
				Usage:           "Remove snapshots that match the name prefix that exceed the configured retention count",
				SkipFlagParsing: false,
				Action:          pruneFunc,
				Flags:           EtcdSnapshotFlags,
			},
		},
		Flags: EtcdSnapshotFlags,
	}
}
