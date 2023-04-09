//go:build windows && !no_embedded_executor
// +build windows,!no_embedded_executor

package executor

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Microsoft/hcsshim"
	"github.com/sirupsen/logrus"

	// registering k3s cloud provider
	_ "github.com/k3s-io/k3s/pkg/cloudprovider"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
)

const (
	networkName = "flannel.4096"
)

type SourceVipResponse struct {
	IP4 struct {
		IP string `json:"ip"`
	} `json:"ip4"`
}

func platformKubeProxyArgs(nodeConfig *daemonconfig.Node) map[string]string {
	argsMap := map[string]string{}
	argsMap["network-name"] = networkName
	if sourceVip := waitForSourceVip(networkName, nodeConfig); sourceVip != "" {
		argsMap["source-vip"] = sourceVip
	}
	return argsMap
}

func waitForSourceVip(networkName string, nodeConfig *daemonconfig.Node) string {
	for range time.Tick(time.Second * 5) {
		network, err := hcsshim.GetHNSNetworkByName(networkName)
		if err != nil {
			logrus.WithError(err).Warningf("can't find HNS network, retrying %s", networkName)
			continue
		}
		if network.ManagementIP == "" {
			logrus.WithError(err).Warningf("wait for management IP, retrying %s", networkName)
			continue
		}

		subnet := network.Subnets[0].AddressPrefix

		configData := `{
			"cniVersion": "0.2.0",
			"name": "vxlan0",
			"ipam": {
				"type": "host-local",
				"ranges": [[{"subnet":"` + subnet + `"}]],
				"dataDir": "/var/lib/cni/networks"
			}
		}`

		cmd := exec.Command("host-local.exe")
		cmd.Env = append(os.Environ(),
			"CNI_COMMAND=ADD",
			"CNI_CONTAINERID=dummy",
			"CNI_NETNS=dummy",
			"CNI_IFNAME=dummy",
			"CNI_PATH="+nodeConfig.AgentConfig.CNIBinDir,
		)

		cmd.Stdin = strings.NewReader(configData)
		out, err := cmd.Output()
		if err != nil {
			logrus.WithError(err).Warning("Failed to execute host-local.exe")
			continue
		}

		var sourceVipResp SourceVipResponse
		err = json.Unmarshal(out, &sourceVipResp)
		if err != nil {
			logrus.WithError(err).Warning("Failed to unmarshal sourceVip response")
			continue
		}

		return strings.TrimSpace(strings.Split(sourceVipResp.IP4.IP, "/")[0])
	}
	return ""
}
