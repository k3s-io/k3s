package cmds

import (
	"os"
	"path/filepath"

	"github.com/k3s-io/k3s/pkg/version"
	"github.com/urfave/cli"
)

type Agent struct {
	Token                    string
	TokenFile                string
	ClusterSecret            string
	ServerURL                string
	APIAddressCh             chan []string
	DisableLoadBalancer      bool
	DisableServiceLB         bool
	ETCDAgent                bool
	LBServerPort             int
	ResolvConf               string
	DataDir                  string
	BindAddress              string
	NodeIP                   cli.StringSlice
	NodeExternalIP           cli.StringSlice
	NodeInternalDNS          cli.StringSlice
	NodeExternalDNS          cli.StringSlice
	NodeName                 string
	PauseImage               string
	Snapshotter              string
	Docker                   bool
	ContainerdNoDefault      bool
	ContainerRuntimeEndpoint string
	DefaultRuntime           string
	ImageServiceEndpoint     string
	FlannelIface             string
	FlannelConf              string
	FlannelCniConfFile       string
	VPNAuth                  string
	VPNAuthFile              string
	Debug                    bool
	EnablePProf              bool
	Rootless                 bool
	RootlessAlreadyUnshared  bool
	WithNodeID               bool
	EnableSELinux            bool
	ProtectKernelDefaults    bool
	ClusterReset             bool
	PrivateRegistry          string
	SystemDefaultRegistry    string
	AirgapExtraRegistry      cli.StringSlice
	ExtraKubeletArgs         cli.StringSlice
	ExtraKubeProxyArgs       cli.StringSlice
	Labels                   cli.StringSlice
	Taints                   cli.StringSlice
	ImageCredProvBinDir      string
	ImageCredProvConfig      string
	ContainerRuntimeReady    chan<- struct{}
	AgentShared
}

type AgentShared struct {
	NodeIP string
}

