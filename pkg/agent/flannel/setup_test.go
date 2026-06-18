package flannel

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flannel-io/flannel/pkg/backend"
	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/pkg/lease"
	"github.com/k3s-io/k3s/pkg/daemons/config"
)

// mockNetwork implements backend.Network for testing WriteSubnetFile.
type mockNetwork struct {
	l   *lease.Lease
	mtu int
}

func (m *mockNetwork) Lease() *lease.Lease   { return m.l }
func (m *mockNetwork) MTU() int              { return m.mtu }
func (m *mockNetwork) Run(_ context.Context) {}

// newMockNetwork builds a mockNetwork from string CIDRs.
// ipv4CIDR and ipv6CIDR may be empty to omit those fields.
func newMockNetwork(ipv4CIDR, ipv6CIDR string, mtu int) backend.Network {
	l := &lease.Lease{}
	if ipv4CIDR != "" {
		_, n, _ := net.ParseCIDR(ipv4CIDR)
		l.Subnet = ip.FromIPNet(n)
		l.EnableIPv4 = true
	}
	if ipv6CIDR != "" {
		_, n, _ := net.ParseCIDR(ipv6CIDR)
		l.IPv6Subnet = ip.FromIP6Net(n)
		l.EnableIPv6 = true
	}
	return &mockNetwork{l: l, mtu: mtu}
}

func stringToCIDR(s string) []*net.IPNet {
	var netCidrs []*net.IPNet
	for _, v := range strings.Split(s, ",") {
		_, parsed, _ := net.ParseCIDR(v)
		netCidrs = append(netCidrs, parsed)
	}
	return netCidrs
}

