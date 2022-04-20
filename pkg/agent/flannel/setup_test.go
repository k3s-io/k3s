package flannel

import (
	"io/ioutil"
	"net"
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
		want    int
		wantErr bool
	}{
		{"dual-stack", "10.42.0.0/16,2001:cafe:22::/56", ipv4 + ipv6, false},
		{"ipv4 only", "10.42.0.0/16", ipv4, false},
		{"ipv6 only", "2001:cafe:42:0::/56", ipv6, false},
		{"wrong input", "wrong", 0, true},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			netCidrs := stringToCIDR(tt.args)
			got, err := findNetMode(netCidrs)
			if (err != nil) != tt.wantErr {
				t.Errorf("findNetMode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("findNetMode() = %v, want %v", got, tt.want)
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
	var containerd = config.Containerd{}
	for _, tt := range tests {
		var agent = config.Agent{}
		agent.ClusterCIDR = stringToCIDR(tt.args)[0]
		agent.ClusterCIDRs = stringToCIDR(tt.args)
		var nodeConfig = &config.Node{ContainerRuntimeEndpoint: "", NoFlannel: false, SELinux: false, FlannelBackend: "vxlan", FlannelConfFile: "test_file", FlannelConfOverride: false, FlannelIface: nil, Containerd: containerd, Images: "", AgentConfig: agent, Token: "", Certificate: nil, ServerHTTPSPort: 0}

		t.Run(tt.name, func(t *testing.T) {
			if err := createFlannelConf(nodeConfig); (err != nil) != tt.wantErr {
				t.Errorf("createFlannelConf() error = %v, wantErr %v", err, tt.wantErr)
			}
			data, err := ioutil.ReadFile("test_file")
			if err != nil {
				t.Errorf("Something went wrong when reading the flannel config file")
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