var (
	appName        = filepath.Base(os.Args[0])
	AgentConfig    Agent
	AgentTokenFlag = &cli.StringFlag{
		Name:        "token,t",
		Usage:       "(cluster) Token to use for authentication",
		EnvVar:      version.ProgramUpper + "_TOKEN",
		Destination: &AgentConfig.Token,
	}
	NodeIPFlag = &cli.StringSliceFlag{
		Name:  "node-ip,i",
		Usage: "(agent/networking) IPv4/IPv6 addresses to advertise for node",
		Value: &AgentConfig.NodeIP,
	}
	NodeExternalIPFlag = &cli.StringSliceFlag{
		Name:  "node-external-ip",
		Usage: "(agent/networking) IPv4/IPv6 external IP addresses to advertise for node",
		Value: &AgentConfig.NodeExternalIP,
	}
	NodeInternalDNSFlag = &cli.StringSliceFlag{
		Name:  "node-internal-dns",
		Usage: "(agent/networking) internal DNS addresses to advertise for node",
		Value: &AgentConfig.NodeInternalDNS,
	}
	NodeExternalDNSFlag = &cli.StringSliceFlag{
		Name:  "node-external-dns",
		Usage: "(agent/networking) external DNS addresses to advertise for node",
		Value: &AgentConfig.NodeExternalDNS,
	}
	NodeNameFlag = &cli.StringFlag{
		Name:        "node-name",
		Usage:       "(agent/node) Node name",
		EnvVar:      version.ProgramUpper + "_NODE_NAME",
		Destination: &AgentConfig.NodeName,
	}
	WithNodeIDFlag = &cli.BoolFlag{
		Name:        "with-node-id",
		Usage:       "(agent/node) Append id to node name",
		Destination: &AgentConfig.WithNodeID,
	}
	ProtectKernelDefaultsFlag = &cli.BoolFlag{
		Name:        "protect-kernel-defaults",
		Usage:       "(agent/node) Kernel tuning behavior. If set, error if kernel tunables are different than kubelet defaults.",
		Destination: &AgentConfig.ProtectKernelDefaults,
	}
	SELinuxFlag = &cli.BoolFlag{
		Name:        "selinux",
		Usage:       "(agent/node) Enable SELinux in containerd",
		Destination: &AgentConfig.EnableSELinux,
		EnvVar:      version.ProgramUpper + "_SELINUX",
	}
	LBServerPortFlag = &cli.IntFlag{
		Name:        "lb-server-port",
		Usage:       "(agent/node) Local port for supervisor client load-balancer. If the supervisor and apiserver are not colocated an additional port 1 less than this port will also be used for the apiserver client load-balancer.",
		Destination: &AgentConfig.LBServerPort,
		EnvVar:      version.ProgramUpper + "_LB_SERVER_PORT",
		Value:       6444,
	}
	DockerFlag = &cli.BoolFlag{
		Name:        "docker",
		Usage:       "(agent/runtime) (experimental) Use cri-dockerd instead of containerd",
		Destination: &AgentConfig.Docker,
	}
	CRIEndpointFlag = &cli.StringFlag{
		Name:        "container-runtime-endpoint",
		Usage:       "(agent/runtime) Disable embedded containerd and use the CRI socket at the given path; when used with --docker this sets the docker socket path",
		Destination: &AgentConfig.ContainerRuntimeEndpoint,
	}
	DefaultRuntimeFlag = &cli.StringFlag{
		Name:        "default-runtime",
		Usage:       "(agent/runtime) Set the default runtime in containerd",
		Destination: &AgentConfig.DefaultRuntime,
	}
	ImageServiceEndpointFlag = &cli.StringFlag{
		Name:        "image-service-endpoint",
		Usage:       "(agent/runtime) Disable embedded containerd image service and use remote image service socket at the given path. If not specified, defaults to --container-runtime-endpoint.",
		Destination: &AgentConfig.ImageServiceEndpoint,
	}
	PrivateRegistryFlag = &cli.StringFlag{
		Name:        "private-registry",
		Usage:       "(agent/runtime) Private registry configuration file",
		Destination: &AgentConfig.PrivateRegistry,
		Value:       "/etc/rancher/" + version.Program + "/registries.yaml",
	}
	AirgapExtraRegistryFlag = &cli.StringSliceFlag{
		Name:   "airgap-extra-registry",
		Usage:  "(agent/runtime) Additional registry to tag airgap images as being sourced from",
		Value:  &AgentConfig.AirgapExtraRegistry,
		Hidden: true,
	}
	PauseImageFlag = &cli.StringFlag{
		Name:        "pause-image",
		Usage:       "(agent/runtime) Customized pause image for containerd or docker sandbox",
		Destination: &AgentConfig.PauseImage,
		Value:       DefaultPauseImage,
	}
	SnapshotterFlag = &cli.StringFlag{
		Name:        "snapshotter",
		Usage:       "(agent/runtime) Override default containerd snapshotter",
		Destination: &AgentConfig.Snapshotter,
		Value:       DefaultSnapshotter,
	}
	FlannelIfaceFlag = &cli.StringFlag{
		Name:        "flannel-iface",
		Usage:       "(agent/networking) Override default flannel interface",
		Destination: &AgentConfig.FlannelIface,
	}
	FlannelConfFlag = &cli.StringFlag{
		Name:        "flannel-conf",
		Usage:       "(agent/networking) Override default flannel config file",
		Destination: &AgentConfig.FlannelConf,
	}
	FlannelCniConfFileFlag = &cli.StringFlag{
		Name:        "flannel-cni-conf",
		Usage:       "(agent/networking) Override default flannel cni config file",
		Destination: &AgentConfig.FlannelCniConfFile,
	}
	VPNAuth = &cli.StringFlag{
		Name:        "vpn-auth",
		Usage:       "(agent/networking) (experimental) Credentials for the VPN provider. It must include the provider name and join key in the format name=<vpn-provider>,joinKey=<key>[,controlServerURL=<url>][,extraArgs=<args>]",
		EnvVar:      version.ProgramUpper + "_VPN_AUTH",
		Destination: &AgentConfig.VPNAuth,
	}
	VPNAuthFile = &cli.StringFlag{
		Name:        "vpn-auth-file",
		Usage:       "(agent/networking) (experimental) File containing credentials for the VPN provider. It must include the provider name and join key in the format name=<vpn-provider>,joinKey=<key>[,controlServerURL=<url>][,extraArgs=<args>]",
		EnvVar:      version.ProgramUpper + "_VPN_AUTH_FILE",
		Destination: &AgentConfig.VPNAuthFile,
	}
	ResolvConfFlag = &cli.StringFlag{
		Name:        "resolv-conf",
		Usage:       "(agent/networking) Kubelet resolv.conf file",
		EnvVar:      version.ProgramUpper + "_RESOLV_CONF",
		Destination: &AgentConfig.ResolvConf,
	}
	ExtraKubeletArgs = &cli.StringSliceFlag{
		Name:  "kubelet-arg",
		Usage: "(agent/flags) Customized flag for kubelet process",
		Value: &AgentConfig.ExtraKubeletArgs,
	}
	ExtraKubeProxyArgs = &cli.StringSliceFlag{
		Name:  "kube-proxy-arg",
		Usage: "(agent/flags) Customized flag for kube-proxy process",
		Value: &AgentConfig.ExtraKubeProxyArgs,
	}
	NodeTaints = &cli.StringSliceFlag{
		Name:  "node-taint",
		Usage: "(agent/node) Registering kubelet with set of taints",
		Value: &AgentConfig.Taints,
	}
	NodeLabels = &cli.StringSliceFlag{
		Name:  "node-label",
		Usage: "(agent/node) Registering and starting kubelet with set of labels",
		Value: &AgentConfig.Labels,
	}
	ImageCredProvBinDirFlag = &cli.StringFlag{
		Name:        "image-credential-provider-bin-dir",
		Usage:       "(agent/node) The path to the directory where credential provider plugin binaries are located",
		Destination: &AgentConfig.ImageCredProvBinDir,
		Value:       "/var/lib/rancher/credentialprovider/bin",
	}
	ImageCredProvConfigFlag = &cli.StringFlag{
		Name:        "image-credential-provider-config",
		Usage:       "(agent/node) The path to the credential provider plugin config file",
		Destination: &AgentConfig.ImageCredProvConfig,
		Value:       "/var/lib/rancher/credentialprovider/config.yaml",
	}
	DisableAgentLBFlag = &cli.BoolFlag{
		Name:        "disable-apiserver-lb",
		Usage:       "(agent/networking) (experimental) Disable the agent's client-side load-balancer and connect directly to the configured server address",
		Destination: &AgentConfig.DisableLoadBalancer,
	}
	DisableDefaultRegistryEndpointFlag = &cli.BoolFlag{
		Name:        "disable-default-registry-endpoint",
		Usage:       "(agent/containerd) Disables containerd's fallback default registry endpoint when a mirror is configured for that registry",
		Destination: &AgentConfig.ContainerdNoDefault,
	}
	EnablePProfFlag = &cli.BoolFlag{
		Name:        "enable-pprof",
		Usage:       "(experimental) Enable pprof endpoint on supervisor port",
		Destination: &AgentConfig.EnablePProf,
	}
	BindAddressFlag = &cli.StringFlag{
		Name:        "bind-address",
		Usage:       "(listener) " + version.Program + " bind address (default: 0.0.0.0)",
		Destination: &AgentConfig.BindAddress,
	}
)

