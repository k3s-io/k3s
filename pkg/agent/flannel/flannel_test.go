package flannel

import (
	"io/ioutil"
	"net"
	"regexp"
	"strings"
	"testing"

	"github.com/rancher/k3s/pkg/daemons/config"
)

func stringToCIDR (s string) []*net.IPNet {
	var netCidrs []*net.IPNet
	for _, v := range strings.Split(s, ",") {
		_, parsed, _ := net.ParseCIDR(v)
                netCidrs = append(netCidrs, parsed)
	}
	return netCidrs
}

func Test_UnitfindNetMode(t *testing.T) {
	var dualStackTests = []struct {
		args string
		want int
	}{
		{"10.42.0.0/16,2001:cafe:22::/56", ipv4+ipv6},
		{"10.42.0.0/16", ipv4},
		{"2001:cafe:42:0::/56", ipv6},
		{"wrong", 0},
	}
        for _, tt := range dualStackTests {
                t.Run("dualStackUnitTest", func(t *testing.T) {
			netCidrs := stringToCIDR(tt.args)
                        if got, _ := findNetMode(netCidrs); got != tt.want {
                                t.Errorf("findNetMode() = %v, want %v", got, tt.want)
                        }
                })
        }
}

func Test_UnitcreateFlannelConf(t *testing.T) {
	var flannelConfTests = []struct {
		args string
		want_error error
		want_config []string
	}{
                {"10.42.0.0/16,2001:cafe:22::/56", nil, []string{"\"Network\": \"10.42.0.0/16\"", "\"IPv6Network\": \"2001:cafe:22::/56\"", "\"EnableIPv6\": true"}},
                {"10.42.0.0/16", nil, []string{"\"Network\": \"10.42.0.0/16\"", "\"IPv6Network\": \"::/0\"","\"EnableIPv6\": false"}},
	}

	var containerd = config.Containerd{}

	for _, tt := range flannelConfTests {
		var agent = config.Agent{}
		agent.ClusterCIDR = stringToCIDR(tt.args)[0]
		agent.ClusterCIDRs = stringToCIDR(tt.args)
		var nodeConfig = &config.Node{false,"",false,false,"vxlan","test_file",false,nil,containerd,"",agent,"",nil,0}

		t.Run("createFlannelUnitTest", func(t *testing.T) {
                if got := createFlannelConf(nodeConfig); got != tt.want_error {
                        t.Errorf("findNetMode() = %v, want %v", got, tt.want_error)
		}
		data, err := ioutil.ReadFile("test_file")
		if err != nil {
			t.Errorf("Something went wrong when reading the flannel config file")
		}
		for _, config := range tt.want_config {
			isExist, _ := regexp.Match(config, data)
			if !isExist {
				t.Errorf("Config is wrong, %s is not present", config)
			}
		}
	})
	}
}
