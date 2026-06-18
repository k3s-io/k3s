//go:build linux && !no_cri_dockerd

package cridockerd

import (
	"net"
	"testing"

	"github.com/k3s-io/k3s/pkg/daemons/config"
)

// argContains returns true if the given args slice contains an element equal to arg.
func argContains(args []string, arg string) bool {
	for _, a := range args {
		if a == arg {
			return true
		}
	}
	return false
}

func Test_UnitGetDockerCRIArgs(t *testing.T) {
	tests := []struct {
		name        string
		cfg         func() *config.Node
		setup       func(t *testing.T)
		mustContain []string
		mustNotHave []string
	}{
		{
			name: "default args are always present",
			cfg: func() *config.Node {
				return &config.Node{
					CRIDockerd: config.CRIDockerd{
						Address: "/run/k3s/cri-dockerd/cri-dockerd.sock",
						Root:    "/var/lib/rancher/k3s/data/current/bin",
					},
				}
			},
			mustContain: []string{
				"--container-runtime-endpoint=/run/k3s/cri-dockerd/cri-dockerd.sock",
				"--cri-dockerd-root-directory=/var/lib/rancher/k3s/data/current/bin",
				"--streaming-bind-addr=127.0.0.1:10010",
			},
		},
		{
			name: "debug mode sets log-level to debug",
			cfg: func() *config.Node {
				return &config.Node{
					CRIDockerd: config.CRIDockerd{
						Debug: true,
					},
				}
			},
			mustContain: []string{"--log-level=debug"},
		},
		{
			name: "CRIDOCKERD_LOG_LEVEL env var overrides log level",
			cfg: func() *config.Node {
				return &config.Node{}
			},
			setup: func(t *testing.T) {
				t.Setenv("CRIDOCKERD_LOG_LEVEL", "trace")
			},
			mustContain: []string{"--log-level=trace"},
		},
		{
			name: "CRIDOCKERD_LOG_LEVEL overrides debug flag",
			cfg: func() *config.Node {
				return &config.Node{
					CRIDockerd: config.CRIDockerd{Debug: true},
				}
			},
			setup: func(t *testing.T) {
				t.Setenv("CRIDOCKERD_LOG_LEVEL", "warn")
			},
			mustContain: []string{"--log-level=warn"},
		},
		{
			name: "docker endpoint without prefix gets unix:// prepended",
			cfg: func() *config.Node {
				return &config.Node{
					ContainerRuntimeEndpoint: "/var/run/docker.sock",
				}
			},
			mustContain: []string{"--docker-endpoint=unix:///var/run/docker.sock"},
		},
		{
			name: "docker endpoint already with unix:// prefix is unchanged",
			cfg: func() *config.Node {
				return &config.Node{
					ContainerRuntimeEndpoint: "unix:///var/run/docker.sock",
				}
			},
			mustContain: []string{"--docker-endpoint=unix:///var/run/docker.sock"},
		},
		{
			name: "empty ContainerRuntimeEndpoint omits docker-endpoint arg",
			cfg: func() *config.Node {
				return &config.Node{}
			},
			mustNotHave: []string{"--docker-endpoint"},
		},
		{
			name: "CNI config dirs are passed when set",
			cfg: func() *config.Node {
				return &config.Node{
					AgentConfig: config.Agent{
						CNIConfDir: "/etc/cni/net.d",
						CNIBinDir:  "/opt/cni/bin",
					},
				}
			},
			mustContain: []string{
				"--cni-conf-dir=/etc/cni/net.d",
				"--cni-bin-dir=/opt/cni/bin",
			},
		},
		{
			name: "CNI plugin flag sets network-plugin=cni",
			cfg: func() *config.Node {
				return &config.Node{
					AgentConfig: config.Agent{
						CNIPlugin: true,
					},
				}
			},
			mustContain: []string{"--network-plugin=cni"},
		},
		{
			name: "network-plugin is absent when CNIPlugin is false",
			cfg: func() *config.Node {
				return &config.Node{
					AgentConfig: config.Agent{
						CNIPlugin: false,
					},
				}
			},
			mustNotHave: []string{"--network-plugin=cni"},
		},
		{
			name: "pause image is passed when set",
			cfg: func() *config.Node {
				return &config.Node{
					AgentConfig: config.Agent{
						PauseImage: "rancher/pause:3.1",
					},
				}
			},
			mustContain: []string{"--pod-infra-container-image=rancher/pause:3.1"},
		},
		{
			name: "dual-stack node IPs enable ipv6-dual-stack",
			cfg: func() *config.Node {
				return &config.Node{
					AgentConfig: config.Agent{
						NodeIPs: []net.IP{
							net.ParseIP("192.168.1.1"),
							net.ParseIP("fd00::1"),
						},
					},
				}
			},
			mustContain: []string{"--ipv6-dual-stack=true"},
		},
		{
			name: "single-stack node IPs do not enable ipv6-dual-stack",
			cfg: func() *config.Node {
				return &config.Node{
					AgentConfig: config.Agent{
						NodeIPs: []net.IP{net.ParseIP("192.168.1.1")},
					},
				}
			},
			mustNotHave: []string{"--ipv6-dual-stack=true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup(t)
			}

			args := getDockerCRIArgs(tt.cfg())

			for _, expected := range tt.mustContain {
				if !argContains(args, expected) {
					t.Errorf("expected arg %q to be present, got: %v", expected, args)
				}
			}

			for _, unexpected := range tt.mustNotHave {
				for _, arg := range args {
					if arg == unexpected {
						t.Errorf("expected arg %q to be absent, but found it in: %v", unexpected, args)
					}
				}
			}
		})
	}
}
