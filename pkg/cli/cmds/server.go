package cmds

import (
	"context"

	"github.com/rancher/k3s/pkg/version"
	"github.com/urfave/cli"
)

const (
	defaultSnapshotRentention    = 5
	defaultSnapshotIntervalHours = 12
)

type Server struct {
	ClusterCIDR          cli.StringSlice
	AgentToken           string
	AgentTokenFile       string
	Token                string
	TokenFile            string
	ClusterSecret        string
	ServiceCIDR          cli.StringSlice
	ServiceNodePortRange string
	ClusterDNS           cli.StringSlice
	ClusterDomain        string
	// The port which kubectl clients can access k8s
	HTTPSPort int
	// The port which custom k3s API runs on
	SupervisorPort int
	// The port which kube-apiserver runs on
	APIServerPort            int
	APIServerBindAddress     string
	DataDir                  string
	DisableAgent             bool
	KubeConfigOutput         string
	KubeConfigMode           string
	TLSSan                   cli.StringSlice
	BindAddress              string
	ExtraAPIArgs             cli.StringSlice
	ExtraSchedulerArgs       cli.StringSlice
	ExtraControllerArgs      cli.StringSlice
	ExtraCloudControllerArgs cli.StringSlice
	Rootless                 bool
	DatastoreEndpoint        string
	DatastoreCAFile          string
	DatastoreCertFile        string
	DatastoreKeyFile         string
	AdvertiseIP              string
	AdvertisePort            int
	DisableScheduler         bool
	ServerURL                string
	FlannelBackend           string
	DefaultLocalStoragePath  string
	DisableCCM               bool
	DisableNPC               bool
	DisableHelmController    bool
	DisableKubeProxy         bool
	DisableAPIServer         bool
	DisableControllerManager bool
	DisableETCD              bool
	ClusterInit              bool
	ClusterReset             bool
	ClusterResetRestorePath  string
	EncryptSecrets           bool
	SystemDefaultRegistry    string
	StartupHooks             []func(context.Context, <-chan struct{}, string) error
	EtcdSnapshotName         string
	EtcdDisableSnapshots     bool
	EtcdExposeMetrics        bool
	EtcdSnapshotDir          string
	EtcdSnapshotCron         string
	EtcdSnapshotRetention    int
	EtcdS3                   bool
	EtcdS3Endpoint           string
	EtcdS3EndpointCA         string
	EtcdS3SkipSSLVerify      bool
	EtcdS3AccessKey          string
	EtcdS3SecretKey          string
	EtcdS3BucketName         string
	EtcdS3Region             string
	EtcdS3Folder             string
}

var (
	ServerConfig Server
	ClusterCIDR  = cli.StringSliceFlag{
		Name:  "cluster-cidr",
		Usage: "(networking) IPv4/IPv6 network CIDRs to use for pod IPs (default: 10.42.0.0/16)",
		Value: &ServerConfig.ClusterCIDR,
	}
	ServiceCIDR = cli.StringSliceFlag{
		Name:  "service-cidr",
		Usage: "(networking) IPv4/IPv6 network CIDRs to use for service IPs (default: 10.43.0.0/16)",
		Value: &ServerConfig.ServiceCIDR,
	}
	ServiceNodePortRange = cli.StringFlag{
		Name:        "service-node-port-range",
		Usage:       "(networking) Port range to reserve for services with NodePort visibility",
		Destination: &ServerConfig.ServiceNodePortRange,
		Value:       "30000-32767",
	}
	ClusterDNS = cli.StringSliceFlag{
		Name:  "cluster-dns",
		Usage: "(networking) IPv4 Cluster IP for coredns service. Should be in your service-cidr range (default: 10.43.0.10)",
		Value: &ServerConfig.ClusterDNS,
	}
	ClusterDomain = cli.StringFlag{
		Name:        "cluster-domain",
		Usage:       "(networking) Cluster Domain",
		Destination: &ServerConfig.ClusterDomain,
		Value:       "cluster.local",
	}
	ExtraAPIArgs = cli.StringSliceFlag{
		Name:  "kube-apiserver-arg",
		Usage: "(flags) Customized flag for kube-apiserver process",
		Value: &ServerConfig.ExtraAPIArgs,
	}
	ExtraSchedulerArgs = cli.StringSliceFlag{
		Name:  "kube-scheduler-arg",
		Usage: "(flags) Customized flag for kube-scheduler process",
		Value: &ServerConfig.ExtraSchedulerArgs,
	}
	ExtraControllerArgs = cli.StringSliceFlag{
		Name:  "kube-controller-manager-arg",
		Usage: "(flags) Customized flag for kube-controller-manager process",
		Value: &ServerConfig.ExtraControllerArgs,
	}
)

