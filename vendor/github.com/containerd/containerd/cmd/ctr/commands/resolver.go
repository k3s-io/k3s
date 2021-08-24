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

package commands

import (
	"bufio"
	gocontext "context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptrace"
	"net/http/httputil"
	"strings"

	"github.com/containerd/console"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/remotes/docker/config"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

// PushTracker returns a new InMemoryTracker which tracks the ref status
var PushTracker = docker.NewInMemoryTracker()

func passwordPrompt() (string, error) {
	c := console.Current()
	defer c.Reset()

	if err := c.DisableEcho(); err != nil {
		return "", errors.Wrap(err, "failed to disable echo")
	}

	line, _, err := bufio.NewReader(c).ReadLine()
	if err != nil {
		return "", errors.Wrap(err, "failed to read line")
	}
	return string(line), nil
}

// GetResolver prepares the resolver from the environment and options
func GetResolver(ctx gocontext.Context, clicontext *cli.Context) (remotes.Resolver, error) {
	username := clicontext.String("user")
	var secret string
	if i := strings.IndexByte(username, ':'); i > 0 {
		secret = username[i+1:]
		username = username[0:i]
	}
	options := docker.ResolverOptions{
		Tracker: PushTracker,
	}
	if username != "" {
		if secret == "" {
			fmt.Printf("Password: ")

			var err error
			secret, err = passwordPrompt()
			if err != nil {
				return nil, err
			}

			fmt.Print("\n")
		}
	} else if rt := clicontext.String("refresh"); rt != "" {
		secret = rt
	}

	hostOptions := config.HostOptions{}
	hostOptions.Credentials = func(host string) (string, string, error) {
		// If host doesn't match...
		// Only one host
		return username, secret, nil
	}
	if clicontext.Bool("plain-http") {
		hostOptions.DefaultScheme = "http"
	}
	defaultTLS, err := resolverDefaultTLS(clicontext)
	if err != nil {
		return nil, err
	}
	hostOptions.DefaultTLS = defaultTLS
	if hostDir := clicontext.String("hosts-dir"); hostDir != "" {
		hostOptions.HostDir = config.HostDirFromRoot(hostDir)
	}

	if clicontext.Bool("http-dump") {
		hostOptions.UpdateClient = func(client *http.Client) error {
			client.Transport = &DebugTransport{
				transport: client.Transport,
				writer:    log.G(ctx).Writer(),
			}
			return nil
		}
	}

	options.Hosts = config.ConfigureHosts(ctx, hostOptions)

	return docker.NewResolver(options), nil
}

func resolverDefaultTLS(clicontext *cli.Context) (*tls.Config, error) {
	config := &tls.Config{}

	if clicontext.Bool("skip-verify") {
		config.InsecureSkipVerify = true
	}

	if tlsRootPath := clicontext.String("tlscacert"); tlsRootPath != "" {
		tlsRootData, err := ioutil.ReadFile(tlsRootPath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read %q", tlsRootPath)
		}

		config.RootCAs = x509.NewCertPool()
		if !config.RootCAs.AppendCertsFromPEM(tlsRootData) {
			return nil, fmt.Errorf("failed to load TLS CAs from %q: invalid data", tlsRootPath)
		}
	}

	tlsCertPath := clicontext.String("tlscert")
	tlsKeyPath := clicontext.String("tlskey")
	if tlsCertPath != "" || tlsKeyPath != "" {
		if tlsCertPath == "" || tlsKeyPath == "" {
			return nil, errors.New("flags --tlscert and --tlskey must be set together")
		}
		keyPair, err := tls.LoadX509KeyPair(tlsCertPath, tlsKeyPath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load TLS client credentials (cert=%q, key=%q)", tlsCertPath, tlsKeyPath)
		}
		config.Certificates = []tls.Certificate{keyPair}
	}

	return config, nil
}

// DebugTransport wraps the underlying http.RoundTripper interface and dumps all requests/responses to the writer.
type DebugTransport struct {
	transport http.RoundTripper
	writer    io.Writer
}

// RoundTrip dumps request/responses and executes the request using the underlying transport.
func (t DebugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	in, err := httputil.DumpRequest(req, true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to dump request")
	}

	if _, err := t.writer.Write(in); err != nil {
		return nil, err
	}

	resp, err := t.transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	out, err := httputil.DumpResponse(resp, true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to dump response")
	}

	if _, err := t.writer.Write(out); err != nil {
		return nil, err
	}

	return resp, err
}

// NewDebugClientTrace returns a Go http trace client predefined to write DNS and connection
// information to the log. This is used via the --http-trace flag on push and pull operations in ctr.
func NewDebugClientTrace(ctx gocontext.Context) *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		DNSStart: func(dnsInfo httptrace.DNSStartInfo) {
			log.G(ctx).WithField("host", dnsInfo.Host).Debugf("DNS lookup")
		},
		DNSDone: func(dnsInfo httptrace.DNSDoneInfo) {
			if dnsInfo.Err != nil {
				log.G(ctx).WithField("lookup_err", dnsInfo.Err).Debugf("DNS lookup error")
			} else {
				log.G(ctx).WithField("result", dnsInfo.Addrs[0].String()).WithField("coalesced", dnsInfo.Coalesced).Debugf("DNS lookup complete")
			}
		},
		GotConn: func(connInfo httptrace.GotConnInfo) {
			log.G(ctx).WithField("reused", connInfo.Reused).WithField("remote_addr", connInfo.Conn.RemoteAddr().String()).Debugf("Connection successful")
		},
	}
}
