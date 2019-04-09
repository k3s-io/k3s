/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"github.com/BurntSushi/toml"
	"github.com/containerd/containerd"
)

// Runtime struct to contain the type(ID), engine, and root variables for a default runtime
// and a runtime for untrusted worload.
type Runtime struct {
	// Type is the runtime type to use in containerd e.g. io.containerd.runtime.v1.linux
	Type string `toml:"runtime_type" json:"runtimeType"`
	// Engine is the name of the runtime engine used by containerd.
	// This only works for runtime type "io.containerd.runtime.v1.linux".
	// DEPRECATED: use Options instead. Remove when shim v1 is deprecated.
	Engine string `toml:"runtime_engine" json:"runtimeEngine"`
	// Root is the directory used by containerd for runtime state.
	// DEPRECATED: use Options instead. Remove when shim v1 is deprecated.
	// This only works for runtime type "io.containerd.runtime.v1.linux".
	Root string `toml:"runtime_root" json:"runtimeRoot"`
	// Options are config options for the runtime. If options is loaded
	// from toml config, it will be toml.Primitive.
	Options *toml.Primitive `toml:"options" json:"options"`
}

// ContainerdConfig contains toml config related to containerd
type ContainerdConfig struct {
	// Snapshotter is the snapshotter used by containerd.
	Snapshotter string `toml:"snapshotter" json:"snapshotter"`
	// DefaultRuntime is the default runtime to use in containerd.
	// This runtime is used when no runtime handler (or the empty string) is provided.
	DefaultRuntime Runtime `toml:"default_runtime" json:"defaultRuntime"`
	// UntrustedWorkloadRuntime is a runtime to run untrusted workloads on it.
	// DEPRECATED: use Runtimes instead. If provided, this runtime is mapped to the runtime handler
	//     named 'untrusted'. It is a configuration error to provide both the (now deprecated)
	//     UntrustedWorkloadRuntime and a handler in the Runtimes handler map (below) for 'untrusted'
	//     workloads at the same time. Please provide one or the other.
	UntrustedWorkloadRuntime Runtime `toml:"untrusted_workload_runtime" json:"untrustedWorkloadRuntime"`
	// Runtimes is a map from CRI RuntimeHandler strings, which specify types of runtime
	// configurations, to the matching configurations.
	Runtimes map[string]Runtime `toml:"runtimes" json:"runtimes"`
	// NoPivot disables pivot-root (linux only), required when running a container in a RamDisk with runc
	// This only works for runtime type "io.containerd.runtime.v1.linux".
	// DEPRECATED: use Runtime.Options instead. Remove when shim v1 is deprecated.
	NoPivot bool `toml:"no_pivot" json:"noPivot"`
}

// CniConfig contains toml config related to cni
type CniConfig struct {
	// NetworkPluginBinDir is the directory in which the binaries for the plugin is kept.
	NetworkPluginBinDir string `toml:"bin_dir" json:"binDir"`
	// NetworkPluginConfDir is the directory in which the admin places a CNI conf.
	NetworkPluginConfDir string `toml:"conf_dir" json:"confDir"`
	// NetworkPluginConfTemplate is the file path of golang template used to generate
	// cni config.
	// When it is set, containerd will get cidr from kubelet to replace {{.PodCIDR}} in
	// the template, and write the config into NetworkPluginConfDir.
	// Ideally the cni config should be placed by system admin or cni daemon like calico,
	// weaveworks etc. However, there are still users using kubenet
	// (https://kubernetes.io/docs/concepts/cluster-administration/network-plugins/#kubenet)
	// today, who don't have a cni daemonset in production. NetworkPluginConfTemplate is
	// a temporary backward-compatible solution for them.
	// TODO(random-liu): Deprecate this option when kubenet is deprecated.
	NetworkPluginConfTemplate string `toml:"conf_template" json:"confTemplate"`
}

// Mirror contains the config related to the registry mirror
type Mirror struct {
	// Endpoints are endpoints for a namespace. CRI plugin will try the endpoints
	// one by one until a working one is found. The endpoint must be a valid url
	// with host specified.
	Endpoints []string `toml:"endpoint" json:"endpoint"`
}

// AuthConfig contains the config related to authentication to a specific registry
type AuthConfig struct {
	// Username is the username to login the registry.
	Username string `toml:"username" json:"username"`
	// Password is the password to login the registry.
	Password string `toml:"password" json:"password"`
	// Auth is a base64 encoded string from the concatenation of the username,
	// a colon, and the password.
	Auth string `toml:"auth" json:"auth"`
	// IdentityToken is used to authenticate the user and get
	// an access token for the registry.
	IdentityToken string `toml:"identitytoken" json:"identitytoken"`
}

// Registry is registry settings configured
type Registry struct {
	// Mirrors are namespace to mirror mapping for all namespaces.
	Mirrors map[string]Mirror `toml:"mirrors" json:"mirrors"`
	// Auths are registry endpoint to auth config mapping. The registry endpoint must
	// be a valid url with host specified.
	Auths map[string]AuthConfig `toml:"auths" json:"auths"`
}

