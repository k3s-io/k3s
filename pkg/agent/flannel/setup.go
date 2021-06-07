package flannel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rancher/k3s/pkg/agent/util"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	cniConf = `{
  "name":"cbr0",
  "cniVersion":"0.3.1",
  "plugins":[
    {
      "type":"flannel",
      "delegate":{
        "hairpinMode":true,
        "forceAddress":true,
        "isDefaultGateway":true
      }
    },
    {
      "type":"portmap",
      "capabilities":{
        "portMappings":true
      }
    }
  ]
}
`

	flannelConf = `{
	"Network": "%CIDR%",
	"Backend": %backend%
}
`

	vxlanBackend = `{
	"Type": "vxlan"
}`

	hostGWBackend = `{
	"Type": "host-gw"
}`

	ipsecBackend = `{
	"Type": "ipsec",
	"UDPEncap": true,
	"PSK": "%psk%"
}`

	wireguardBackend = `{
	"Type": "extension",
	"PreStartupCommand": "wg genkey | tee %flannelConfDir%/privatekey | wg pubkey",
	"PostStartupCommand": "export SUBNET_IP=$(echo $SUBNET | cut -d'/' -f 1); ip link del flannel.1 2>/dev/null; echo $PATH >&2; wg-add.sh flannel.1 && wg set flannel.1 listen-port 51820 private-key %flannelConfDir%/privatekey && ip addr add $SUBNET_IP/32 dev flannel.1 && ip link set flannel.1 up && ip route add $NETWORK dev flannel.1",
	"ShutdownCommand": "ip link del flannel.1",
	"SubnetAddCommand": "read PUBLICKEY; wg set flannel.1 peer $PUBLICKEY endpoint $PUBLIC_IP:51820 allowed-ips $SUBNET persistent-keepalive 25",
	"SubnetRemoveCommand": "read PUBLICKEY; wg set flannel.1 peer $PUBLICKEY remove"
}`
)

func Prepare(ctx context.Context, nodeConfig *config.Node) error {
	if err := createCNIConf(nodeConfig.AgentConfig.CNIConfDir); err != nil {
		return err
	}

	return createFlannelConf(nodeConfig)
}

func Run(ctx context.Context, nodeConfig *config.Node, nodes v1.NodeInterface) error {
	nodeName := nodeConfig.AgentConfig.NodeName

	for {
		node, err := nodes.Get(ctx, nodeName, metav1.GetOptions{})
		if err == nil && node.Spec.PodCIDR != "" {
			break
		}
		if err == nil {
			logrus.Info("Waiting for node " + nodeName + " CIDR not assigned yet")
		} else {
			logrus.Infof("Waiting for node %s: %v", nodeName, err)
		}
		time.Sleep(2 * time.Second)
	}
	logrus.Info("Node CIDR assigned for: " + nodeName)

	go func() {
		err := flannel(ctx, nodeConfig.FlannelIface, nodeConfig.FlannelConf, nodeConfig.AgentConfig.KubeConfigKubelet)
		logrus.Fatalf("flannel exited: %v", err)
	}()

	return nil
}

func createCNIConf(dir string) error {
	if dir == "" {
		return nil
	}
	p := filepath.Join(dir, "10-flannel.conflist")
	return util.WriteFile(p, cniConf)
}

func createFlannelConf(nodeConfig *config.Node) error {
	if nodeConfig.FlannelConf == "" {
		return nil
	}
	if nodeConfig.FlannelConfOverride {
		logrus.Infof("Using custom flannel conf defined at %s", nodeConfig.FlannelConf)
		return nil
	}
	confJSON := strings.ReplaceAll(flannelConf, "%CIDR%", nodeConfig.AgentConfig.ClusterCIDR.String())

	var backendConf string

	switch nodeConfig.FlannelBackend {
	case config.FlannelBackendVXLAN:
		backendConf = vxlanBackend
	case config.FlannelBackendHostGW:
		backendConf = hostGWBackend
	case config.FlannelBackendIPSEC:
		backendConf = strings.ReplaceAll(ipsecBackend, "%psk%", nodeConfig.AgentConfig.IPSECPSK)
		if err := setupStrongSwan(nodeConfig); err != nil {
			return err
		}
	case config.FlannelBackendWireguard:
		backendConf = strings.ReplaceAll(wireguardBackend, "%flannelConfDir%", filepath.Dir(nodeConfig.FlannelConf))
	default:
		return fmt.Errorf("Cannot configure unknown flannel backend '%s'", nodeConfig.FlannelBackend)
	}
	confJSON = strings.ReplaceAll(confJSON, "%backend%", backendConf)

	return util.WriteFile(nodeConfig.FlannelConf, confJSON)
}

func setupStrongSwan(nodeConfig *config.Node) error {
	// if data dir env is not set point to root
	dataDir := os.Getenv(version.ProgramUpper + "_DATA_DIR")
	if dataDir == "" {
		dataDir = "/"
	}
	dataDir = filepath.Join(dataDir, "etc", "strongswan")

	info, err := os.Lstat(nodeConfig.AgentConfig.StrongSwanDir)
	// something exists but is not a symlink, return
	if err == nil && info.Mode()&os.ModeSymlink == 0 {
		return nil
	}
	if err == nil {
		target, err := os.Readlink(nodeConfig.AgentConfig.StrongSwanDir)
		// current link is the same, return
		if err == nil && target == dataDir {
			return nil
		}
	}

	// clean up strongswan old link
	os.Remove(nodeConfig.AgentConfig.StrongSwanDir)

	// make new strongswan link
	return os.Symlink(dataDir, nodeConfig.AgentConfig.StrongSwanDir)
}
