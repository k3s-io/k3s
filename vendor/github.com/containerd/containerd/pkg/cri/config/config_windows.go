// +build windows

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
	"os"
	"path/filepath"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/pkg/cri/streaming"
)

// DefaultConfig returns default configurations of cri plugin.
func DefaultConfig() PluginConfig {
	return PluginConfig{
		CniConfig: CniConfig{
			NetworkPluginBinDir:       filepath.Join(os.Getenv("ProgramFiles"), "containerd", "cni", "bin"),
			NetworkPluginConfDir:      filepath.Join(os.Getenv("ProgramFiles"), "containerd", "cni", "conf"),
			NetworkPluginMaxConfNum:   1,
			NetworkPluginConfTemplate: "",
		},
		ContainerdConfig: ContainerdConfig{
			Snapshotter:        containerd.DefaultSnapshotter,
			DefaultRuntimeName: "runhcs-wcow-process",
			NoPivot:            false,
			Runtimes: map[string]Runtime{
				"runhcs-wcow-process": {
					Type: "io.containerd.runhcs.v1",
				},
			},
		},
		DisableTCPService:   true,
		StreamServerAddress: "127.0.0.1",
		StreamServerPort:    "0",
		StreamIdleTimeout:   streaming.DefaultConfig.StreamIdleTimeout.String(), // 4 hour
		EnableTLSStreaming:  false,
		X509KeyPairStreaming: X509KeyPairStreaming{
			TLSKeyFile:  "",
			TLSCertFile: "",
		},
		SandboxImage:              "k8s.gcr.io/pause:3.5",
		StatsCollectPeriod:        10,
		MaxContainerLogLineSize:   16 * 1024,
		MaxConcurrentDownloads:    3,
		IgnoreImageDefinedVolumes: false,
		// TODO(windows): Add platform specific config, so that most common defaults can be shared.

		ImageDecryption: ImageDecryption{
			KeyModel: KeyModelNode,
		},
	}
}