// PluginConfig contains toml config related to CRI plugin,
// it is a subset of Config.
type PluginConfig struct {
	// ContainerdConfig contains config related to containerd
	ContainerdConfig `toml:"containerd" json:"containerd"`
	// CniConfig contains config related to cni
	CniConfig `toml:"cni" json:"cni"`
	// Registry contains config related to the registry
	Registry Registry `toml:"registry" json:"registry"`
	// StreamServerAddress is the ip address streaming server is listening on.
	StreamServerAddress string `toml:"stream_server_address" json:"streamServerAddress"`
	// StreamServerPort is the port streaming server is listening on.
	StreamServerPort string `toml:"stream_server_port" json:"streamServerPort"`
	// EnableSelinux indicates to enable the selinux support.
	EnableSelinux bool `toml:"enable_selinux" json:"enableSelinux"`
	// SandboxImage is the image used by sandbox container.
	SandboxImage string `toml:"sandbox_image" json:"sandboxImage"`
	// StatsCollectPeriod is the period (in seconds) of snapshots stats collection.
	StatsCollectPeriod int `toml:"stats_collect_period" json:"statsCollectPeriod"`
	// SystemdCgroup enables systemd cgroup support.
	// This only works for runtime type "io.containerd.runtime.v1.linux".
	// DEPRECATED: config runc runtime handler instead. Remove when shim v1 is deprecated.
	SystemdCgroup bool `toml:"systemd_cgroup" json:"systemdCgroup"`
	// EnableTLSStreaming indicates to enable the TLS streaming support.
	EnableTLSStreaming bool `toml:"enable_tls_streaming" json:"enableTLSStreaming"`
	// X509KeyPairStreaming is a x509 key pair used for TLS streaming
	X509KeyPairStreaming `toml:"x509_key_pair_streaming" json:"x509KeyPairStreaming"`
	// MaxContainerLogLineSize is the maximum log line size in bytes for a container.
	// Log line longer than the limit will be split into multiple lines. Non-positive
	// value means no limit.
	MaxContainerLogLineSize int `toml:"max_container_log_line_size" json:"maxContainerLogSize"`
	// DisableCgroup indicates to disable the cgroup support.
	// This is useful when the containerd does not have permission to access cgroup.
	DisableCgroup bool `toml:"disable_cgroup" json:"disableCgroup"`
	// DisableApparmor indicates to disable the apparmor support.
	// This is useful when the containerd does not have permission to access Apparmor.
	DisableApparmor bool `toml:"disable_apparmor" json:"disableApparmor"`
	// RestrictOOMScoreAdj indicates to limit the lower bound of OOMScoreAdj to the containerd's
	// current OOMScoreADj.
	// This is useful when the containerd does not have permission to decrease OOMScoreAdj.
	RestrictOOMScoreAdj bool `toml:"restrict_oom_score_adj" json:"restrictOOMScoreAdj"`
}

// X509KeyPairStreaming contains the x509 configuration for streaming
type X509KeyPairStreaming struct {
	// TLSCertFile is the path to a certificate file
	TLSCertFile string `toml:"tls_cert_file" json:"tlsCertFile"`
	// TLSKeyFile is the path to a private key file
	TLSKeyFile string `toml:"tls_key_file" json:"tlsKeyFile"`
}

// Config contains all configurations for cri server.
type Config struct {
	// PluginConfig is the config for CRI plugin.
	PluginConfig
	// ContainerdRootDir is the root directory path for containerd.
	ContainerdRootDir string `json:"containerdRootDir"`
	// ContainerdEndpoint is the containerd endpoint path.
	ContainerdEndpoint string `json:"containerdEndpoint"`
	// RootDir is the root directory path for managing cri plugin files
	// (metadata checkpoint etc.)
	RootDir string `json:"rootDir"`
	// StateDir is the root directory path for managing volatile pod/container data
	StateDir string `json:"stateDir"`
}

// DefaultConfig returns default configurations of cri plugin.
func DefaultConfig() PluginConfig {
	return PluginConfig{
		CniConfig: CniConfig{
			NetworkPluginBinDir:       "/opt/cni/bin",
			NetworkPluginConfDir:      "/etc/cni/net.d",
			NetworkPluginConfTemplate: "",
		},
		ContainerdConfig: ContainerdConfig{
			Snapshotter: containerd.DefaultSnapshotter,
			DefaultRuntime: Runtime{
				Type:   "io.containerd.runtime.v1.linux",
				Engine: "",
				Root:   "",
			},
			NoPivot: false,
		},
		StreamServerAddress: "127.0.0.1",
		StreamServerPort:    "0",
		EnableSelinux:       false,
		EnableTLSStreaming:  false,
		X509KeyPairStreaming: X509KeyPairStreaming{
			TLSKeyFile:  "",
			TLSCertFile: "",
		},
		SandboxImage:            "k8s.gcr.io/pause:3.1",
		StatsCollectPeriod:      10,
		SystemdCgroup:           false,
		MaxContainerLogLineSize: 16 * 1024,
		Registry: Registry{
			Mirrors: map[string]Mirror{
				"docker.io": {
					Endpoints: []string{"https://registry-1.docker.io"},
				},
			},
		},
	}
}

const (
	// RuntimeUntrusted is the implicit runtime defined for ContainerdConfig.UntrustedWorkloadRuntime
	RuntimeUntrusted = "untrusted"
)
