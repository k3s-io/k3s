package containerd

import (
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/pkg/agent/templates"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/spegel"
	"github.com/rancher/wharfie/pkg/registries"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

func u(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func Test_UnitGetHostConfigs(t *testing.T) {
	type args struct {
		registryContent   string
		noDefaultEndpoint bool
		mirrorAddr        string
	}
	tests := []struct {
		name string
		args args
		want HostConfigs
	}{
		{
			name: "no registries",
			want: HostConfigs{},
		},
		{
			name: "registry with default endpoint",
			args: args{
				registryContent: `
				  mirrors:
						docker.io:
				`,
			},
			want: HostConfigs{},
		},
		{
			name: "registry with default endpoint explicitly listed",
			args: args{
				registryContent: `
				  mirrors:
						docker.io:
							endpoint:
								- docker.io
				`,
			},
			want: HostConfigs{},
		},
		{
			name: "registry with default endpoint - embedded registry",
			args: args{
				mirrorAddr: "127.0.0.1:6443",
				registryContent: `
				  mirrors:
						docker.io:
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("https://127.0.0.1:6443/v2"),
							Config: registries.RegistryConfig{
								TLS: &registries.TLSConfig{
									CAFile:   "server-ca",
									KeyFile:  "client-key",
									CertFile: "client-cert",
								},
							},
						},
					},
				},
				"127.0.0.1:6443": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://127.0.0.1:6443/v2"),
						Config: registries.RegistryConfig{
							TLS: &registries.TLSConfig{
								CAFile:   "server-ca",
								KeyFile:  "client-key",
								CertFile: "client-cert",
							},
						},
					},
				},
			},
		},
		{
			name: "registry with default endpoint and creds",
			args: args{
				registryContent: `
				  mirrors:
						docker.io:
					configs:
					  docker.io:
						  auth:
							  username: user
								password: pass
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
						Config: registries.RegistryConfig{
							Auth: &registries.AuthConfig{
								Username: "user",
								Password: "pass",
							},
						},
					},
				},
			},
		},
		{
			name: "registry with default endpoint explicitly listed and creds",
			args: args{
				registryContent: `
				  mirrors:
						docker.io:
						  endpoint:
							  - docker.io
					configs:
					  docker.io:
						  auth:
							  username: user
								password: pass
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
						Config: registries.RegistryConfig{
							Auth: &registries.AuthConfig{
								Username: "user",
								Password: "pass",
							},
						},
					},
				},
			},
		},

		{
			name: "registry with only creds",
			args: args{
				registryContent: `
					configs:
					  docker.io:
						  auth:
							  username: user
								password: pass
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
						Config: registries.RegistryConfig{
							Auth: &registries.AuthConfig{
								Username: "user",
								Password: "pass",
							},
						},
					},
				},
			},
		},
		{
			name: "private registry with default endpoint",
			args: args{
				registryContent: `
				  mirrors:
						registry.example.com:
				`,
			},
			want: HostConfigs{},
		},
		{
			name: "private registry with default endpoint and creds",
			args: args{
				registryContent: `
				  mirrors:
						registry.example.com:
					configs:
					  registry.example.com:
						  auth:
							  username: user
								password: pass
				`,
			},
			want: HostConfigs{
				"registry.example.com": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry.example.com/v2"),
						Config: registries.RegistryConfig{
							Auth: &registries.AuthConfig{
								Username: "user",
								Password: "pass",
							},
						},
					},
				},
			},
		},
		{
			name: "private registry with only creds",
			args: args{
				registryContent: `
					configs:
					  registry.example.com:
						  auth:
							  username: user
								password: pass
				`,
			},
			want: HostConfigs{
				"registry.example.com": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry.example.com/v2"),
						Config: registries.RegistryConfig{
							Auth: &registries.AuthConfig{
								Username: "user",
								Password: "pass",
							},
						},
					},
				},
			},
		},
		{
			name: "registry with mirror endpoint - full URL with override path",
			args: args{
				registryContent: `
				  mirrors:
						docker.io:
							endpoint:
							  - https://registry.example.com/prefix/v2
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							OverridePath: true,
							URL:          u("https://registry.example.com/prefix/v2"),
						},
					},
				},
			},
		},
		{
			name: "registry with mirror endpoint - hostname only with override path",
			args: args{
				registryContent: `
				  mirrors:
						docker.io:
							endpoint:
							  - registry.example.com/prefix/v2
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							OverridePath: true,
							URL:          u("https://registry.example.com/prefix/v2"),
						},
					},
				},
			},
		},
		{
			name: "registry with mirror endpoint - hostname only with default path",
			args: args{
				registryContent: `
				  mirrors:
						docker.io:
							endpoint:
							  - registry.example.com/v2
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("https://registry.example.com/v2"),
						},
					},
				},
			},
		},
		{
			name: "registry with mirror endpoint - full URL",
			args: args{
				registryContent: `
				  mirrors:
						docker.io:
							endpoint:
							  - https://registry.example.com/v2
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("https://registry.example.com/v2"),
						},
					},
				},
			},
		},
		{
			name: "registry with mirror endpoint - URL without path",
			args: args{
				registryContent: `
				  mirrors:
						docker.io:
							endpoint:
							  - https://registry.example.com
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("https://registry.example.com/v2"),
						},
					},
				},
			},
		},
		{
			name: "registry with mirror endpoint - hostname only",
			args: args{
				registryContent: `
				  mirrors:
						docker.io:
							endpoint:
							  - registry.example.com
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("https://registry.example.com/v2"),
						},
					},
				},
			},
		},
		{
			name: "registry with mirror endpoint - hostname and port only",
			args: args{
				registryContent: `
				  mirrors:
						docker.io:
							endpoint:
								- registry.example.com:443
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("https://registry.example.com:443/v2"),
						},
					},
				},
			},
		},
		{
			name: "registry with mirror endpoint - ip address only",
			args: args{
				registryContent: `
				  mirrors:
						docker.io:
							endpoint:
								- 1.2.3.4
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("https://1.2.3.4/v2"),
						},
					},
				},
			},
		},
		{
			name: "registry with mirror endpoint - ip and port only",
			args: args{
				registryContent: `
				  mirrors:
						docker.io:
							endpoint:
								- 1.2.3.4:443
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("https://1.2.3.4:443/v2"),
						},
					},
				},
			},
		},
		{
			name: "registry with mirror endpoint - duplicate endpoints",
			args: args{
				registryContent: `
				  mirrors:
						docker.io:
							endpoint:
								- registry.example.com
								- registry.example.com
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("https://registry.example.com/v2"),
						},
					},
				},
			},
		},
		{
			name: "registry with mirror endpoint - duplicate endpoints in different formats",
			args: args{
				registryContent: `
				  mirrors:
						docker.io:
							endpoint:
								- registry.example.com
								- https://registry.example.com
								- https://registry.example.com/v2
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("https://registry.example.com/v2"),
						},
					},
				},
			},
		},
		{
			name: "registry with mirror endpoint - duplicate endpoints in different positions",
			args: args{
				registryContent: `
				  mirrors:
						docker.io:
							endpoint:
								- https://registry.example.com
								- https://registry.example.org
								- https://registry.example.com
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("https://registry.example.com/v2"),
						},
						{
							URL: u("https://registry.example.org/v2"),
						},
					},
				},
			},
		},
		{
			name: "registry with mirror endpoint - localhost and port only",
			args: args{
				registryContent: `
				  mirrors:
						docker.io:
							endpoint:
								- localhost:5000
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("http://localhost:5000/v2"),
						},
					},
				},
			},
		},
		{
			name: "registry with mirror endpoint - localhost and port with scheme",
			args: args{
				registryContent: `
				  mirrors:
						docker.io:
							endpoint:
								- https://localhost:5000
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("https://localhost:5000/v2"),
						},
					},
				},
			},
		},
		{
			name: "registry with mirror endpoint - loopback ip and port only",
			args: args{
				registryContent: `
				  mirrors:
						docker.io:
							endpoint:
								- 127.0.0.1:5000
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("http://127.0.0.1:5000/v2"),
						},
					},
				},
			},
		},
		{
			name: "registry with mirror endpoint - loopback ip and port with scheme",
			args: args{
				registryContent: `
				  mirrors:
						docker.io:
							endpoint:
								- https://127.0.0.1:5000
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("https://127.0.0.1:5000/v2"),
						},
					},
				},
			},
		},
		{
			name: "registry with mirror endpoint and mirror creds",
			args: args{
				registryContent: `
				  mirrors:
						docker.io:
						  endpoint:
							  - https://registry.example.com/v2
					configs:
					  registry.example.com:
						  auth:
							  username: user
								password: pass
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("https://registry.example.com/v2"),
							Config: registries.RegistryConfig{
								Auth: &registries.AuthConfig{
									Username: "user",
									Password: "pass",
								},
							},
						},
					},
				},
				"registry.example.com": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry.example.com/v2"),
						Config: registries.RegistryConfig{
							Auth: &registries.AuthConfig{
								Username: "user",
								Password: "pass",
							},
						},
					},
				},
			},
		},
		{
			name: "registry with mirror endpoint and mirror creds - override path with v2",
			args: args{
				registryContent: `
				  mirrors:
						docker.io:
						  endpoint:
							  - https://registry.example.com/prefix/v2
						registry.example.com:
						  endpoint:
							  - https://registry.example.com/prefix/v2
					configs:
					  registry.example.com:
						  auth:
							  username: user
								password: pass
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							OverridePath: true,
							URL:          u("https://registry.example.com/prefix/v2"),
							Config: registries.RegistryConfig{
								Auth: &registries.AuthConfig{
									Username: "user",
									Password: "pass",
								},
							},
						},
					},
				},
				"registry.example.com": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry.example.com/v2"),
						Config: registries.RegistryConfig{
							Auth: &registries.AuthConfig{
								Username: "user",
								Password: "pass",
							},
						},
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							OverridePath: true,
							URL:          u("https://registry.example.com/prefix/v2"),
							Config: registries.RegistryConfig{
								Auth: &registries.AuthConfig{
									Username: "user",
									Password: "pass",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "registry with mirror endpoint and mirror creds - override path without v2",
			args: args{
				registryContent: `
				  mirrors:
						docker.io:
						  endpoint:
							  - https://registry.example.com/project/registry
						registry.example.com:
						  endpoint:
							  - https://registry.example.com/project/registry
					configs:
					  registry.example.com:
						  auth:
							  username: user
								password: pass
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							OverridePath: true,
							URL:          u("https://registry.example.com/project/registry"),
							Config: registries.RegistryConfig{
								Auth: &registries.AuthConfig{
									Username: "user",
									Password: "pass",
								},
							},
						},
					},
				},
				"registry.example.com": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry.example.com/v2"),
						Config: registries.RegistryConfig{
							Auth: &registries.AuthConfig{
								Username: "user",
								Password: "pass",
							},
						},
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							OverridePath: true,
							URL:          u("https://registry.example.com/project/registry"),
							Config: registries.RegistryConfig{
								Auth: &registries.AuthConfig{
									Username: "user",
									Password: "pass",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "registry with mirror endpoint and mirror creds - no default endpoint",
			args: args{
				noDefaultEndpoint: true,
				registryContent: `
				  mirrors:
						docker.io:
						  endpoint:
							  - https://registry.example.com/v2
					configs:
					  registry.example.com:
						  auth:
							  username: user
								password: pass
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("https://registry.example.com/v2"),
							Config: registries.RegistryConfig{
								Auth: &registries.AuthConfig{
									Username: "user",
									Password: "pass",
								},
							},
						},
					},
				},
				"registry.example.com": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry.example.com/v2"),
						Config: registries.RegistryConfig{
							Auth: &registries.AuthConfig{
								Username: "user",
								Password: "pass",
							},
						},
					},
				},
			},
		},
		{
			name: "registry with mirror endpoint and mirror creds - embedded registry",
			args: args{
				mirrorAddr: "127.0.0.1:6443",
				registryContent: `
				  mirrors:
						docker.io:
						  endpoint:
							  - https://registry.example.com/v2
					configs:
					  registry.example.com:
						  auth:
							  username: user
								password: pass
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("https://127.0.0.1:6443/v2"),
							Config: registries.RegistryConfig{
								TLS: &registries.TLSConfig{
									CAFile:   "server-ca",
									KeyFile:  "client-key",
									CertFile: "client-cert",
								},
							},
						},
						{
							URL: u("https://registry.example.com/v2"),
							Config: registries.RegistryConfig{
								Auth: &registries.AuthConfig{
									Username: "user",
									Password: "pass",
								},
							},
						},
					},
				},
				"registry.example.com": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry.example.com/v2"),
						Config: registries.RegistryConfig{
							Auth: &registries.AuthConfig{
								Username: "user",
								Password: "pass",
							},
						},
					},
				},
				"127.0.0.1:6443": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://127.0.0.1:6443/v2"),
						Config: registries.RegistryConfig{
							TLS: &registries.TLSConfig{
								CAFile:   "server-ca",
								KeyFile:  "client-key",
								CertFile: "client-cert",
							},
						},
					},
				},
			},
		},
		{
			name: "registry with mirror endpoint and mirror creds - embedded registry with rewrites",
			args: args{
				mirrorAddr: "127.0.0.1:6443",
				registryContent: `
				  mirrors:
						docker.io:
						  endpoint:
							  - https://registry.example.com/v2
							rewrite:
							  "^rancher/(.*)": "docker/rancher-images/$1"
					configs:
					  registry.example.com:
						  auth:
							  username: user
								password: pass
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("https://127.0.0.1:6443/v2"),
							Config: registries.RegistryConfig{
								TLS: &registries.TLSConfig{
									CAFile:   "server-ca",
									KeyFile:  "client-key",
									CertFile: "client-cert",
								},
							},
						},
						{
							URL: u("https://registry.example.com/v2"),
							Config: registries.RegistryConfig{
								Auth: &registries.AuthConfig{
									Username: "user",
									Password: "pass",
								},
							},
							Rewrites: map[string]string{
								"^rancher/(.*)": "docker/rancher-images/$1",
							},
						},
					},
				},
				"registry.example.com": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry.example.com/v2"),
						Config: registries.RegistryConfig{
							Auth: &registries.AuthConfig{
								Username: "user",
								Password: "pass",
							},
						},
					},
				},
				"127.0.0.1:6443": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://127.0.0.1:6443/v2"),
						Config: registries.RegistryConfig{
							TLS: &registries.TLSConfig{
								CAFile:   "server-ca",
								KeyFile:  "client-key",
								CertFile: "client-cert",
							},
						},
					},
				},
			},
		},
		{
			name: "registry with mirror endpoint and mirror creds - embedded registry and no default endpoint",
			args: args{
				mirrorAddr:        "127.0.0.1:6443",
				noDefaultEndpoint: true,
				registryContent: `
				  mirrors:
						docker.io:
						  endpoint:
							  - https://registry.example.com/v2
					configs:
					  registry.example.com:
						  auth:
							  username: user
								password: pass
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("https://127.0.0.1:6443/v2"),
							Config: registries.RegistryConfig{
								TLS: &registries.TLSConfig{
									CAFile:   "server-ca",
									KeyFile:  "client-key",
									CertFile: "client-cert",
								},
							},
						},
						{
							URL: u("https://registry.example.com/v2"),
							Config: registries.RegistryConfig{
								Auth: &registries.AuthConfig{
									Username: "user",
									Password: "pass",
								},
							},
						},
					},
				},
				"registry.example.com": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry.example.com/v2"),
						Config: registries.RegistryConfig{
							Auth: &registries.AuthConfig{
								Username: "user",
								Password: "pass",
							},
						},
					},
				},
				"127.0.0.1:6443": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://127.0.0.1:6443/v2"),
						Config: registries.RegistryConfig{
							TLS: &registries.TLSConfig{
								CAFile:   "server-ca",
								KeyFile:  "client-key",
								CertFile: "client-cert",
							},
						},
					},
				},
			},
		},
		{
			name: "registry with mirror endpoint - embedded registry, default endpoint explicitly listed",
			args: args{
				mirrorAddr: "127.0.0.1:6443",
				registryContent: `
				  mirrors:
						docker.io:
						  endpoint:
							  - registry.example.com
							  - registry.example.org
								- docker.io
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://registry-1.docker.io/v2"),
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("https://127.0.0.1:6443/v2"),
							Config: registries.RegistryConfig{
								TLS: &registries.TLSConfig{
									CAFile:   "server-ca",
									KeyFile:  "client-key",
									CertFile: "client-cert",
								},
							},
						},
						{
							URL: u("https://registry.example.com/v2"),
						},
						{
							URL: u("https://registry.example.org/v2"),
						},
					},
				},
				"127.0.0.1:6443": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://127.0.0.1:6443/v2"),
						Config: registries.RegistryConfig{
							TLS: &registries.TLSConfig{
								CAFile:   "server-ca",
								KeyFile:  "client-key",
								CertFile: "client-cert",
							},
						},
					},
				},
			},
		},
		{
			name: "registry with mirror endpoint - embedded registry and no default endpoint, default endpoint explicitly listed",
			args: args{
				mirrorAddr:        "127.0.0.1:6443",
				noDefaultEndpoint: true,
				registryContent: `
				  mirrors:
						docker.io:
						  endpoint:
							  - registry.example.com
								- registry.example.org
								- docker.io
				`,
			},
			want: HostConfigs{
				"docker.io": templates.HostConfig{
					Program: "k3s",
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("https://127.0.0.1:6443/v2"),
							Config: registries.RegistryConfig{
								TLS: &registries.TLSConfig{
									CAFile:   "server-ca",
									KeyFile:  "client-key",
									CertFile: "client-cert",
								},
							},
						},
						{
							URL: u("https://registry.example.com/v2"),
						},
						{
							URL: u("https://registry.example.org/v2"),
						},
						{
							URL: u("https://registry-1.docker.io/v2"),
						},
					},
				},
				"127.0.0.1:6443": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://127.0.0.1:6443/v2"),
						Config: registries.RegistryConfig{
							TLS: &registries.TLSConfig{
								CAFile:   "server-ca",
								KeyFile:  "client-key",
								CertFile: "client-cert",
							},
						},
					},
				},
			},
		},
		{
			name: "wildcard mirror endpoint - no endpoints",
			args: args{
				registryContent: `
				  mirrors:
						"*":
				`,
			},
			want: HostConfigs{},
		},
		{
			name: "wildcard mirror endpoint - full URL",
			args: args{
				registryContent: `
				  mirrors:
						"*":
							endpoint:
								- https://registry.example.com/v2
				`,
			},
			want: HostConfigs{
				"_default": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u(""),
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("https://registry.example.com/v2"),
						},
					},
				},
			},
		},
		{
			name: "wildcard mirror endpoint - full URL, embedded registry",
			args: args{
				mirrorAddr: "127.0.0.1:6443",
				registryContent: `
				  mirrors:
						"*":
							endpoint:
								- https://registry.example.com/v2
				`,
			},
			want: HostConfigs{
				"_default": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u(""),
					},
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("https://127.0.0.1:6443/v2"),
							Config: registries.RegistryConfig{
								TLS: &registries.TLSConfig{
									CAFile:   "server-ca",
									KeyFile:  "client-key",
									CertFile: "client-cert",
								},
							},
						},
						{
							URL: u("https://registry.example.com/v2"),
						},
					},
				},
				"127.0.0.1:6443": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://127.0.0.1:6443/v2"),
						Config: registries.RegistryConfig{
							TLS: &registries.TLSConfig{
								CAFile:   "server-ca",
								KeyFile:  "client-key",
								CertFile: "client-cert",
							},
						},
					},
				},
			},
		},
		{
			name: "wildcard mirror endpoint - full URL, embedded registry, no default",
			args: args{
				noDefaultEndpoint: true,
				mirrorAddr:        "127.0.0.1:6443",
				registryContent: `
				  mirrors:
						"*":
							endpoint:
								- https://registry.example.com/v2
				`,
			},
			want: HostConfigs{
				"_default": templates.HostConfig{
					Program: "k3s",
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("https://127.0.0.1:6443/v2"),
							Config: registries.RegistryConfig{
								TLS: &registries.TLSConfig{
									CAFile:   "server-ca",
									KeyFile:  "client-key",
									CertFile: "client-cert",
								},
							},
						},
						{
							URL: u("https://registry.example.com/v2"),
						},
					},
				},
				"127.0.0.1:6443": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://127.0.0.1:6443/v2"),
						Config: registries.RegistryConfig{
							TLS: &registries.TLSConfig{
								CAFile:   "server-ca",
								KeyFile:  "client-key",
								CertFile: "client-cert",
							},
						},
					},
				},
			},
		},

		{
			name: "wildcard config",
			args: args{
				registryContent: `
				  configs:
						"*":
						  auth:
							  username: user
								password: pass
							tls:
								insecure_skip_verify: true
				`,
			},
			want: HostConfigs{
				"_default": {
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						Config: registries.RegistryConfig{
							Auth: &registries.AuthConfig{
								Username: "user",
								Password: "pass",
							},
							TLS: &registries.TLSConfig{
								InsecureSkipVerify: true,
							},
						},
					},
				},
			},
		},
		{
			name: "localhost registry - default https endpoint on unspecified port",
			args: args{
				registryContent: `
				  mirrors:
						"localhost":
				`,
			},
			want: HostConfigs{},
		},
		{
			name: "localhost registry - default https endpoint on https port",
			args: args{
				registryContent: `
				  mirrors:
						"localhost:443":
				`,
			},
			want: HostConfigs{},
		},
		{
			name: "localhost registry - default http endpoint on odd port",
			args: args{
				registryContent: `
				  mirrors:
						"localhost:5000":
				`,
			},
			want: HostConfigs{},
		},
		{
			name: "localhost registry - default http endpoint on http port",
			args: args{
				registryContent: `
				  mirrors:
						"localhost:80":
				`,
			},
			want: HostConfigs{},
		},
		{
			name: "localhost registry - default http endpoint on odd port, embedded registry",
			args: args{
				mirrorAddr: "127.0.0.1:6443",
				registryContent: `
				  mirrors:
						"localhost:5000":
				`,
			},
			want: HostConfigs{
				// localhost registries are not handled by the embedded registry mirror.
				"127.0.0.1:6443": templates.HostConfig{
					Program: "k3s",
					Default: &templates.RegistryEndpoint{
						URL: u("https://127.0.0.1:6443/v2"),
						Config: registries.RegistryConfig{
							TLS: &registries.TLSConfig{
								CAFile:   "server-ca",
								KeyFile:  "client-key",
								CertFile: "client-cert",
							},
						},
					},
				},
			},
		},
		{
			name: "localhost registry - https endpoint on odd port with tls verification disabled",
			args: args{
				registryContent: `
				  mirrors:
						localhost:5000:
						  endpoint:
							  - https://localhost:5000
					configs:
						localhost:5000:
						  tls:
							  insecure_skip_verify: true
				`,
			},
			want: HostConfigs{
				"localhost:5000": templates.HostConfig{
					Default: &templates.RegistryEndpoint{
						URL: u("http://localhost:5000/v2"),
						Config: registries.RegistryConfig{
							TLS: &registries.TLSConfig{
								InsecureSkipVerify: true,
							},
						},
					},
					Program: "k3s",
					Endpoints: []templates.RegistryEndpoint{
						{
							URL: u("https://localhost:5000/v2"),
							Config: registries.RegistryConfig{
								TLS: &registries.TLSConfig{
									InsecureSkipVerify: true,
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// replace tabs from the inline yaml with spaces; yaml doesn't support tabs for indentation.
			tt.args.registryContent = strings.ReplaceAll(tt.args.registryContent, "\t", "  ")
			tempDir := t.TempDir()
			registriesFile := filepath.Join(tempDir, "registries.yaml")
			os.WriteFile(registriesFile, []byte(tt.args.registryContent), 0644)
			t.Logf("%s:\n%s", registriesFile, tt.args.registryContent)

			registry, err := registries.GetPrivateRegistries(registriesFile)
			if err != nil {
				t.Fatalf("failed to parse %s: %v\n", registriesFile, err)
			}

			// This is an odd mishmash of linux and windows stuff just to excercise all the template bits
			nodeConfig := &config.Node{
				DefaultRuntime: "runhcs-wcow-process",
				Containerd: config.Containerd{
					Registry: tempDir + "/hosts.d",
					Config:   tempDir + "/config.toml",
					Template: tempDir,
					Address:  "/run/k3s/containerd/containerd.sock",
					Root:     "/var/lib/rancher/k3s/agent/containerd",
					Opt:      "/var/lib/rancher/k3s/agent/containerd",
					State:    "/run/k3s/containerd",
				},
				AgentConfig: config.Agent{
					ImageServiceSocket: "containerd-stargz-grpc.sock",
					Registry:           registry.Registry,
					Snapshotter:        "stargz",
					CNIBinDir:          "/var/lib/rancher/k3s/data/cni",
					CNIConfDir:         "/var/lib/rancher/k3s/agent/etc/cni/net.d",
				},
			}

			// set up embedded registry, if enabled for the test
			if tt.args.mirrorAddr != "" {
				conf := spegel.DefaultRegistry
				conf.ServerCAFile = "server-ca"
				conf.ClientKeyFile = "client-key"
				conf.ClientCertFile = "client-cert"
				conf.InternalAddress, conf.RegistryPort, _ = net.SplitHostPort(tt.args.mirrorAddr)
				conf.InjectMirror(nodeConfig)
			}

			// Generate config template struct for all hosts
			got := getHostConfigs(registry.Registry, tt.args.noDefaultEndpoint, tt.args.mirrorAddr)
			assert.Equal(t, tt.want, got, "getHostConfigs()")

			// Confirm that hosts.toml renders properly for all registries
			for host, config := range got {
				hostsToml, err := templates.ParseHostsTemplateFromConfig(templates.HostsTomlTemplate, config)
				assert.NoError(t, err, "ParseHostTemplateFromConfig for %s", host)
				t.Logf("%s/hosts.d/%s/hosts.toml\n%s", tempDir, hostDirectory(host), hostsToml)
			}

			for _, template := range []string{"config.toml.tmpl", "config-v3.toml.tmpl"} {
				t.Run(template, func(t *testing.T) {
					templateFile := filepath.Join(tempDir, template)
					err = os.WriteFile(templateFile, []byte(`{{ template "base" . }}`), 0600)
					assert.NoError(t, err, "Write Template")

					// Confirm that the main containerd config.toml renders properly
					containerdConfig := templates.ContainerdConfig{
						NodeConfig:            nodeConfig,
						PrivateRegistryConfig: registry.Registry,
						Program:               "k3s",
						ExtraRuntimes: map[string]templates.ContainerdRuntimeConfig{
							"wasmtime": {
								RuntimeType: "io.containerd.wasmtime.v1",
								BinaryName:  "containerd-shim-wasmtime-v1",
							},
						},
					}
					err = writeContainerdConfig(nodeConfig, containerdConfig)
					assert.NoError(t, err, "ParseTemplateFromConfig")
					configToml, err := os.ReadFile(nodeConfig.Containerd.Config)
					assert.NoError(t, err, "ReadFile "+nodeConfig.Containerd.Config)
					t.Logf("%s\n%s", nodeConfig.Containerd.Config, configToml)
				})
			}
		})
	}
}

// replacementTemplate is a full-replacement custom template in the common
// nvidia-runtime shape: it carries the registry section with only config_path,
// and no registry configs auth stanza.
const replacementTemplate = `
version = 2
root = "/var/lib/rancher/k3s/agent/containerd"
state = "/run/k3s/containerd"

[plugins."io.containerd.grpc.v1.cri".registry]
  config_path = "/var/lib/rancher/k3s/agent/etc/containerd/certs.d"

[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia]
  runtime_type = "io.containerd.runc.v2"

[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia.options]
  BinaryName = "/usr/bin/nvidia-container-runtime"
  SystemdCgroup = true
`

func Test_UnitWarnMissingRegistryAuth(t *testing.T) {
	tests := []struct {
		name            string
		registryContent string
		userTemplate    string
		templateName    string
		wantWarnHosts   []string
	}{
		{
			name: "auth configured, no user template",
			registryContent: `
			  configs:
				  ghcr.io:
					  auth:
							username: user
							password: pass
			`,
			wantWarnHosts: nil,
		},
		{
			name: "auth configured, custom template extends base",
			registryContent: `
			  configs:
				  ghcr.io:
					  auth:
							username: user
							password: pass
			`,
			userTemplate: `{{ template "base" . }}

[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia]
  runtime_type = "io.containerd.runc.v2"
`,
			templateName:  "config.toml.tmpl",
			wantWarnHosts: nil,
		},
		{
			name: "auth configured, replacement template drops it",
			registryContent: `
			  configs:
				  ghcr.io:
					  auth:
							username: user
							password: pass
			`,
			userTemplate:  replacementTemplate,
			templateName:  "config.toml.tmpl",
			wantWarnHosts: []string{"ghcr.io"},
		},
		{
			name: "auth configured, replacement template drops it - v3",
			registryContent: `
			  configs:
				  ghcr.io:
					  auth:
							username: user
							password: pass
			`,
			userTemplate:  replacementTemplate,
			templateName:  "config-v3.toml.tmpl",
			wantWarnHosts: []string{"ghcr.io"},
		},
		{
			name: "auth for multiple registries dropped",
			registryContent: `
			  configs:
				  ghcr.io:
					  auth:
							username: user
							password: pass
					registry.example.com:
						auth:
							username: user
							password: pass
			`,
			userTemplate:  replacementTemplate,
			templateName:  "config.toml.tmpl",
			wantWarnHosts: []string{"ghcr.io", "registry.example.com"},
		},
		{
			name: "no auth configured, replacement template",
			registryContent: `
			  mirrors:
				  docker.io:
						endpoint:
							- https://registry.example.com/v2
			`,
			userTemplate:  replacementTemplate,
			templateName:  "config.toml.tmpl",
			wantWarnHosts: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.registryContent = strings.ReplaceAll(tt.registryContent, "\t", "  ")
			tempDir := t.TempDir()
			registriesFile := filepath.Join(tempDir, "registries.yaml")
			assert.NoError(t, os.WriteFile(registriesFile, []byte(tt.registryContent), 0644))

			registry, err := registries.GetPrivateRegistries(registriesFile)
			assert.NoError(t, err, "GetPrivateRegistries")

			nodeConfig := &config.Node{
				Containerd: config.Containerd{
					Registry: tempDir + "/hosts.d",
					Config:   tempDir + "/config.toml",
					Template: tempDir,
					Address:  "/run/k3s/containerd/containerd.sock",
					Root:     "/var/lib/rancher/k3s/agent/containerd",
					Opt:      "/var/lib/rancher/k3s/agent/containerd",
					State:    "/run/k3s/containerd",
				},
				AgentConfig: config.Agent{
					Registry: registry.Registry,
				},
			}

			if tt.userTemplate != "" {
				templateFile := filepath.Join(tempDir, tt.templateName)
				assert.NoError(t, os.WriteFile(templateFile, []byte(tt.userTemplate), 0600))
			}

			containerdConfig := templates.ContainerdConfig{
				NodeConfig:            nodeConfig,
				PrivateRegistryConfig: registry.Registry,
				Program:               "k3s",
			}
			hook := logtest.NewGlobal()
			defer hook.Reset()
			assert.NoError(t, writeContainerdConfig(nodeConfig, containerdConfig), "writeContainerdConfig")

			var warns []string
			for _, entry := range hook.AllEntries() {
				if entry.Level == logrus.WarnLevel {
					warns = append(warns, entry.Message)
				}
			}
			if len(tt.wantWarnHosts) == 0 {
				assert.Empty(t, warns, "expected no warnings")
				return
			}
			if assert.Len(t, warns, 1, "expected a single warning") {
				for _, host := range tt.wantWarnHosts {
					assert.Contains(t, warns[0], host)
				}
			}
		})
	}
}

func Test_UnitRegistryAuthPresent(t *testing.T) {
	v2Style := `
[plugins."io.containerd.grpc.v1.cri".registry.configs."ghcr.io".auth]
  username = "user"
`
	v3Style := `
[plugins.'io.containerd.cri.v1.images'.registry.configs.'ghcr.io'.auth]
  username = "user"
`
	assert.True(t, registryAuthPresent(v2Style, "ghcr.io"), "v2 double-quoted stanza not detected")
	assert.True(t, registryAuthPresent(v3Style, "ghcr.io"), "v3 single-quoted stanza not detected")
	assert.False(t, registryAuthPresent(v2Style, "docker.io"), "auth detected for a registry that is not configured")
	assert.True(t, registryAuthPresent("not = valid toml", "ghcr.io"), "unparseable config should not warn")
}
