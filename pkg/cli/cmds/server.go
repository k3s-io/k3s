package cmds

import (
	"github.com/rancher/k3s/pkg/version"
	"github.com/rancher/spur/cli"
	"github.com/rancher/spur/cli/altsrc"
)

const (
	DisableItems = "coredns, servicelb, traefik, local-storage, metrics-server"
)

type Server struct {
	ClusterCIDR    string
	AgentToken     string
	AgentTokenFile string
	Token          string
	TokenFile      string
	ClusterSecret  string
	ServiceCIDR    string
	ClusterDNS     string
	ClusterDomain  string
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
	TLSSan                   []string
	BindAddress              string
	ExtraAPIArgs             []string
	ExtraSchedulerArgs       []string
	ExtraControllerArgs      []string
	ExtraCloudControllerArgs []string
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
	DisableKubeProxy         bool
	ClusterInit              bool
	ClusterReset             bool
	EncryptSecrets           bool
}

var ServerConfig Server

func NewServerCommand(action func(*cli.Context) error) *cli.Command {
	return &cli.Command{
		Name:      "server",
		Usage:     "Run management server",
		UsageText: appName + " server [OPTIONS]",
		Before:    DebugContext(cli.InitAllInputSource(altsrc.NewConfigFromFlag(ConfigFlag.Name))),
		Action:    InitLogging(action),
		Flags: []cli.Flag{
			&ConfigFlag,
			&DebugFlag,
			&VLevel,
			&VModule,
			&LogFile,
			&AlsoLogToStderr,
			&cli.StringFlag{
				Name:        "bind-address",
				Usage:       "(listener) " + version.Program + " bind address (default: 0.0.0.0)",
				Destination: &ServerConfig.BindAddress,
			},
			&cli.IntFlag{
				Name:        "https-listen-port",
				Usage:       "(listener) HTTPS listen port",
				Value:       6443,
				Destination: &ServerConfig.HTTPSPort,
			},
			&cli.StringFlag{
				Name:        "advertise-address",
				Usage:       "(listener) IP address that apiserver uses to advertise to members of the cluster (default: node-external-ip/node-ip)",
				Destination: &ServerConfig.AdvertiseIP,
			},
			&cli.IntFlag{
				Name:        "advertise-port",
				Usage:       "(listener) Port that apiserver uses to advertise to members of the cluster (default: listen-port)",
				Destination: &ServerConfig.AdvertisePort,
			},
			&cli.StringSliceFlag{
				Name:        "tls-san",
				Usage:       "(listener) Add additional hostname or IP as a Subject Alternative Name in the TLS cert",
				Destination: &ServerConfig.TLSSan,
			},
			&cli.StringFlag{
				Name:        "data-dir,d",
				Usage:       "(data) Folder to hold state default /var/lib/rancher/" + version.Program + " or ${HOME}/.rancher/" + version.Program + " if not root",
				Destination: &ServerConfig.DataDir,
			},
			&cli.StringFlag{
				Name:        "cluster-cidr",
				Usage:       "(networking) Network CIDR to use for pod IPs",
				Destination: &ServerConfig.ClusterCIDR,
				Value:       "10.42.0.0/16",
			},
			&cli.StringFlag{
				Name:        "service-cidr",
				Usage:       "(networking) Network CIDR to use for services IPs",
				Destination: &ServerConfig.ServiceCIDR,
				Value:       "10.43.0.0/16",
			},
			&cli.StringFlag{
				Name:        "cluster-dns",
				Usage:       "(networking) Cluster IP for coredns service. Should be in your service-cidr range (default: 10.43.0.10)",
				Destination: &ServerConfig.ClusterDNS,
				Value:       "",
			},
			&cli.StringFlag{
				Name:        "cluster-domain",
				Usage:       "(networking) Cluster Domain",
				Destination: &ServerConfig.ClusterDomain,
				Value:       "cluster.local",
			},
			&cli.StringFlag{
				Name:        "flannel-backend",
				Usage:       "(networking) One of 'none', 'vxlan', 'ipsec', 'host-gw', or 'wireguard'",
				Destination: &ServerConfig.FlannelBackend,
				Value:       "vxlan",
			},
			&cli.StringFlag{
				Name:        "token,t",
				Usage:       "(cluster) Shared secret used to join a server or agent to a cluster",
				Destination: &ServerConfig.Token,
				EnvVars:     []string{version.ProgramUpper + "_TOKEN"},
			},
			&cli.StringFlag{
				Name:        "token-file",
				Usage:       "(cluster) File containing the cluster-secret/token",
				Destination: &ServerConfig.TokenFile,
				EnvVars:     []string{version.ProgramUpper + "_TOKEN_FILE"},
			},
			&cli.StringFlag{
				Name:        "write-kubeconfig,o",
				Usage:       "(client) Write kubeconfig for admin client to this file",
				Destination: &ServerConfig.KubeConfigOutput,
				EnvVars:     []string{version.ProgramUpper + "_KUBECONFIG_OUTPUT"},
			},
			&cli.StringFlag{
				Name:        "write-kubeconfig-mode",
				Usage:       "(client) Write kubeconfig with this mode",
				Destination: &ServerConfig.KubeConfigMode,
				EnvVars:     []string{version.ProgramUpper + "_KUBECONFIG_MODE"},
			},
			&cli.StringSliceFlag{
				Name:        "kube-apiserver-arg",
				Usage:       "(flags) Customized flag for kube-apiserver process",
				Destination: &ServerConfig.ExtraAPIArgs,
			},
			&cli.StringSliceFlag{
				Name:        "kube-scheduler-arg",
				Usage:       "(flags) Customized flag for kube-scheduler process",
				Destination: &ServerConfig.ExtraSchedulerArgs,
			},
			&cli.StringSliceFlag{
				Name:        "kube-controller-manager-arg",
				Usage:       "(flags) Customized flag for kube-controller-manager process",
				Destination: &ServerConfig.ExtraControllerArgs,
			},
			&cli.StringSliceFlag{
				Name:        "kube-cloud-controller-manager-arg",
				Usage:       "(flags) Customized flag for kube-cloud-controller-manager process",
				Destination: &ServerConfig.ExtraCloudControllerArgs,
			},
			&cli.StringFlag{
				Name:        "datastore-endpoint",
				Usage:       "(db) Specify etcd, Mysql, Postgres, or Sqlite (default) data source name",
				Destination: &ServerConfig.DatastoreEndpoint,
				EnvVars:     []string{version.ProgramUpper + "_DATASTORE_ENDPOINT"},
			},
			&cli.StringFlag{
				Name:        "datastore-cafile",
				Usage:       "(db) TLS Certificate Authority file used to secure datastore backend communication",
				Destination: &ServerConfig.DatastoreCAFile,
				EnvVars:     []string{version.ProgramUpper + "_DATASTORE_CAFILE"},
			},
			&cli.StringFlag{
				Name:        "datastore-certfile",
				Usage:       "(db) TLS certification file used to secure datastore backend communication",
				Destination: &ServerConfig.DatastoreCertFile,
				EnvVars:     []string{version.ProgramUpper + "_DATASTORE_CERTFILE"},
			},
			&cli.StringFlag{
				Name:        "datastore-keyfile",
				Usage:       "(db) TLS key file used to secure datastore backend communication",
				Destination: &ServerConfig.DatastoreKeyFile,
				EnvVars:     []string{version.ProgramUpper + "_DATASTORE_KEYFILE"},
			},
			&cli.StringFlag{
				Name:        "default-local-storage-path",
				Usage:       "(storage) Default local storage path for local provisioner storage class",
				Destination: &ServerConfig.DefaultLocalStoragePath,
			},
			&cli.StringSliceFlag{
				Name:  "disable",
				Usage: "(components) Do not deploy packaged components and delete any deployed components (valid items: " + DisableItems + ")",
			},
			&cli.BoolFlag{
				Name:        "disable-scheduler",
				Usage:       "(components) Disable Kubernetes default scheduler",
				Destination: &ServerConfig.DisableScheduler,
			},
			&cli.BoolFlag{
				Name:        "disable-cloud-controller",
				Usage:       "(components) Disable " + version.Program + " default cloud controller manager",
				Destination: &ServerConfig.DisableCCM,
			},
			&cli.BoolFlag{
				Name:        "disable-kube-proxy",
				Usage:       "(components) Disable running kube-proxy",
				Destination: &ServerConfig.DisableKubeProxy,
			},
			&cli.BoolFlag{
				Name:        "disable-network-policy",
				Usage:       "(components) Disable " + version.Program + " default network policy controller",
				Destination: &ServerConfig.DisableNPC,
			},
			&NodeNameFlag,
			&WithNodeIDFlag,
			&NodeLabels,
			&NodeTaints,
			&DockerFlag,
			&DisableSELinuxFlag,
			&CRIEndpointFlag,
			&PauseImageFlag,
			&SnapshotterFlag,
			&PrivateRegistryFlag,
			&NodeIPFlag,
			&NodeExternalIPFlag,
			&ResolvConfFlag,
			&FlannelIfaceFlag,
			&FlannelConfFlag,
			&ExtraKubeletArgs,
			&ExtraKubeProxyArgs,
			&ProtectKernelDefaultsFlag,
			&cli.BoolFlag{
				Name:        "rootless",
				Usage:       "(experimental) Run rootless",
				Destination: &ServerConfig.Rootless,
			},
			&cli.StringFlag{
				Name:        "agent-token",
				Usage:       "(experimental/cluster) Shared secret used to join agents to the cluster, but not servers",
				Destination: &ServerConfig.AgentToken,
				EnvVars:     []string{version.ProgramUpper + "_AGENT_TOKEN"},
			},
			&cli.StringFlag{
				Name:        "agent-token-file",
				Usage:       "(experimental/cluster) File containing the agent secret",
				Destination: &ServerConfig.AgentTokenFile,
				EnvVars:     []string{version.ProgramUpper + "_AGENT_TOKEN_FILE"},
			},
			&cli.StringFlag{
				Name:        "server,s",
				Usage:       "(experimental/cluster) Server to connect to, used to join a cluster",
				EnvVars:     []string{version.ProgramUpper + "_URL"},
				Destination: &ServerConfig.ServerURL,
			},
			&cli.BoolFlag{
				Name:        "cluster-init",
				Hidden:      hideDqlite,
				Usage:       "(experimental/cluster) Initialize new cluster master",
				EnvVars:     []string{version.ProgramUpper + "_CLUSTER_INIT"},
				Destination: &ServerConfig.ClusterInit,
			},
			&cli.BoolFlag{
				Name:        "cluster-reset",
				Hidden:      hideDqlite,
				Usage:       "(experimental/cluster) Forget all peers and become a single cluster new cluster master",
				EnvVars:     []string{version.ProgramUpper + "_CLUSTER_RESET"},
				Destination: &ServerConfig.ClusterReset,
			},
			&cli.BoolFlag{
				Name:        "secrets-encryption",
				Usage:       "(experimental) Enable Secret encryption at rest",
				Destination: &ServerConfig.EncryptSecrets,
			},

			// Hidden/Deprecated flags below

			&FlannelFlag,
			&cli.StringSliceFlag{
				Name:  "no-deploy",
				Usage: "(deprecated) Do not deploy packaged components (valid items: " + DisableItems + ")",
			},
			&cli.StringFlag{
				Name:        "cluster-secret",
				Usage:       "(deprecated) use --token",
				Destination: &ServerConfig.ClusterSecret,
				EnvVars:     []string{version.ProgramUpper + "_CLUSTER_SECRET"},
			},
			&cli.BoolFlag{
				Name:        "disable-agent",
				Usage:       "Do not run a local agent and register a local kubelet",
				Hidden:      true,
				Destination: &ServerConfig.DisableAgent,
			},
			&cli.StringSliceFlag{
				Hidden:      true,
				Name:        "kube-controller-arg",
				Usage:       "(flags) Customized flag for kube-controller-manager process",
				Destination: &ServerConfig.ExtraControllerArgs,
			},
			&cli.StringSliceFlag{
				Hidden:      true,
				Name:        "kube-cloud-controller-arg",
				Usage:       "(flags) Customized flag for kube-cloud-controller-manager process",
				Destination: &ServerConfig.ExtraCloudControllerArgs,
			},
		},
	}
}
