package flannel

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/rancher/k3s/pkg/agent/util"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
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

	ipsecBackend = `{
	"Type": "ipsec",
	"UDPEncap": true,
	"PSK": "%psk%"
}`

	wireguardBackend = `{
	"Type": "extension",
	"PreStartupCommand": "wg genkey | tee privatekey | wg pubkey",
	"PostStartupCommand": "export SUBNET_IP=$(echo $SUBNET | cut -d'/' -f 1); ip link del flannel.1 2>/dev/null; echo $PATH >&2; wg-add.sh flannel.1 && wg set flannel.1 listen-port 51820 private-key privatekey && ip addr add $SUBNET_IP/32 dev flannel.1 && ip link set flannel.1 up && ip route add $NETWORK dev flannel.1",
	"ShutdownCommand": "ip link del flannel.1",
	"SubnetAddCommand": "read PUBLICKEY; wg set flannel.1 peer $PUBLICKEY endpoint $PUBLIC_IP:51820 allowed-ips $SUBNET",
	"SubnetRemoveCommand": "read PUBLICKEY; wg set flannel.1 peer $PUBLICKEY remove"
}`
)

func Prepare(ctx context.Context, nodeConfig *config.Node) error {
	if err := createCNIConf(nodeConfig.AgentConfig.CNIConfDir); err != nil {
		return err
	}

	return createFlannelConf(nodeConfig)
}

func Run(ctx context.Context, nodeConfig *config.Node) error {
	nodeName := nodeConfig.AgentConfig.NodeName

	restConfig, err := clientcmd.BuildConfigFromFlags("", nodeConfig.AgentConfig.KubeConfigNode)
	if err != nil {
		return err
	}

	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	for {
		node, err := client.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
		if err == nil && node.Spec.PodCIDR != "" {
			break
		}
		if err == nil {
			logrus.Infof("waiting for node %s CIDR not assigned yet", nodeName)
		} else {
			logrus.Infof("waiting for node %s: %v", nodeName, err)
		}
		time.Sleep(2 * time.Second)
	}

	go func() {
		err := flannel(ctx, nodeConfig.FlannelIface, nodeConfig.FlannelConf, nodeConfig.AgentConfig.KubeConfigNode)
		logrus.Fatalf("flannel exited: %v", err)
	}()

	return err
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
	confJSON := strings.Replace(flannelConf, "%CIDR%", nodeConfig.AgentConfig.ClusterCIDR.String(), -1)

	var backendConf string

	switch nodeConfig.FlannelBackend {
	case config.FlannelBackendVXLAN:
		backendConf = vxlanBackend
	case config.FlannelBackendIPSEC:
		backendConf = strings.Replace(ipsecBackend, "%psk%", nodeConfig.AgentConfig.IPSECPSK, -1)
		if err := setupStrongSwan(nodeConfig); err != nil {
			return err
		}
	case config.FlannelBackendWireguard:
		backendConf = wireguardBackend
	default:
		return fmt.Errorf("Cannot configure unknown flannel backend '%s'", nodeConfig.FlannelBackend)
	}
	confJSON = strings.Replace(confJSON, "%backend%", backendConf, -1)

	return util.WriteFile(nodeConfig.FlannelConf, confJSON)
}

func setupStrongSwan(nodeConfig *config.Node) error {
	// if data dir env is not set point to root
	dataDir := os.Getenv("K3S_DATA_DIR")
	if dataDir == "" {
		dataDir = "/"
	}
	dataDir = path.Join(dataDir, "etc", "strongswan")

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