func Test_UnitcreateCNIConf(t *testing.T) {
	tests := []struct {
		name        string
		dir         string // empty string means use t.TempDir()
		nodeConfig  func(dir string) *config.Node
		wantErr     bool
		wantContent string // substring that must appear in the written file; empty means no content check
		wantAbsent  bool   // when true, the output file must NOT exist after the call
	}{
		{
			name: "empty dir is a no-op",
			dir:  "",
			nodeConfig: func(dir string) *config.Node {
				return &config.Node{}
			},
			wantAbsent: true,
		},
		{
			name: "default cni conf is written",
			nodeConfig: func(dir string) *config.Node {
				return &config.Node{
					AgentConfig: config.Agent{CNIConfDir: dir},
				}
			},
			wantContent: `"name":"cbr0"`,
		},
		{
			name: "flannel plugin is included in default conf",
			nodeConfig: func(dir string) *config.Node {
				return &config.Node{
					AgentConfig: config.Agent{CNIConfDir: dir},
				}
			},
			wantContent: `"type":"flannel"`,
		},
		{
			name: "portmap plugin is included in default conf",
			nodeConfig: func(dir string) *config.Node {
				return &config.Node{
					AgentConfig: config.Agent{CNIConfDir: dir},
				}
			},
			wantContent: `"type":"portmap"`,
		},
		{
			name: "custom CNIConfFile is copied to destination",
			nodeConfig: func(dir string) *config.Node {
				src := filepath.Join(dir, "custom.conflist")
				os.WriteFile(src, []byte(`{"custom":true}`), 0644)
				return &config.Node{
					AgentConfig: config.Agent{CNIConfDir: dir},
					Flannel:     config.Flannel{CNIConfFile: src},
				}
			},
			wantContent: `{"custom":true}`,
		},
		{
			name: "missing CNIConfFile returns error",
			nodeConfig: func(dir string) *config.Node {
				return &config.Node{
					AgentConfig: config.Agent{CNIConfDir: dir},
					Flannel:     config.Flannel{CNIConfFile: filepath.Join(dir, "nonexistent.conflist")},
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.dir
			if dir == "" && !tt.wantAbsent {
				dir = t.TempDir()
			}

			cfg := tt.nodeConfig(dir)
			err := createCNIConf(dir, cfg)

			if (err != nil) != tt.wantErr {
				t.Fatalf("createCNIConf() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			outPath := filepath.Join(dir, "10-flannel.conflist")

			if tt.wantAbsent {
				if _, statErr := os.Stat(outPath); statErr == nil {
					t.Errorf("expected no output file, but %s exists", outPath)
				}
				return
			}

			data, err := os.ReadFile(outPath)
			if err != nil {
				t.Fatalf("could not read output file %s: %v", outPath, err)
			}

			if tt.wantContent != "" && !strings.Contains(string(data), tt.wantContent) {
				t.Errorf("output file does not contain %q\ngot:\n%s", tt.wantContent, string(data))
			}
		})
	}
}

func Test_findNetMode(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		wantv4  bool
		wantv6  bool
		wantErr bool
	}{
		{"dual-stack", "10.42.0.0/16,2001:cafe:22::/56", true, true, false},
		{"dual-stack ipv6 first", "2001:cafe:22::/56,10.42.0.0/16", true, true, false},
		{"ipv4 only", "10.42.0.0/16", true, false, false},
		{"ipv6 only", "2001:cafe:42:0::/56", false, true, false},
		{"empty", "", false, false, true},
		{"wrong input", "wrong", false, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			netCidrs := stringToCIDR(tt.args)
			got, err := findNetMode(netCidrs)
			if (err != nil) != tt.wantErr {
				t.Fatalf("got error %v, want %v", err, tt.wantErr)
			}
			if gotv4 := got.IPv4Enabled(); gotv4 != tt.wantv4 {
				t.Errorf("got ipv4 %v, want %v", gotv4, tt.wantv4)
			}
			if gotv6 := got.IPv6Enabled(); gotv6 != tt.wantv6 {
				t.Errorf("got ipv6 %v, want %v", gotv6, tt.wantv6)
			}
		})
	}
}

func Test_UnitcreateFlannelConf(t *testing.T) {
	tests := []struct {
		name        string
		nodeConfig  func(confFile string) *config.Node
		wantErr     bool
		wantContain []string
		wantAbsent  []string
	}{
		{
			name: "dual-stack",
			nodeConfig: func(confFile string) *config.Node {
				cidrs := stringToCIDR("10.42.0.0/16,2001:cafe:22::/56")
				return &config.Node{
					Flannel:     config.Flannel{Backend: BackendVXLAN, ConfFile: confFile},
					AgentConfig: config.Agent{ClusterCIDR: cidrs[0], ClusterCIDRs: cidrs},
				}
			},
			wantContain: []string{
				`"Network": "10.42.0.0/16"`,
				`"IPv6Network": "2001:cafe:22::/56"`,
				`"EnableIPv6": true`,
				`"EnableIPv4": true`,
			},
		},
		{
			name: "ipv4 only",
			nodeConfig: func(confFile string) *config.Node {
				cidrs := stringToCIDR("10.42.0.0/16")
				return &config.Node{
					Flannel:     config.Flannel{Backend: BackendVXLAN, ConfFile: confFile},
					AgentConfig: config.Agent{ClusterCIDR: cidrs[0], ClusterCIDRs: cidrs},
				}
			},
			wantContain: []string{
				`"Network": "10.42.0.0/16"`,
				`"IPv6Network": "::/0"`,
				`"EnableIPv6": false`,
				`"EnableIPv4": true`,
			},
		},
		{
			name: "empty ConfFile returns error",
			nodeConfig: func(_ string) *config.Node {
				cidrs := stringToCIDR("10.42.0.0/16")
				return &config.Node{
					Flannel:     config.Flannel{Backend: BackendVXLAN, ConfFile: ""},
					AgentConfig: config.Agent{ClusterCIDR: cidrs[0], ClusterCIDRs: cidrs},
				}
			},
			wantErr: true,
		},
		{
			name: "ConfOverride skips writing and returns nil, no file is written",
			nodeConfig: func(confFile string) *config.Node {
				cidrs := stringToCIDR("10.42.0.0/16")
				return &config.Node{
					Flannel:     config.Flannel{Backend: BackendVXLAN, ConfFile: confFile, ConfOverride: true},
					AgentConfig: config.Agent{ClusterCIDR: cidrs[0], ClusterCIDRs: cidrs},
				}
			},
		},
		{
			name: "ipv6 only",
			nodeConfig: func(confFile string) *config.Node {
				cidrs := stringToCIDR("2001:cafe:42::/56")
				return &config.Node{
					Flannel:     config.Flannel{Backend: BackendVXLAN, ConfFile: confFile},
					AgentConfig: config.Agent{ClusterCIDR: cidrs[0], ClusterCIDRs: cidrs},
				}
			},
			wantContain: []string{
				`"Network": "0.0.0.0/0"`,
				`"IPv6Network": "2001:cafe:42::/56"`,
				`"EnableIPv6": true`,
				`"EnableIPv4": false`,
			},
		},
		{
			name: "host-gw backend",
			nodeConfig: func(confFile string) *config.Node {
				cidrs := stringToCIDR("10.42.0.0/16")
				return &config.Node{
					Flannel:     config.Flannel{Backend: BackendHostGW, ConfFile: confFile},
					AgentConfig: config.Agent{ClusterCIDR: cidrs[0], ClusterCIDRs: cidrs},
				}
			},
			wantContain: []string{`"Type": "host-gw"`},
		},
		{
			name: "wireguard-native backend",
			nodeConfig: func(confFile string) *config.Node {
				cidrs := stringToCIDR("10.42.0.0/16")
				return &config.Node{
					Flannel:     config.Flannel{Backend: BackendWireguardNative, ConfFile: confFile},
					AgentConfig: config.Agent{ClusterCIDR: cidrs[0], ClusterCIDRs: cidrs},
				}
			},
			wantContain: []string{`"Type": "wireguard"`},
		},
		{
			name: "tailscale backend ipv4 includes $SUBNET route",
			nodeConfig: func(confFile string) *config.Node {
				cidrs := stringToCIDR("10.42.0.0/16")
				return &config.Node{
					Flannel:     config.Flannel{Backend: BackendTailscale, ConfFile: confFile},
					AgentConfig: config.Agent{ClusterCIDR: cidrs[0], ClusterCIDRs: cidrs},
				}
			},
			wantContain: []string{`"Type": "extension"`, `$SUBNET`},
			wantAbsent:  []string{`$IPV6SUBNET`},
		},
		{
			name: "tailscale backend dual-stack includes both routes",
			nodeConfig: func(confFile string) *config.Node {
				cidrs := stringToCIDR("10.42.0.0/16,2001:cafe:22::/56")
				return &config.Node{
					Flannel:     config.Flannel{Backend: BackendTailscale, ConfFile: confFile},
					AgentConfig: config.Agent{ClusterCIDR: cidrs[0], ClusterCIDRs: cidrs},
				}
			},
			wantContain: []string{`$SUBNET`, `$IPV6SUBNET`},
		},
		{
			name: "unknown backend returns error",
			nodeConfig: func(confFile string) *config.Node {
				cidrs := stringToCIDR("10.42.0.0/16")
				return &config.Node{
					Flannel:     config.Flannel{Backend: "unknown-backend", ConfFile: confFile},
					AgentConfig: config.Agent{ClusterCIDR: cidrs[0], ClusterCIDRs: cidrs},
				}
			},
			wantErr: true,
		},
		{
			name: "dual-stack ipv6 first picks correct ipv4 CIDR",
			nodeConfig: func(confFile string) *config.Node {
				cidrs := stringToCIDR("2001:cafe:22::/56,10.42.0.0/16")
				return &config.Node{
					Flannel:     config.Flannel{Backend: BackendVXLAN, ConfFile: confFile},
					AgentConfig: config.Agent{ClusterCIDR: cidrs[1], ClusterCIDRs: cidrs},
				}
			},
			wantContain: []string{
				`"Network": "10.42.0.0/16"`,
				`"IPv6Network": "2001:cafe:22::/56"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confFile := filepath.Join(t.TempDir(), "flannel.conf")
			cfg := tt.nodeConfig(confFile)

			err := createFlannelConf(cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("createFlannelConf() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			// ConfOverride: file must not have been written
			if cfg.Flannel.ConfOverride {
				if _, statErr := os.Stat(confFile); statErr == nil {
					t.Errorf("expected no file to be written when ConfOverride=true, but %s exists", confFile)
				}
				return
			}

			data, err := os.ReadFile(confFile)
			if err != nil {
				t.Fatalf("could not read conf file %s: %v", confFile, err)
			}
			content := string(data)

			for _, want := range tt.wantContain {
				if !strings.Contains(content, want) {
					t.Errorf("conf does not contain %q\ngot:\n%s", want, content)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(content, absent) {
					t.Errorf("conf should not contain %q\ngot:\n%s", absent, content)
				}
			}
		})
	}
}

func Test_UnitWriteSubnetFile(t *testing.T) {
	tests := []struct {
		name        string
		ipv4Net     string
		ipv6Net     string
		ipMasq      bool
		mtu         int
		nm          netMode
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:    "ipv4 only writes FLANNEL_NETWORK and FLANNEL_SUBNET",
			ipv4Net: "10.42.0.0/16",
			ipv6Net: "",
			ipMasq:  true,
			mtu:     1500,
			nm:      ipv4,
			wantContain: []string{
				"FLANNEL_NETWORK=10.42.0.0/16",
				"FLANNEL_SUBNET=",
				"FLANNEL_MTU=1500",
				"FLANNEL_IPMASQ=true",
			},
			wantAbsent: []string{"FLANNEL_IPV6_NETWORK", "FLANNEL_IPV6_SUBNET"},
		},
		{
			name:    "ipv6 only writes FLANNEL_IPV6 keys only",
			ipv4Net: "",
			ipv6Net: "2001:cafe:22::/56",
			ipMasq:  false,
			mtu:     1400,
			nm:      ipv6,
			wantContain: []string{
				"FLANNEL_IPV6_NETWORK=2001:cafe:22::/56",
				"FLANNEL_IPV6_SUBNET=",
				"FLANNEL_MTU=1400",
				"FLANNEL_IPMASQ=false",
			},
			wantAbsent: []string{"FLANNEL_NETWORK=", "FLANNEL_SUBNET="},
		},
		{
			name:    "dual-stack writes all four CIDR keys",
			ipv4Net: "10.42.0.0/16",
			ipv6Net: "2001:cafe:22::/56",
			ipMasq:  true,
			mtu:     1450,
			nm:      ipv4 | ipv6,
			wantContain: []string{
				"FLANNEL_NETWORK=10.42.0.0/16",
				"FLANNEL_SUBNET=",
				"FLANNEL_IPV6_NETWORK=2001:cafe:22::/56",
				"FLANNEL_IPV6_SUBNET=",
				"FLANNEL_MTU=1450",
				"FLANNEL_IPMASQ=true",
			},
		},
		{
			name:        "ipmasq false is written correctly",
			ipv4Net:     "10.42.0.0/16",
			ipv6Net:     "",
			ipMasq:      false,
			mtu:         1500,
			nm:          ipv4,
			wantContain: []string{"FLANNEL_IPMASQ=false"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "subnet.env")
			bn := newMockNetwork(tt.ipv4Net, tt.ipv6Net, tt.mtu)

			var nw ip.IP4Net
			if tt.ipv4Net != "" {
				_, n, _ := net.ParseCIDR(tt.ipv4Net)
				nw = ip.FromIPNet(n)
			}
			var nwv6 ip.IP6Net
			if tt.ipv6Net != "" {
				_, n, _ := net.ParseCIDR(tt.ipv6Net)
				nwv6 = ip.FromIP6Net(n)
			}

			if err := WriteSubnetFile(path, nw, nwv6, tt.ipMasq, bn, tt.nm); err != nil {
				t.Fatalf("WriteSubnetFile() error = %v", err)
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("could not read subnet file: %v", err)
			}
			content := string(data)

			for _, want := range tt.wantContain {
				if !strings.Contains(content, want) {
					t.Errorf("subnet file missing %q\ngot:\n%s", want, content)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(content, absent) {
					t.Errorf("subnet file should not contain %q\ngot:\n%s", absent, content)
				}
			}
		})
	}
}

func Test_UnitReadCIDRFromSubnetFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		key     string
		want    string // expected String() of the returned IP4Net; "0.0.0.0/0" means zero value
	}{
		{
			name:    "reads FLANNEL_NETWORK",
			content: "FLANNEL_NETWORK=10.42.0.0/16\nFLANNEL_MTU=1500\n",
			key:     "FLANNEL_NETWORK",
			want:    "10.42.0.0/16",
		},
		{
			name:    "reads FLANNEL_SUBNET",
			content: "FLANNEL_SUBNET=10.42.1.1/24\n",
			key:     "FLANNEL_SUBNET",
			want:    "10.42.1.0/24",
		},
		{
			name:    "missing key returns zero value",
			content: "FLANNEL_MTU=1500\n",
			key:     "FLANNEL_NETWORK",
			want:    "0.0.0.0/0",
		},
		{
			name:    "non-existent file returns zero value",
			content: "", // signals: don't create the file
			key:     "FLANNEL_NETWORK",
			want:    "0.0.0.0/0",
		},
		{
			name:    "multiple CIDRs for key returns zero value",
			content: "FLANNEL_NETWORK=10.42.0.0/16,10.43.0.0/16\n",
			key:     "FLANNEL_NETWORK",
			want:    "0.0.0.0/0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "subnet.env")
			if tt.content != "" {
				os.WriteFile(path, []byte(tt.content), 0644)
			}

			got := ReadCIDRFromSubnetFile(path, tt.key)
			if got.String() != tt.want {
				t.Errorf("ReadCIDRFromSubnetFile() = %s, want %s", got, tt.want)
			}
		})
	}
}

func Test_UnitReadIP6CIDRFromSubnetFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		key     string
		want    string // expected String() of the returned IP6Net; "::/0" means zero value
	}{
		{
			name:    "reads FLANNEL_IPV6_NETWORK",
			content: "FLANNEL_IPV6_NETWORK=2001:cafe:22::/56\nFLANNEL_MTU=1500\n",
			key:     "FLANNEL_IPV6_NETWORK",
			want:    "2001:cafe:22::/56",
		},
		{
			name:    "reads FLANNEL_IPV6_SUBNET",
			content: "FLANNEL_IPV6_SUBNET=2001:cafe:22::1/56\n",
			key:     "FLANNEL_IPV6_SUBNET",
			want:    "2001:cafe:22::/56",
		},
		{
			name:    "missing key returns zero value",
			content: "FLANNEL_MTU=1500\n",
			key:     "FLANNEL_IPV6_NETWORK",
			want:    "::/0",
		},
		{
			name:    "non-existent file returns zero value",
			content: "",
			key:     "FLANNEL_IPV6_NETWORK",
			want:    "::/0",
		},
		{
			name:    "multiple CIDRs for key returns zero value",
			content: "FLANNEL_IPV6_NETWORK=2001:cafe:22::/56,2001:cafe:33::/56\n",
			key:     "FLANNEL_IPV6_NETWORK",
			want:    "::/0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "subnet.env")
			if tt.content != "" {
				os.WriteFile(path, []byte(tt.content), 0644)
			}

			got := ReadIP6CIDRFromSubnetFile(path, tt.key)
			if got.String() != tt.want {
				t.Errorf("ReadIP6CIDRFromSubnetFile() = %s, want %s", got, tt.want)
			}
		})
	}
}

func Test_UnitWriteAndReadSubnetFileRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		ipv4Net string
		ipv6Net string
		mtu     int
		nm      netMode
	}{
		{"ipv4 only", "10.42.0.0/16", "", 1500, ipv4},
		{"ipv6 only", "", "2001:cafe:22::/56", 1400, ipv6},
		{"dual-stack", "10.42.0.0/16", "2001:cafe:22::/56", 1450, ipv4 | ipv6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "subnet.env")
			bn := newMockNetwork(tt.ipv4Net, tt.ipv6Net, tt.mtu)

			var nw ip.IP4Net
			if tt.ipv4Net != "" {
				_, n, _ := net.ParseCIDR(tt.ipv4Net)
				nw = ip.FromIPNet(n)
			}
			var nwv6 ip.IP6Net
			if tt.ipv6Net != "" {
				_, n, _ := net.ParseCIDR(tt.ipv6Net)
				nwv6 = ip.FromIP6Net(n)
			}

			if err := WriteSubnetFile(path, nw, nwv6, true, bn, tt.nm); err != nil {
				t.Fatalf("WriteSubnetFile() error = %v", err)
			}

			if tt.ipv4Net != "" {
				gotNet := ReadCIDRFromSubnetFile(path, "FLANNEL_NETWORK")
				if gotNet.String() != tt.ipv4Net {
					t.Errorf("FLANNEL_NETWORK: got %s, want %s", gotNet, tt.ipv4Net)
				}
				gotSubnet := ReadCIDRsFromSubnetFile(path, "FLANNEL_SUBNET")
				if len(gotSubnet) == 0 {
					t.Errorf("FLANNEL_SUBNET: got empty, want a subnet of %s", tt.ipv4Net)
				}
			}

			if tt.ipv6Net != "" {
				_, wantNet, _ := net.ParseCIDR(tt.ipv6Net)
				gotNet := ReadIP6CIDRFromSubnetFile(path, "FLANNEL_IPV6_NETWORK")
				if gotNet.String() != wantNet.String() {
					t.Errorf("FLANNEL_IPV6_NETWORK: got %s, want %s", gotNet, wantNet)
				}
				gotSubnet := ReadIP6CIDRsFromSubnetFile(path, "FLANNEL_IPV6_SUBNET")
				if len(gotSubnet) == 0 {
					t.Errorf("FLANNEL_IPV6_SUBNET: got empty, want a subnet of %s", tt.ipv6Net)
				}
			}
		})
	}
}
