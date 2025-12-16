package flannel

import (
	"net"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/pkg/daemons/config"
)

func stringToCIDR(s string) []*net.IPNet {
	var netCidrs []*net.IPNet
	for _, v := range strings.Split(s, ",") {
		_, parsed, _ := net.ParseCIDR(v)
		netCidrs = append(netCidrs, parsed)
	}
	return netCidrs
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

func Test_createFlannelConf(t *testing.T) {
	tests := []struct {
		name       string
		args       string
		wantConfig []string
		wantErr    bool
	}{
		{"dual-stack", "10.42.0.0/16,2001:cafe:22::/56", []string{"\"Network\": \"10.42.0.0/16\"", "\"IPv6Network\": \"2001:cafe:22::/56\"", "\"EnableIPv6\": true"}, false},
		{"ipv4 only", "10.42.0.0/16", []string{"\"Network\": \"10.42.0.0/16\"", "\"IPv6Network\": \"::/0\"", "\"EnableIPv6\": false"}, false},
	}
	for _, tt := range tests {
		var nodeConfig = &config.Node{
			Flannel: config.Flannel{
				Backend:  "vxlan",
				ConfFile: "test_file",
			},
			AgentConfig: config.Agent{
				ClusterCIDR:  stringToCIDR(tt.args)[0],
				ClusterCIDRs: stringToCIDR(tt.args),
			},
		}

		t.Run(tt.name, func(t *testing.T) {
			if err := createFlannelConf(nodeConfig); (err != nil) != tt.wantErr {
				t.Errorf("createFlannelConf() error = %v, wantErr %v", err, tt.wantErr)
			}
			data, err := os.ReadFile("test_file")
			if err != nil {
				t.Error("Something went wrong when reading the flannel config file")
			}
			for _, config := range tt.wantConfig {
				isExist, _ := regexp.Match(config, data)
				if !isExist {
					t.Errorf("Config is wrong, %s is not present", config)
				}
			}
		})
	}
}
