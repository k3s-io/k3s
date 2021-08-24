// +build !windows

/*
   Copyright The containerd Authors.

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
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/pkg/cri/streaming"
	"github.com/pelletier/go-toml"
)

// DefaultConfig returns default configurations of cri plugin.
func DefaultConfig() PluginConfig {
	defaultRuncV2Opts := `
	# NoPivotRoot disables pivot root when creating a container.
	NoPivotRoot = false

	# NoNewKeyring disables new keyring for the container.
	NoNewKeyring = false

	# ShimCgroup places the shim in a cgroup.
	ShimCgroup = ""

	# IoUid sets the I/O's pipes uid.
	IoUid = 0

	# IoGid sets the I/O's pipes gid.
	IoGid = 0

	# BinaryName is the binary name of the runc binary.
	BinaryName = ""

	# Root is the runc root directory.
	Root = ""

	# CriuPath is the criu binary path.
	CriuPath = ""

	# SystemdCgroup enables systemd cgroups.
	SystemdCgroup = false

	# CriuImagePath is the criu image path
	CriuImagePath = ""

	# CriuWorkPath is the criu work path.
	CriuWorkPath = ""
`
	tree, _ := toml.Load(defaultRuncV2Opts)
	return PluginConfig{
		CniConfig: CniConfig{
			NetworkPluginBinDir:       "/opt/cni/bin",
			NetworkPluginConfDir:      "/etc/cni/net.d",
			NetworkPluginMaxConfNum:   1, // only one CNI plugin config file will be loaded
			NetworkPluginConfTemplate: "",
		},
		ContainerdConfig: ContainerdConfig{
			Snapshotter:        containerd.DefaultSnapshotter,
			DefaultRuntimeName: "runc",
			NoPivot:            false,
			Runtimes: map[string]Runtime{
				"runc": {
					Type:    "io.containerd.runc.v2",
					Options: tree.ToMap(),
				},
			},
			DisableSnapshotAnnotations: true,
		},
		DisableTCPService:    true,
		StreamServerAddress:  "127.0.0.1",
		StreamServerPort:     "0",
		StreamIdleTimeout:    streaming.DefaultConfig.StreamIdleTimeout.String(), // 4 hour
		EnableSelinux:        false,
		SelinuxCategoryRange: 1024,
		EnableTLSStreaming:   false,
		X509KeyPairStreaming: X509KeyPairStreaming{
			TLSKeyFile:  "",
			TLSCertFile: "",
		},
		SandboxImage:                     "k8s.gcr.io/pause:3.5",
		StatsCollectPeriod:               10,
		SystemdCgroup:                    false,
		MaxContainerLogLineSize:          16 * 1024,
		MaxConcurrentDownloads:           3,
		DisableProcMount:                 false,
		TolerateMissingHugetlbController: true,
		DisableHugetlbController:         true,
		IgnoreImageDefinedVolumes:        false,
		ImageDecryption: ImageDecryption{
			KeyModel: KeyModelNode,
		},
	}
}
