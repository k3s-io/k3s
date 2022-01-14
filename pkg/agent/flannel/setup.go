package flannel

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/agent/util"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	utilsnet "k8s.io/utils/net"
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
	"EnableIPv6": %DUALSTACK%,
	"IPv6Network": "%CIDR_IPV6%",
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

	emptyIPv6Network = "::/0"

	ipv4 = iota
	ipv6
)

func Prepare(ctx context.Context, nodeConfig *config.Node) error {
	if err := createCNIConf(nodeConfig.AgentConfig.CNIConfDir); err != nil {
		return err
	}

	return createFlannelConf(nodeConfig)
}

func Run(ctx context.Context, nodeConfig *config.Node, nodes typedcorev1.NodeInterface) error {
	logrus.Infof("Starting flannel with backend %s", nodeConfig.FlannelBackend)
	if err := waitForPodCIDR(ctx, nodeConfig.AgentConfig.NodeName, nodes); err != nil {
		return errors.Wrap(err, "flannel failed to wait for PodCIDR assignment")
	}

	netMode, err := findNetMode(nodeConfig.AgentConfig.ClusterCIDRs)
	if err != nil {
		return errors.Wrap(err, "failed to check netMode for flannel")
	}
	go func() {
		err := flannel(ctx, nodeConfig.FlannelIface, nodeConfig.FlannelConfFile, nodeConfig.AgentConfig.KubeConfigKubelet, nodeConfig.FlannelIPv6Masq, netMode)
		if err != nil && !errors.Is(err, context.Canceled) {
			logrus.Fatalf("flannel exited: %v", err)
		}
	}()

	return nil
}

// waitForPodCIDR watches nodes with this node's name, and returns when the PodCIDR has been set.
func waitForPodCIDR(ctx context.Context, nodeName string, nodes typedcorev1.NodeInterface) error {
	fieldSelector := fields.Set{metav1.ObjectNameField: nodeName}.String()
	watch, err := nodes.Watch(ctx, metav1.ListOptions{FieldSelector: fieldSelector})
	if err != nil {
		return err
	}
	defer watch.Stop()

	for ev := range watch.ResultChan() {
		node, ok := ev.Object.(*corev1.Node)
		if !ok {
			return fmt.Errorf("could not convert event object to node: %v", ev)
		}
		if node.Spec.PodCIDR != "" {
			break
		}
	}
	logrus.Info("Flannel found PodCIDR assigned for node " + nodeName)
	return nil
}

func createCNIConf(dir string) error {
	logrus.Debugf("Creating the CNI conf in directory %s", dir)
	if dir == "" {
		return nil
	}
	p := filepath.Join(dir, "10-flannel.conflist")
	return util.WriteFile(p, cniConf)
}

func createFlannelConf(nodeConfig *config.Node) error {
	logrus.Debugf("Creating the flannel configuration for backend %s in file %s", nodeConfig.FlannelBackend, nodeConfig.FlannelConfFile)
	if nodeConfig.FlannelConfFile == "" {
		return errors.New("Flannel configuration not defined")
	}
	if nodeConfig.FlannelConfOverride {
		logrus.Infof("Using custom flannel conf defined at %s", nodeConfig.FlannelConfFile)
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
		backendConf = strings.ReplaceAll(wireguardBackend, "%flannelConfDir%", filepath.Dir(nodeConfig.FlannelConfFile))
	default:
		return fmt.Errorf("Cannot configure unknown flannel backend '%s'", nodeConfig.FlannelBackend)
	}
	confJSON = strings.ReplaceAll(confJSON, "%backend%", backendConf)

	netMode, err := findNetMode(nodeConfig.AgentConfig.ClusterCIDRs)
	if err != nil {
		logrus.Fatalf("Flannel error checking netMode: %v", err)
		return err
	}

	if netMode == (ipv4 + ipv6) {
		confJSON = strings.ReplaceAll(confJSON, "%DUALSTACK%", "true")
		for _, cidr := range nodeConfig.AgentConfig.ClusterCIDRs {
			if utilsnet.IsIPv6(cidr.IP) {
				// Only one ipv6 range available. This might change in future: https://github.com/kubernetes/enhancements/issues/2593
				confJSON = strings.ReplaceAll(confJSON, "%CIDR_IPV6%", cidr.String())
			}
		}
	} else {
		confJSON = strings.ReplaceAll(confJSON, "%DUALSTACK%", "false")
		confJSON = strings.ReplaceAll(confJSON, "%CIDR_IPV6%", emptyIPv6Network)
	}

	logrus.Debugf("The flannel configuration is %s", confJSON)
	return util.WriteFile(nodeConfig.FlannelConfFile, confJSON)
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

// fundNetMode returns the mode (ipv4, ipv6 or dual-stack) in which flannel is operating
func findNetMode(cidrs []*net.IPNet) (int, error) {
	dualStack, err := utilsnet.IsDualStackCIDRs(cidrs)
	if err != nil {
		return 0, err
	}
	if dualStack {
		return ipv4 + ipv6, nil
	}

	for _, cidr := range cidrs {
		if utilsnet.IsIPv4CIDR(cidr) {
			return ipv4, nil
		}
		if utilsnet.IsIPv6CIDR(cidr) {
			return ipv6, nil
		}
	}
	return 0, errors.New("Failed checking netMode")
}
