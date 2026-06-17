package etcd

import (
	"testing"

	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
)

type fakeClientConn struct {
	state       resolver.State
	updateCount int
	errCount    int
}

func (f *fakeClientConn) UpdateState(s resolver.State) error {
	f.state = s
	f.updateCount++
	return nil
}

func (f *fakeClientConn) ReportError(error) {
	f.errCount++
}

func (f *fakeClientConn) NewAddress([]resolver.Address) {}

func (f *fakeClientConn) ParseServiceConfig(string) *serviceconfig.ParseResult {
	return nil
}

func Test_UnitInterpret(t *testing.T) {
	tests := []struct {
		name           string
		endpoint       string
		wantAddress    string
		wantServerName string
	}{
		{
			name:           "unix absolute path",
			endpoint:       "unix:///var/lib/rancher/k3s/server/db/etcd.sock",
			wantAddress:    "unix:///var/lib/rancher/k3s/server/db/etcd.sock",
			wantServerName: "etcd.sock",
		},
		{
			name:           "unix local path with double slash",
			endpoint:       "unix://var/lib/rancher/k3s/server/db/etcd.sock",
			wantAddress:    "unix:var/lib/rancher/k3s/server/db/etcd.sock",
			wantServerName: "etcd.sock",
		},
		{
			name:           "unix local path single colon",
			endpoint:       "unix:var/lib/rancher/k3s/server/db/etcd.sock",
			wantAddress:    "unix:var/lib/rancher/k3s/server/db/etcd.sock",
			wantServerName: "etcd.sock",
		},
		{
			name:           "https endpoint returns host",
			endpoint:       "https://127.0.0.1:2379",
			wantAddress:    "127.0.0.1:2379",
			wantServerName: "127.0.0.1:2379",
		},
		{
			name:           "http endpoint returns host",
			endpoint:       "http://127.0.0.1:2379",
			wantAddress:    "127.0.0.1:2379",
			wantServerName: "127.0.0.1:2379",
		},
		{
			name:           "non-http scheme preserves endpoint address",
			endpoint:       "tcp://127.0.0.1:2379",
			wantAddress:    "tcp://127.0.0.1:2379",
			wantServerName: "127.0.0.1:2379",
		},
		{
			name:           "malformed scheme endpoint returns endpoint for both",
			endpoint:       "://bad-endpoint",
			wantAddress:    "://bad-endpoint",
			wantServerName: "://bad-endpoint",
		},
		{
			name:           "plain hostport endpoint",
			endpoint:       "127.0.0.1:2379",
			wantAddress:    "127.0.0.1:2379",
			wantServerName: "127.0.0.1:2379",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAddress, gotServerName := interpret(tt.endpoint)
			if gotAddress != tt.wantAddress {
				t.Errorf("interpret() address = %q, want %q", gotAddress, tt.wantAddress)
			}
			if gotServerName != tt.wantServerName {
				t.Errorf("interpret() serverName = %q, want %q", gotServerName, tt.wantServerName)
			}
		})
	}
}

func Test_UnitAuthority(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
	}{
		{
			name:     "scheme authority form",
			endpoint: "https://127.0.0.1:2379",
			want:     "127.0.0.1:2379",
		},
		{
			name:     "unix prefix",
			endpoint: "unix:/var/lib/rancher/k3s/server/db/etcd.sock",
			want:     "/var/lib/rancher/k3s/server/db/etcd.sock",
		},
		{
			name:     "unixs prefix",
			endpoint: "unixs:/var/lib/rancher/k3s/server/db/etcd.sock",
			want:     "/var/lib/rancher/k3s/server/db/etcd.sock",
		},
		{
			name:     "plain endpoint",
			endpoint: "127.0.0.1:2379",
			want:     "127.0.0.1:2379",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := authority(tt.endpoint); got != tt.want {
				t.Errorf("authority() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_UnitSimpleResolver_Build(t *testing.T) {
	tests := []struct {
		name             string
		endpoint         string
		withClientConn   bool
		wantUpdateCount  int
		wantEndpointAddr string
		wantServerName   string
	}{
		{
			name:             "with client conn updates resolver state",
			endpoint:         "https://127.0.0.1:2379",
			withClientConn:   true,
			wantUpdateCount:  1,
			wantEndpointAddr: "127.0.0.1:2379",
			wantServerName:   "127.0.0.1:2379",
		},
		{
			name:            "without client conn does not update state",
			endpoint:        "https://127.0.0.1:2379",
			withClientConn:  false,
			wantUpdateCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewSimpleResolver(tt.endpoint)

			var cc *fakeClientConn
			var ccArg resolver.ClientConn
			if tt.withClientConn {
				cc = &fakeClientConn{}
				ccArg = cc
			}

			res, err := r.Build(resolver.Target{}, ccArg, resolver.BuildOptions{})
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}
			if res == nil {
				t.Fatalf("Build() returned nil resolver")
			}

			if tt.withClientConn {
				if cc.updateCount != tt.wantUpdateCount {
					t.Fatalf("Build() updateCount = %d, want %d", cc.updateCount, tt.wantUpdateCount)
				}
				if len(cc.state.Endpoints) != 1 || len(cc.state.Endpoints[0].Addresses) != 1 {
					t.Fatalf("Build() updated unexpected state: %+v", cc.state)
				}
				gotAddr := cc.state.Endpoints[0].Addresses[0]
				if gotAddr.Addr != tt.wantEndpointAddr {
					t.Errorf("Build() address = %q, want %q", gotAddr.Addr, tt.wantEndpointAddr)
				}
				if gotAddr.ServerName != tt.wantServerName {
					t.Errorf("Build() serverName = %q, want %q", gotAddr.ServerName, tt.wantServerName)
				}
			}
		})
	}
}
