package loadbalancer

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/proxy"
)

var originalDialer proxy.Dialer
var defaultEnv map[string]string
var proxyEnvs = []string{version.ProgramUpper + "_AGENT_HTTP_PROXY_ALLOWED", "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy"}

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

func prepareEnv(env ...string) {
	originalDialer = defaultDialer
	defaultEnv = map[string]string{}
	for _, e := range proxyEnvs {
		if v, ok := os.LookupEnv(e); ok {
			defaultEnv[e] = v
			os.Unsetenv(e)
		}
	}
	for _, e := range env {
		k, v, _ := strings.Cut(e, "=")
		os.Setenv(k, v)
	}
}

func restoreEnv() {
	defaultDialer = originalDialer
	for _, e := range proxyEnvs {
		if v, ok := defaultEnv[e]; ok {
			os.Setenv(e, v)
		} else {
			os.Unsetenv(e)
		}
	}
}

func Test_UnitSetHTTPProxy(t *testing.T) {
	type args struct {
		address string
	}
	tests := []struct {
		name       string
		args       args
		setup      func() error
		teardown   func() error
		wantErr    bool
		wantDialer string
	}{
		{
			name:       "Default Proxy",
			args:       args{address: "https://1.2.3.4:6443"},
			wantDialer: "*net.Dialer",
			setup: func() error {
				prepareEnv(version.ProgramUpper+"_AGENT_HTTP_PROXY_ALLOWED=", "HTTP_PROXY=", "HTTPS_PROXY=", "NO_PROXY=")
				return nil
			},
			teardown: func() error {
				restoreEnv()
				return nil
			},
		},
		{
			name:       "Agent Proxy Enabled",
			args:       args{address: "https://1.2.3.4:6443"},
			wantDialer: "*http_dialer.HttpTunnel",
			setup: func() error {
				prepareEnv(version.ProgramUpper+"_AGENT_HTTP_PROXY_ALLOWED=true", "HTTP_PROXY=http://proxy:8080", "HTTPS_PROXY=http://proxy:8080", "NO_PROXY=")
				return nil
			},
			teardown: func() error {
				restoreEnv()
				return nil
			},
		},
		{
			name:       "Agent Proxy Enabled with Bogus Proxy",
			args:       args{address: "https://1.2.3.4:6443"},
			wantDialer: "*net.Dialer",
			wantErr:    true,
			setup: func() error {
				prepareEnv(version.ProgramUpper+"_AGENT_HTTP_PROXY_ALLOWED=true", "HTTP_PROXY=proxy proxy", "HTTPS_PROXY=proxy proxy", "NO_PROXY=")
				return nil
			},
			teardown: func() error {
				restoreEnv()
				return nil
			},
		},
		{
			name:       "Agent Proxy Enabled with Bogus Server",
			args:       args{address: "https://1.2.3.4:k3s"},
			wantDialer: "*net.Dialer",
			wantErr:    true,
			setup: func() error {
				prepareEnv(version.ProgramUpper+"_AGENT_HTTP_PROXY_ALLOWED=true", "HTTP_PROXY=http://proxy:8080", "HTTPS_PROXY=http://proxy:8080", "NO_PROXY=")
				return nil
			},
			teardown: func() error {
				restoreEnv()
				return nil
			},
		},
		{
			name:       "Agent Proxy Enabled but IP Excluded",
			args:       args{address: "https://1.2.3.4:6443"},
			wantDialer: "*net.Dialer",
			setup: func() error {
				prepareEnv(version.ProgramUpper+"_AGENT_HTTP_PROXY_ALLOWED=true", "HTTP_PROXY=http://proxy:8080", "HTTPS_PROXY=http://proxy:8080", "NO_PROXY=1.2.0.0/16")
				return nil
			},
			teardown: func() error {
				restoreEnv()
				return nil
			},
		},
		{
			name:       "Agent Proxy Enabled but Domain Excluded",
			args:       args{address: "https://server.example.com:6443"},
			wantDialer: "*net.Dialer",
			setup: func() error {
				prepareEnv(version.ProgramUpper+"_AGENT_HTTP_PROXY_ALLOWED=true", "HTTP_PROXY=http://proxy:8080", "HTTPS_PROXY=http://proxy:8080", "NO_PROXY=*.example.com")
				return nil
			},
			teardown: func() error {
				restoreEnv()
				return nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer tt.teardown()
			if err := tt.setup(); err != nil {
				t.Errorf("Setup for SetHTTPProxy() failed = %v", err)
				return
			}
			err := SetHTTPProxy(tt.args.address)
			t.Logf("SetHTTPProxy() error = %v", err)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetHTTPProxy() error = %v, wantErr %v", err, tt.wantErr)
			}
			if dialerType := fmt.Sprintf("%T", defaultDialer); dialerType != tt.wantDialer {
				t.Errorf("Got wrong dialer type %s, wanted %s", dialerType, tt.wantDialer)
			}
		})
	}
}
