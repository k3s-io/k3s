package flannel

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/k3s-io/k3s/pkg/agent/util"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	toolswatch "k8s.io/client-go/tools/watch"
	utilsnet "k8s.io/utils/net"
)

const (
	cniConf = `{
  "name":"cbr0",
  "cniVersion":"1.0.0",
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
	"EnableIPv6": %IPV6_ENABLED%,
	"EnableIPv4": %IPV4_ENABLED%,
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

	wireguardNativeBackend = `{
	"Type": "wireguard",
	"PersistentKeepaliveInterval": %PersistentKeepaliveInterval%,
	"Mode": "%Mode%"
}`

	emptyIPv6Network = "::/0"

	ipv4 = iota
	ipv6
)

func Prepare(ctx context.Context, nodeConfig *config.Node) error {
	if err := createCNIConf(nodeConfig.AgentConfig.CNIConfDir, nodeConfig); err != nil {
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
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (object runtime.Object, e error) {
			options.FieldSelector = fieldSelector
			return nodes.List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
			options.FieldSelector = fieldSelector
			return nodes.Watch(ctx, options)
		},
	}
	condition := func(ev watch.Event) (bool, error) {
		if n, ok := ev.Object.(*v1.Node); ok {
			return n.Spec.PodCIDR != "", nil
		}
		return false, errors.New("event object not of type v1.Node")
	}

	if _, err := toolswatch.UntilWithSync(ctx, lw, &v1.Node{}, nil, condition); err != nil {
		return errors.Wrap(err, "failed to wait for PodCIDR assignment")
	}

	logrus.Info("Flannel found PodCIDR assigned for node " + nodeName)
	return nil
}

func createCNIConf(dir string, nodeConfig *config.Node) error {
	logrus.Debugf("Creating the CNI conf in directory %s", dir)
	if dir == "" {
		return nil
	}
	p := filepath.Join(dir, "10-flannel.conflist")

	if nodeConfig.AgentConfig.FlannelCniConfFile != "" {
		logrus.Debugf("Using %s as the flannel CNI conf", nodeConfig.AgentConfig.FlannelCniConfFile)
		return util.CopyFile(nodeConfig.AgentConfig.FlannelCniConfFile, p)
	}
	return util.WriteFile(p, cniConf)
}

func createFlannelConf(nodeConfig *config.Node) error {
	var ipv4Enabled string
	logrus.Debugf("Creating the flannel configuration for backend %s in file %s", nodeConfig.FlannelBackend, nodeConfig.FlannelConfFile)
	if nodeConfig.FlannelConfFile == "" {
		return errors.New("Flannel configuration not defined")
	}
	if nodeConfig.FlannelConfOverride {
		logrus.Infof("Using custom flannel conf defined at %s", nodeConfig.FlannelConfFile)
		return nil
	}
	netMode, err := findNetMode(nodeConfig.AgentConfig.ClusterCIDRs)
	if err != nil {
		logrus.Fatalf("Flannel error checking netMode: %v", err)
		return err
	}
	if netMode == ipv4 || netMode == (ipv4+ipv6) {
		ipv4Enabled = "true"
	} else {
		ipv4Enabled = "false"
	}
	confJSON := strings.ReplaceAll(flannelConf, "%IPV4_ENABLED%", ipv4Enabled)
	if netMode == ipv4 {
		confJSON = strings.ReplaceAll(confJSON, "%CIDR%", nodeConfig.AgentConfig.ClusterCIDR.String())
		confJSON = strings.ReplaceAll(confJSON, "%IPV6_ENABLED%", "false")
		confJSON = strings.ReplaceAll(confJSON, "%CIDR_IPV6%", emptyIPv6Network)
	} else if netMode == (ipv4 + ipv6) {
		confJSON = strings.ReplaceAll(confJSON, "%CIDR%", nodeConfig.AgentConfig.ClusterCIDR.String())
		confJSON = strings.ReplaceAll(confJSON, "%IPV6_ENABLED%", "true")
		for _, cidr := range nodeConfig.AgentConfig.ClusterCIDRs {
			if utilsnet.IsIPv6(cidr.IP) {
				// Only one ipv6 range available. This might change in future: https://github.com/kubernetes/enhancements/issues/2593
				confJSON = strings.ReplaceAll(confJSON, "%CIDR_IPV6%", cidr.String())
			}
		}
	} else {
		confJSON = strings.ReplaceAll(confJSON, "%CIDR%", "0.0.0.0/0")
		confJSON = strings.ReplaceAll(confJSON, "%IPV6_ENABLED%", "true")
		for _, cidr := range nodeConfig.AgentConfig.ClusterCIDRs {
			if utilsnet.IsIPv6(cidr.IP) {
				// Only one ipv6 range available. This might change in future: https://github.com/kubernetes/enhancements/issues/2593
				confJSON = strings.ReplaceAll(confJSON, "%CIDR_IPV6%", cidr.String())
			}
		}
	}

	var backendConf string
	parts := strings.SplitN(nodeConfig.FlannelBackend, "=", 2)
	backend := parts[0]
	backendOptions := make(map[string]string)
	if len(parts) > 1 {
		options := strings.Split(parts[1], ",")
		for _, o := range options {
			p := strings.SplitN(o, "=", 2)
			if len(p) == 1 {
				backendOptions[p[0]] = ""
			} else {
				backendOptions[p[0]] = p[1]
			}
		}
	}

	switch backend {
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
		logrus.Warnf("The wireguard backend is deprecated and will be removed in k3s v1.26, please switch to wireguard-native. Check our docs for information about how to migrate")
	case config.FlannelBackendWireguardNative:
		mode, ok := backendOptions["Mode"]
		if !ok {
			mode = "separate"
		}
		keepalive, ok := backendOptions["PersistentKeepaliveInterval"]
		if !ok {
			keepalive = "25"
		}
		backendConf = strings.ReplaceAll(wireguardNativeBackend, "%Mode%", mode)
		backendConf = strings.ReplaceAll(backendConf, "%PersistentKeepaliveInterval%", keepalive)
	default:
		return fmt.Errorf("Cannot configure unknown flannel backend '%s'", nodeConfig.FlannelBackend)
	}
	confJSON = strings.ReplaceAll(confJSON, "%backend%", backendConf)

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
