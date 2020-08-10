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

// config package containers utilities for helping configure the Docker resolver
package config

import (
	"context"
	"crypto/tls"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/pkg/errors"
)

type hostConfig struct {
	scheme string
	host   string
	path   string

	capabilities docker.HostCapabilities

	caCerts     []string
	clientPairs [][2]string
	skipVerify  *bool

	header http.Header

	// TODO: API ("docker" or "oci")
	// TODO: API Version ("v1", "v2")
	// TODO: Add credential configuration (domain alias, username)
}

// HostOptions is used to configure registry hosts
type HostOptions struct {
	HostDir       func(string) (string, error)
	Credentials   func(host string) (string, string, error)
	DefaultTLS    *tls.Config
	DefaultScheme string
}

// ConfigureHosts creates a registry hosts function from the provided
// host creation options. The host directory can read hosts.toml or
// certificate files laid out in the Docker specific layout.
// If a `HostDir` function is not required, defaults are used.
func ConfigureHosts(ctx context.Context, options HostOptions) docker.RegistryHosts {
	return func(host string) ([]docker.RegistryHost, error) {
		var hosts []hostConfig
		if options.HostDir != nil {
			dir, err := options.HostDir(host)
			if err != nil && !errdefs.IsNotFound(err) {
				return nil, err
			}
			if dir != "" {
				log.G(ctx).WithField("dir", dir).Debug("loading host directory")
				hosts, err = loadHostDir(ctx, dir)
				if err != nil {
					return nil, err
				}
			}

		}

		// If hosts was not set, add a default host
		// NOTE: Check nil here and not empty, the host may be
		// intentionally configured to not have any endpoints
		if hosts == nil {
			hosts = make([]hostConfig, 1)
		}
		if len(hosts) > 0 && hosts[len(hosts)-1].host == "" {
			if host == "docker.io" {
				hosts[len(hosts)-1].scheme = "https"
				hosts[len(hosts)-1].host = "registry-1.docker.io"
			} else {
				hosts[len(hosts)-1].host = host
				if options.DefaultScheme != "" {
					hosts[len(hosts)-1].scheme = options.DefaultScheme
				} else {
					hosts[len(hosts)-1].scheme = "https"
				}
			}
			hosts[len(hosts)-1].path = "/v2"
			hosts[len(hosts)-1].capabilities = docker.HostCapabilityPull | docker.HostCapabilityResolve | docker.HostCapabilityPush
		}

		var defaultTLSConfig *tls.Config
		if options.DefaultTLS != nil {
			defaultTLSConfig = options.DefaultTLS
		} else {
			defaultTLSConfig = &tls.Config{}
		}

		defaultTransport := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:       30 * time.Second,
				KeepAlive:     30 * time.Second,
				FallbackDelay: 300 * time.Millisecond,
			}).DialContext,
			MaxIdleConns:          10,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			TLSClientConfig:       defaultTLSConfig,
			ExpectContinueTimeout: 5 * time.Second,
		}

		client := &http.Client{
			Transport: defaultTransport,
		}

		authOpts := []docker.AuthorizerOpt{docker.WithAuthClient(client)}
		if options.Credentials != nil {
			authOpts = append(authOpts, docker.WithAuthCreds(options.Credentials))
		}
		authorizer := docker.NewDockerAuthorizer(authOpts...)

		rhosts := make([]docker.RegistryHost, len(hosts))
		for i, host := range hosts {

			rhosts[i].Scheme = host.scheme
			rhosts[i].Host = host.host
			rhosts[i].Path = host.path
			rhosts[i].Capabilities = host.capabilities
			rhosts[i].Header = host.header

			if host.caCerts != nil || host.clientPairs != nil || host.skipVerify != nil {
				tr := defaultTransport.Clone()
				tlsConfig := tr.TLSClientConfig
				if host.skipVerify != nil {
					tlsConfig.InsecureSkipVerify = *host.skipVerify
				}
				if host.caCerts != nil {
					if tlsConfig.RootCAs == nil {
						rootPool, err := rootSystemPool()
						if err != nil {
							return nil, errors.Wrap(err, "unable to initialize cert pool")
						}
						tlsConfig.RootCAs = rootPool
					}
					for _, f := range host.caCerts {
						data, err := ioutil.ReadFile(f)
						if err != nil {
							return nil, errors.Wrapf(err, "unable to read CA cert %q", f)
						}
						if !tlsConfig.RootCAs.AppendCertsFromPEM(data) {
							return nil, errors.Errorf("unable to load CA cert %q", f)
						}
					}
				}

				if host.clientPairs != nil {
					for _, pair := range host.clientPairs {
						certPEMBlock, err := ioutil.ReadFile(pair[0])
						if err != nil {
							return nil, errors.Wrapf(err, "unable to read CERT file %q", pair[0])
						}
						var keyPEMBlock []byte
						if pair[1] != "" {
							keyPEMBlock, err = ioutil.ReadFile(pair[1])
							if err != nil {
								return nil, errors.Wrapf(err, "unable to read CERT file %q", pair[1])
							}
						} else {
							// Load key block from same PEM file
							keyPEMBlock = certPEMBlock
						}
						cert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
						if err != nil {
							return nil, errors.Wrap(err, "failed to load X509 key pair")
						}

						tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
					}
				}

				c := *client
				c.Transport = tr

				rhosts[i].Client = &c
				rhosts[i].Authorizer = docker.NewDockerAuthorizer(append(authOpts, docker.WithAuthClient(&c))...)
			} else {
				rhosts[i].Client = client
				rhosts[i].Authorizer = authorizer
			}
		}

		return rhosts, nil
	}

}