func NewServerCommand(action func(*cli.Context) error) cli.Command {
	return cli.Command{
		Name:      "server",
		Usage:     "Run management server",
		UsageText: appName + " server [OPTIONS]",
		Before:    SetupDebug(CheckSELinuxFlags),
		Action:    action,
		Flags: []cli.Flag{
			ConfigFlag,
			DebugFlag,
			VLevel,
			VModule,
			LogFile,
			AlsoLogToStderr,
			cli.StringFlag{
				Name:        "bind-address",
				Usage:       "(listener) " + version.Program + " bind address (default: 0.0.0.0)",
				Destination: &ServerConfig.BindAddress,
			},
			cli.IntFlag{
				Name:        "https-listen-port",
				Usage:       "(listener) HTTPS listen port",
				Value:       6443,
				Destination: &ServerConfig.HTTPSPort,
			},
			cli.StringFlag{
				Name:        "advertise-address",
				Usage:       "(listener) IPv4 address that apiserver uses to advertise to members of the cluster (default: node-external-ip/node-ip)",
				Destination: &ServerConfig.AdvertiseIP,
			},
			cli.IntFlag{
				Name:        "advertise-port",
				Usage:       "(listener) Port that apiserver uses to advertise to members of the cluster (default: listen-port)",
				Destination: &ServerConfig.AdvertisePort,
			},
			cli.StringSliceFlag{
				Name:  "tls-san",
				Usage: "(listener) Add additional hostnames or IPv4/IPv6 addresses as Subject Alternative Names on the server TLS cert",
				Value: &ServerConfig.TLSSan,
			},
			cli.StringFlag{
				Name:        "data-dir,d",
				Usage:       "(data) Folder to hold state default /var/lib/rancher/" + version.Program + " or ${HOME}/.rancher/" + version.Program + " if not root",
				Destination: &ServerConfig.DataDir,
			},
			ClusterCIDR,
			ServiceCIDR,
			ServiceNodePortRange,
			ClusterDNS,
			ClusterDomain,
			cli.StringFlag{
				Name:        "flannel-backend",
				Usage:       "(networking) One of 'none', 'vxlan', 'ipsec', 'host-gw', or 'wireguard'",
				Destination: &ServerConfig.FlannelBackend,
				Value:       "vxlan",
			},
			cli.StringFlag{
				Name:        "token,t",
				Usage:       "(cluster) Shared secret used to join a server or agent to a cluster",
				Destination: &ServerConfig.Token,
				EnvVar:      version.ProgramUpper + "_TOKEN",
			},
			cli.StringFlag{
				Name:        "token-file",
				Usage:       "(cluster) File containing the cluster-secret/token",
				Destination: &ServerConfig.TokenFile,
				EnvVar:      version.ProgramUpper + "_TOKEN_FILE",
			},
			cli.StringFlag{
				Name:        "write-kubeconfig,o",
				Usage:       "(client) Write kubeconfig for admin client to this file",
				Destination: &ServerConfig.KubeConfigOutput,
				EnvVar:      version.ProgramUpper + "_KUBECONFIG_OUTPUT",
			},
			cli.StringFlag{
				Name:        "write-kubeconfig-mode",
				Usage:       "(client) Write kubeconfig with this mode",
				Destination: &ServerConfig.KubeConfigMode,
				EnvVar:      version.ProgramUpper + "_KUBECONFIG_MODE",
			},
			ExtraAPIArgs,
			ExtraControllerArgs,
			ExtraSchedulerArgs,
			cli.StringSliceFlag{
				Name:  "kube-cloud-controller-manager-arg",
				Usage: "(flags) Customized flag for kube-cloud-controller-manager process",
				Value: &ServerConfig.ExtraCloudControllerArgs,
			},
			cli.StringFlag{
				Name:        "datastore-endpoint",
				Usage:       "(db) Specify etcd, Mysql, Postgres, or Sqlite (default) data source name",
				Destination: &ServerConfig.DatastoreEndpoint,
				EnvVar:      version.ProgramUpper + "_DATASTORE_ENDPOINT",
			},
			cli.StringFlag{
				Name:        "datastore-cafile",
				Usage:       "(db) TLS Certificate Authority file used to secure datastore backend communication",
				Destination: &ServerConfig.DatastoreCAFile,
				EnvVar:      version.ProgramUpper + "_DATASTORE_CAFILE",
			},
			cli.StringFlag{
				Name:        "datastore-certfile",
				Usage:       "(db) TLS certification file used to secure datastore backend communication",
				Destination: &ServerConfig.DatastoreCertFile,
				EnvVar:      version.ProgramUpper + "_DATASTORE_CERTFILE",
			},
			cli.StringFlag{
				Name:        "datastore-keyfile",
				Usage:       "(db) TLS key file used to secure datastore backend communication",
				Destination: &ServerConfig.DatastoreKeyFile,
				EnvVar:      version.ProgramUpper + "_DATASTORE_KEYFILE",
			},
			&cli.BoolFlag{
				Name:        "etcd-expose-metrics",
				Usage:       "(db) Expose etcd metrics to client interface. (Default false)",
				Destination: &ServerConfig.EtcdExposeMetrics,
			},
			&cli.BoolFlag{
				Name:        "etcd-disable-snapshots",
				Usage:       "(db) Disable automatic etcd snapshots",
				Destination: &ServerConfig.EtcdDisableSnapshots,
			},
			&cli.StringFlag{
				Name:        "etcd-snapshot-name",
				Usage:       "(db) Set the base name of etcd snapshots. Default: etcd-snapshot-<unix-timestamp>",
				Destination: &ServerConfig.EtcdSnapshotName,
				Value:       "etcd-snapshot",
			},
			&cli.StringFlag{
				Name:        "etcd-snapshot-schedule-cron",
				Usage:       "(db) Snapshot interval time in cron spec. eg. every 5 hours '* */5 * * *'",
				Destination: &ServerConfig.EtcdSnapshotCron,
				Value:       "0 */12 * * *",
			},
			&cli.IntFlag{
				Name:        "etcd-snapshot-retention",
				Usage:       "(db) Number of snapshots to retain Default: 5",
				Destination: &ServerConfig.EtcdSnapshotRetention,
				Value:       defaultSnapshotRentention,
			},
			&cli.StringFlag{
				Name:        "etcd-snapshot-dir",
				Usage:       "(db) Directory to save db snapshots. (Default location: ${data-dir}/db/snapshots)",
				Destination: &ServerConfig.EtcdSnapshotDir,
			},
			&cli.BoolFlag{
				Name:        "etcd-s3",
				Usage:       "(db) Enable backup to S3",
				Destination: &ServerConfig.EtcdS3,
			},
			&cli.StringFlag{
				Name:        "etcd-s3-endpoint",
				Usage:       "(db) S3 endpoint url",
				Destination: &ServerConfig.EtcdS3Endpoint,
				Value:       "s3.amazonaws.com",
			},
			&cli.StringFlag{
				Name:        "etcd-s3-endpoint-ca",
				Usage:       "(db) S3 custom CA cert to connect to S3 endpoint",
				Destination: &ServerConfig.EtcdS3EndpointCA,
			},
			&cli.BoolFlag{
				Name:        "etcd-s3-skip-ssl-verify",
				Usage:       "(db) Disables S3 SSL certificate validation",
				Destination: &ServerConfig.EtcdS3SkipSSLVerify,
			},
			&cli.StringFlag{
				Name:        "etcd-s3-access-key",
				Usage:       "(db) S3 access key",
				EnvVar:      "AWS_ACCESS_KEY_ID",
				Destination: &ServerConfig.EtcdS3AccessKey,
			},
			&cli.StringFlag{
				Name:        "etcd-s3-secret-key",
				Usage:       "(db) S3 secret key",
				EnvVar:      "AWS_SECRET_ACCESS_KEY",
				Destination: &ServerConfig.EtcdS3SecretKey,
			},
			&cli.StringFlag{
				Name:        "etcd-s3-bucket",
				Usage:       "(db) S3 bucket name",
				Destination: &ServerConfig.EtcdS3BucketName,
			},
			&cli.StringFlag{
				Name:        "etcd-s3-region",
				Usage:       "(db) S3 region / bucket location (optional)",
				Destination: &ServerConfig.EtcdS3Region,
				Value:       "us-east-1",
			},
			&cli.StringFlag{
				Name:        "etcd-s3-folder",
				Usage:       "(db) S3 folder",
				Destination: &ServerConfig.EtcdS3Folder,
			},
			cli.StringFlag{
				Name:        "default-local-storage-path",
				Usage:       "(storage) Default local storage path for local provisioner storage class",
				Destination: &ServerConfig.DefaultLocalStoragePath,
			},
			cli.StringSliceFlag{
				Name:  "disable",
				Usage: "(components) Do not deploy packaged components and delete any deployed components (valid items: " + DisableItems + ")",
			},
			cli.BoolFlag{
				Name:        "disable-scheduler",
				Usage:       "(components) Disable Kubernetes default scheduler",
				Destination: &ServerConfig.DisableScheduler,
			},
			cli.BoolFlag{
				Name:        "disable-cloud-controller",
				Usage:       "(components) Disable " + version.Program + " default cloud controller manager",
				Destination: &ServerConfig.DisableCCM,
			},
			cli.BoolFlag{
				Name:        "disable-kube-proxy",
				Usage:       "(components) Disable running kube-proxy",
				Destination: &ServerConfig.DisableKubeProxy,
			},
			cli.BoolFlag{
				Name:        "disable-network-policy",
				Usage:       "(components) Disable " + version.Program + " default network policy controller",
				Destination: &ServerConfig.DisableNPC,
			},
			cli.BoolFlag{
				Name:        "disable-helm-controller",
				Usage:       "(components) Disable Helm controller",
				Destination: &ServerConfig.DisableHelmController,
			},
			cli.BoolFlag{
				Name:        "disable-apiserver",
				Hidden:      true,
				Usage:       "(experimental/components) Disable running api server",
				Destination: &ServerConfig.DisableAPIServer,
			},
			cli.BoolFlag{
				Name:        "disable-controller-manager",
				Hidden:      true,
				Usage:       "(experimental/components) Disable running kube-controller-manager",
				Destination: &ServerConfig.DisableControllerManager,
			},
			cli.BoolFlag{
				Name:        "disable-etcd",
				Hidden:      true,
				Usage:       "(experimental/components) Disable running etcd",
				Destination: &ServerConfig.DisableETCD,
			},
			NodeNameFlag,
			WithNodeIDFlag,
			NodeLabels,
			NodeTaints,
			ImageCredProvBinDirFlag,
			ImageCredProvConfigFlag,
			DockerFlag,
			CRIEndpointFlag,
			PauseImageFlag,
			SnapshotterFlag,
			PrivateRegistryFlag,
			AirgapExtraRegistryFlag,
			NodeIPFlag,
			NodeExternalIPFlag,
			ResolvConfFlag,
			FlannelIfaceFlag,
			FlannelConfFlag,
			ExtraKubeletArgs,
			ExtraKubeProxyArgs,
			ProtectKernelDefaultsFlag,
			cli.BoolFlag{
				Name:        "rootless",
				Usage:       "(experimental) Run rootless",
				Destination: &ServerConfig.Rootless,
			},
			cli.StringFlag{
				Name:        "agent-token",
				Usage:       "(experimental/cluster) Shared secret used to join agents to the cluster, but not servers",
				Destination: &ServerConfig.AgentToken,
				EnvVar:      version.ProgramUpper + "_AGENT_TOKEN",
			},
			cli.StringFlag{
				Name:        "agent-token-file",
				Usage:       "(experimental/cluster) File containing the agent secret",
				Destination: &ServerConfig.AgentTokenFile,
				EnvVar:      version.ProgramUpper + "_AGENT_TOKEN_FILE",
			},
			cli.StringFlag{
				Name:        "server,s",
				Usage:       "(experimental/cluster) Server to connect to, used to join a cluster",
				EnvVar:      version.ProgramUpper + "_URL",
				Destination: &ServerConfig.ServerURL,
			},
			cli.BoolFlag{
				Name:        "cluster-init",
				Usage:       "(experimental/cluster) Initialize a new cluster using embedded Etcd",
				EnvVar:      version.ProgramUpper + "_CLUSTER_INIT",
				Destination: &ServerConfig.ClusterInit,
			},
			cli.BoolFlag{
				Name:        "cluster-reset",
				Usage:       "(experimental/cluster) Forget all peers and become sole member of a new cluster",
				EnvVar:      version.ProgramUpper + "_CLUSTER_RESET",
				Destination: &ServerConfig.ClusterReset,
			},
			&cli.StringFlag{
				Name:        "cluster-reset-restore-path",
				Usage:       "(db) Path to snapshot file to be restored",
				Destination: &ServerConfig.ClusterResetRestorePath,
			},
			cli.BoolFlag{
				Name:        "secrets-encryption",
				Usage:       "(experimental) Enable Secret encryption at rest",
				Destination: &ServerConfig.EncryptSecrets,
			},
			cli.StringFlag{
				Name:        "system-default-registry",
				Usage:       "(image) Private registry to be used for all system images",
				EnvVar:      version.ProgramUpper + "_SYSTEM_DEFAULT_REGISTRY",
				Destination: &ServerConfig.SystemDefaultRegistry,
			},
			&SELinuxFlag,
			LBServerPortFlag,

			// Hidden/Deprecated flags below

			&DisableSELinuxFlag,
			FlannelFlag,
			cli.StringSliceFlag{
				Name:  "no-deploy",
				Usage: "(deprecated) Do not deploy packaged components (valid items: " + DisableItems + ")",
			},
			cli.StringFlag{
				Name:        "cluster-secret",
				Usage:       "(deprecated) use --token",
				Destination: &ServerConfig.ClusterSecret,
				EnvVar:      version.ProgramUpper + "_CLUSTER_SECRET",
			},
			cli.BoolFlag{
				Name:        "disable-agent",
				Usage:       "Do not run a local agent and register a local kubelet",
				Hidden:      true,
				Destination: &ServerConfig.DisableAgent,
			},
			cli.StringSliceFlag{
				Hidden: true,
				Name:   "kube-controller-arg",
				Usage:  "(flags) Customized flag for kube-controller-manager process",
				Value:  &ServerConfig.ExtraControllerArgs,
			},
			cli.StringSliceFlag{
				Hidden: true,
				Name:   "kube-cloud-controller-arg",
				Usage:  "(flags) Customized flag for kube-cloud-controller-manager process",
				Value:  &ServerConfig.ExtraCloudControllerArgs,
			},
		},
	}
}
