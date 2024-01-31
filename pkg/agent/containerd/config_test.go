package containerd

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/pkg/agent/templates"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/spegel"
	"github.com/rancher/wharfie/pkg/registries"
	"github.com/stretchr/testify/assert"
)

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
					DefaultEndpoint: "https://registry-1.docker.io/v2",
					Program:         "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://127.0.0.1:6443/v2",
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
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://127.0.0.1:6443/v2",
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
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://registry-1.docker.io/v2",
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
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://registry-1.docker.io/v2",
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
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://registry.example.com/v2",
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
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://registry.example.com/v2",
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
					DefaultEndpoint: "https://registry-1.docker.io/v2",
					Program:         "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							OverridePath: true,
							URI:          "https://registry.example.com/prefix/v2",
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
					DefaultEndpoint: "https://registry-1.docker.io/v2",
					Program:         "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							OverridePath: true,
							URI:          "https://registry.example.com/prefix/v2",
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
					DefaultEndpoint: "https://registry-1.docker.io/v2",
					Program:         "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://registry.example.com/v2",
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
					DefaultEndpoint: "https://registry-1.docker.io/v2",
					Program:         "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://registry.example.com/v2",
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
					DefaultEndpoint: "https://registry-1.docker.io/v2",
					Program:         "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://registry.example.com/v2",
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
					DefaultEndpoint: "https://registry-1.docker.io/v2",
					Program:         "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://registry.example.com/v2",
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
					DefaultEndpoint: "https://registry-1.docker.io/v2",
					Program:         "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://registry.example.com:443/v2",
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
					DefaultEndpoint: "https://registry-1.docker.io/v2",
					Program:         "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://1.2.3.4/v2",
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
					DefaultEndpoint: "https://registry-1.docker.io/v2",
					Program:         "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://1.2.3.4:443/v2",
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
					DefaultEndpoint: "https://registry-1.docker.io/v2",
					Program:         "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "http://localhost:5000/v2",
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
					DefaultEndpoint: "https://registry-1.docker.io/v2",
					Program:         "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://localhost:5000/v2",
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
					DefaultEndpoint: "https://registry-1.docker.io/v2",
					Program:         "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "http://127.0.0.1:5000/v2",
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
					DefaultEndpoint: "https://registry-1.docker.io/v2",
					Program:         "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://127.0.0.1:5000/v2",
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
					DefaultEndpoint: "https://registry-1.docker.io/v2",
					Program:         "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://registry.example.com/v2",
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
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://registry.example.com/v2",
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
					DefaultEndpoint: "https://registry-1.docker.io/v2",
					Program:         "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							OverridePath: true,
							URI:          "https://registry.example.com/prefix/v2",
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
					DefaultEndpoint: "https://registry.example.com/v2",
					Program:         "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							OverridePath: true,
							URI:          "https://registry.example.com/prefix/v2",
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
					DefaultEndpoint: "https://registry-1.docker.io/v2",
					Program:         "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							OverridePath: true,
							URI:          "https://registry.example.com/project/registry",
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
					DefaultEndpoint: "https://registry.example.com/v2",
					Program:         "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							OverridePath: true,
							URI:          "https://registry.example.com/project/registry",
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
						templates.RegistryEndpoint{
							URI: "https://registry.example.com/v2",
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
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://registry.example.com/v2",
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
					DefaultEndpoint: "https://registry-1.docker.io/v2",
					Program:         "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://127.0.0.1:6443/v2",
							Config: registries.RegistryConfig{
								TLS: &registries.TLSConfig{
									CAFile:   "server-ca",
									KeyFile:  "client-key",
									CertFile: "client-cert",
								},
							},
						},
						templates.RegistryEndpoint{
							URI: "https://registry.example.com/v2",
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
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://registry.example.com/v2",
							Config: registries.RegistryConfig{
								Auth: &registries.AuthConfig{
									Username: "user",
									Password: "pass",
								},
							},
						},
					},
				},
				"127.0.0.1:6443": templates.HostConfig{
					Program: "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://127.0.0.1:6443/v2",
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
					DefaultEndpoint: "https://registry-1.docker.io/v2",
					Program:         "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://127.0.0.1:6443/v2",
							Config: registries.RegistryConfig{
								TLS: &registries.TLSConfig{
									CAFile:   "server-ca",
									KeyFile:  "client-key",
									CertFile: "client-cert",
								},
							},
						},
						templates.RegistryEndpoint{
							URI: "https://registry.example.com/v2",
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
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://registry.example.com/v2",
							Config: registries.RegistryConfig{
								Auth: &registries.AuthConfig{
									Username: "user",
									Password: "pass",
								},
							},
						},
					},
				},
				"127.0.0.1:6443": templates.HostConfig{
					Program: "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://127.0.0.1:6443/v2",
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
						templates.RegistryEndpoint{
							URI: "https://127.0.0.1:6443/v2",
							Config: registries.RegistryConfig{
								TLS: &registries.TLSConfig{
									CAFile:   "server-ca",
									KeyFile:  "client-key",
									CertFile: "client-cert",
								},
							},
						},
						templates.RegistryEndpoint{
							URI: "https://registry.example.com/v2",
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
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://registry.example.com/v2",
							Config: registries.RegistryConfig{
								Auth: &registries.AuthConfig{
									Username: "user",
									Password: "pass",
								},
							},
						},
					},
				},
				"127.0.0.1:6443": templates.HostConfig{
					Program: "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://127.0.0.1:6443/v2",
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
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://registry.example.com/v2",
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
					Endpoints: []templates.RegistryEndpoint{
						// note that the embedded registry mirror is NOT listed as an endpoint.
						// individual registries must be enabled for mirroring by name.
						templates.RegistryEndpoint{
							URI: "https://registry.example.com/v2",
						},
					},
				},
				"127.0.0.1:6443": templates.HostConfig{
					Program: "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://127.0.0.1:6443/v2",
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
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://127.0.0.1:6443/v2",
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
					DefaultEndpoint: "http://localhost:5000/v2",
					Program:         "k3s",
					Endpoints: []templates.RegistryEndpoint{
						templates.RegistryEndpoint{
							URI: "https://localhost:5000/v2",
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
			registriesFile := filepath.Join(t.TempDir(), "registries.yaml")
			os.WriteFile(registriesFile, []byte(tt.args.registryContent), 0644)

			registry, err := registries.GetPrivateRegistries(registriesFile)
			if err != nil {
				t.Fatalf("failed to parse %s: %v\n", registriesFile, err)
			}

			// set up embedded registry, if enabled for the test
			if tt.args.mirrorAddr != "" {
				conf := spegel.DefaultRegistry
				conf.ServerCAFile = "server-ca"
				conf.ClientKeyFile = "client-key"
				conf.ClientCertFile = "client-cert"
				conf.InternalAddress, conf.RegistryPort, _ = net.SplitHostPort(tt.args.mirrorAddr)
				conf.InjectMirror(&config.Node{AgentConfig: config.Agent{Registry: registry.Registry}})
			}

			got := getHostConfigs(registry.Registry, tt.args.noDefaultEndpoint, tt.args.mirrorAddr)
			assert.Equal(t, tt.want, got, "getHostConfigs()")
		})
	}
}