// HostDirFromRoot returns a function which finds a host directory
// based at the given root.
func HostDirFromRoot(root string) func(string) (string, error) {
	return func(host string) (string, error) {
		for _, p := range hostPaths(root, host) {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			} else if !os.IsNotExist(err) {
				return "", err
			}
		}
		return "", errdefs.ErrNotFound
	}
}

// hostDirectory converts ":port" to "_port_" in directory names
func hostDirectory(host string) string {
	idx := strings.LastIndex(host, ":")
	if idx > 0 {
		return host[:idx] + "_" + host[idx+1:] + "_"
	}
	return host
}

func loadHostDir(ctx context.Context, hostsDir string) ([]hostConfig, error) {
	b, err := ioutil.ReadFile(filepath.Join(hostsDir, "hosts.toml"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if len(b) == 0 {
		// If hosts.toml does not exist, fallback to checking for
		// certificate files based on Docker's certificate file
		// pattern (".crt", ".cert", ".key" files)
		return loadCertFiles(ctx, hostsDir)
	}

	hosts, err := parseHostsFile(ctx, hostsDir, b)
	if err != nil {
		log.G(ctx).WithError(err).Error("failed to decode hosts.toml")
		// Fallback to checking certificate files
		return loadCertFiles(ctx, hostsDir)
	}

	return hosts, nil
}

type hostFileConfig struct {
	// Capabilities determine what operations a host is
	// capable of performing. Allowed values
	//  - pull
	//  - resolve
	//  - push
	Capabilities []string `toml:"capabilities"`

	// CACert can be a string or an array of strings
	CACert toml.Primitive `toml:"ca"`

	// TODO: Make this an array (two key types, one for pairs (multiple files), one for single file?)
	Client toml.Primitive `toml:"client"`

	SkipVerify *bool `toml:"skip_verify"`

	Header map[string]toml.Primitive `toml:"header"`

	// API (default: "docker")
	// API Version (default: "v2")
	// Credentials: helper? name? username? alternate domain? token?
}

type configFile struct {
	// hostConfig holds defaults for all hosts as well as
	// for the default server
	hostFileConfig

	// Server specifies the default server. When `host` is
	// also specified, those hosts are tried first.
	Server string `toml:"server"`

	// HostConfigs store the per-host configuration
	HostConfigs map[string]hostFileConfig `toml:"host"`
}

func parseHostsFile(ctx context.Context, baseDir string, b []byte) ([]hostConfig, error) {
	var c configFile
	md, err := toml.Decode(string(b), &c)
	if err != nil {
		return nil, err
	}

	var orderedHosts []string
	for _, key := range md.Keys() {
		if len(key) >= 2 {
			if key[0] == "host" && (len(orderedHosts) == 0 || orderedHosts[len(orderedHosts)-1] != key[1]) {
				orderedHosts = append(orderedHosts, key[1])
			}
		}
	}

	if c.HostConfigs == nil {
		c.HostConfigs = map[string]hostFileConfig{}
	}
	if c.Server != "" {
		c.HostConfigs[c.Server] = c.hostFileConfig
		orderedHosts = append(orderedHosts, c.Server)
	} else if len(orderedHosts) == 0 {
		c.HostConfigs[""] = c.hostFileConfig
		orderedHosts = append(orderedHosts, "")
	}
	hosts := make([]hostConfig, len(orderedHosts))
	for i, server := range orderedHosts {
		hostConfig := c.HostConfigs[server]

		if server != "" {
			if !strings.HasPrefix(server, "http") {
				server = "https://" + server
			}
			u, err := url.Parse(server)
			if err != nil {
				return nil, errors.Errorf("unable to parse server %v", server)
			}
			hosts[i].scheme = u.Scheme
			hosts[i].host = u.Host

			// TODO: Handle path based on registry protocol
			// Define a registry protocol type
			//   OCI v1    - Always use given path as is
			//   Docker v2 - Always ensure ends with /v2/
			if len(u.Path) > 0 {
				u.Path = path.Clean(u.Path)
				if !strings.HasSuffix(u.Path, "/v2") {
					u.Path = u.Path + "/v2"
				}
			} else {
				u.Path = "/v2"
			}
			hosts[i].path = u.Path
		}
		hosts[i].skipVerify = hostConfig.SkipVerify

		if len(hostConfig.Capabilities) > 0 {
			for _, c := range hostConfig.Capabilities {
				switch strings.ToLower(c) {
				case "pull":
					hosts[i].capabilities |= docker.HostCapabilityPull
				case "resolve":
					hosts[i].capabilities |= docker.HostCapabilityResolve
				case "push":
					hosts[i].capabilities |= docker.HostCapabilityPush
				default:
					return nil, errors.Errorf("unknown capability %v", c)
				}
			}
		} else {
			hosts[i].capabilities = docker.HostCapabilityPull | docker.HostCapabilityResolve | docker.HostCapabilityPush
		}

		baseKey := []string{}
		if server != "" && server != c.Server {
			baseKey = append(baseKey, "host", server)
		}
		caKey := append(baseKey, "ca")
		if md.IsDefined(caKey...) {
			switch t := md.Type(caKey...); t {
			case "String":
				var caCert string
				if err := md.PrimitiveDecode(hostConfig.CACert, &caCert); err != nil {
					return nil, errors.Wrap(err, "failed to decode \"ca\"")
				}
				hosts[i].caCerts = []string{makeAbsPath(caCert, baseDir)}
			case "Array":
				var caCerts []string
				if err := md.PrimitiveDecode(hostConfig.CACert, &caCerts); err != nil {
					return nil, errors.Wrap(err, "failed to decode \"ca\"")
				}
				for i, p := range caCerts {
					caCerts[i] = makeAbsPath(p, baseDir)
				}

				hosts[i].caCerts = caCerts
			default:
				return nil, errors.Errorf("invalid type %v for \"ca\"", t)
			}
		}

		clientKey := append(baseKey, "client")
		if md.IsDefined(clientKey...) {
			switch t := md.Type(clientKey...); t {
			case "String":
				var clientCert string
				if err := md.PrimitiveDecode(hostConfig.Client, &clientCert); err != nil {
					return nil, errors.Wrap(err, "failed to decode \"ca\"")
				}
				hosts[i].clientPairs = [][2]string{{makeAbsPath(clientCert, baseDir), ""}}
			case "Array":
				var clientCerts []interface{}
				if err := md.PrimitiveDecode(hostConfig.Client, &clientCerts); err != nil {
					return nil, errors.Wrap(err, "failed to decode \"ca\"")
				}
				for _, pairs := range clientCerts {
					switch p := pairs.(type) {
					case string:
						hosts[i].clientPairs = append(hosts[i].clientPairs, [2]string{makeAbsPath(p, baseDir), ""})
					case []interface{}:
						var pair [2]string
						if len(p) > 2 {
							return nil, errors.Errorf("invalid pair %v for \"client\"", p)
						}
						for pi, cp := range p {
							s, ok := cp.(string)
							if !ok {
								return nil, errors.Errorf("invalid type %T for \"client\"", cp)
							}
							pair[pi] = makeAbsPath(s, baseDir)
						}
						hosts[i].clientPairs = append(hosts[i].clientPairs, pair)
					default:
						return nil, errors.Errorf("invalid type %T for \"client\"", p)
					}
				}
			default:
				return nil, errors.Errorf("invalid type %v for \"client\"", t)
			}
		}

		headerKey := append(baseKey, "header")
		if md.IsDefined(headerKey...) {
			header := http.Header{}
			for key, prim := range hostConfig.Header {
				switch t := md.Type(append(headerKey, key)...); t {
				case "String":
					var value string
					if err := md.PrimitiveDecode(prim, &value); err != nil {
						return nil, errors.Wrapf(err, "failed to decode header %q", key)
					}
					header[key] = []string{value}
				case "Array":
					var value []string
					if err := md.PrimitiveDecode(prim, &value); err != nil {
						return nil, errors.Wrapf(err, "failed to decode header %q", key)
					}

					header[key] = value
				default:
					return nil, errors.Errorf("invalid type %v for header %q", t, key)
				}
			}
			hosts[i].header = header
		}
	}

	return hosts, nil
}

func makeAbsPath(p string, base string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(base, p)
}

// loadCertsDir loads certs from certsDir like "/etc/docker/certs.d" .
// Compatible with Docker file layout
// - files ending with ".crt" are treated as CA certificate files
// - files ending with ".cert" are treated as client certificates, and
//   files with the same name but ending with ".key" are treated as the
//   corresponding private key.
//   NOTE: If a ".key" file is missing, this function will just return
//   the ".cert", which may contain the private key. If the ".cert" file
//   does not contain the private key, the caller should detect and error.
func loadCertFiles(ctx context.Context, certsDir string) ([]hostConfig, error) {
	fs, err := ioutil.ReadDir(certsDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	hosts := make([]hostConfig, 1)
	for _, f := range fs {
		if !f.IsDir() {
			continue
		}
		if strings.HasSuffix(f.Name(), ".crt") {
			hosts[0].caCerts = append(hosts[0].caCerts, filepath.Join(certsDir, f.Name()))
		}
		if strings.HasSuffix(f.Name(), ".cert") {
			var pair [2]string
			certFile := f.Name()
			pair[0] = filepath.Join(certsDir, certFile)
			// Check if key also exists
			keyFile := certFile[:len(certFile)-5] + ".key"
			if _, err := os.Stat(keyFile); err == nil {
				pair[1] = filepath.Join(certsDir, keyFile)
			} else if !os.IsNotExist(err) {
				return nil, err
			}
			hosts[0].clientPairs = append(hosts[0].clientPairs, pair)
		}
	}
	return hosts, nil
}