func NewAgentCommand(action func(ctx *cli.Context) error) cli.Command {
	return cli.Command{
		Name:      "agent",
		Usage:     "Run node agent",
		UsageText: appName + " agent [OPTIONS]",
		Action:    action,
		Flags: []cli.Flag{
			ConfigFlag,
			DebugFlag,
			VLevel,
			VModule,
			LogFile,
			AlsoLogToStderr,
			AgentTokenFlag,
			&cli.StringFlag{
				Name:        "token-file",
				Usage:       "(cluster) Token file to use for authentication",
				EnvVar:      version.ProgramUpper + "_TOKEN_FILE",
				Destination: &AgentConfig.TokenFile,
			},
			&cli.StringFlag{
				Name:        "server,s",
				Usage:       "(cluster) Server to connect to",
				EnvVar:      version.ProgramUpper + "_URL",
				Destination: &AgentConfig.ServerURL,
			},
			// Note that this is different from DataDirFlag used elswhere in the CLI,
			// as this is bound to AgentConfig instead of ServerConfig.
			&cli.StringFlag{
				Name:        "data-dir,d",
				Usage:       "(agent/data) Folder to hold state",
				Destination: &AgentConfig.DataDir,
				Value:       "/var/lib/rancher/" + version.Program + "",
				EnvVar:      version.ProgramUpper + "_DATA_DIR",
			},
			NodeNameFlag,
			WithNodeIDFlag,
			NodeLabels,
			NodeTaints,
			ImageCredProvBinDirFlag,
			ImageCredProvConfigFlag,
			SELinuxFlag,
			LBServerPortFlag,
			ProtectKernelDefaultsFlag,
			CRIEndpointFlag,
			DefaultRuntimeFlag,
			ImageServiceEndpointFlag,
			PauseImageFlag,
			SnapshotterFlag,
			PrivateRegistryFlag,
			DisableDefaultRegistryEndpointFlag,
			AirgapExtraRegistryFlag,
			NodeIPFlag,
			BindAddressFlag,
			NodeExternalIPFlag,
			NodeInternalDNSFlag,
			NodeExternalDNSFlag,
			ResolvConfFlag,
			FlannelIfaceFlag,
			FlannelConfFlag,
			FlannelCniConfFileFlag,
			ExtraKubeletArgs,
			ExtraKubeProxyArgs,
			// Experimental flags
			EnablePProfFlag,
			&cli.BoolFlag{
				Name:        "rootless",
				Usage:       "(experimental) Run rootless",
				Destination: &AgentConfig.Rootless,
			},
			PreferBundledBin,
			// Deprecated/hidden below
			DockerFlag,
			VPNAuth,
			VPNAuthFile,
			DisableAgentLBFlag,
		},
	}
}
