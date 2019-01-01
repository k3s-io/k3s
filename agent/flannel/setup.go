package flannel

import (
	"context"
	"strings"
	"time"

	"github.com/rancher/rio/agent/util"

	"github.com/rancher/rio/agent/config"
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
	netJson = `{
    "Network": "%CIDR%",
    "Backend": {
    "Type": "vxlan"
    }
}
`
)

func Run(ctx context.Context, config *config.NodeConfig) error {
	nodeName := config.AgentConfig.NodeName

	restConfig, err := clientcmd.BuildConfigFromFlags("", config.AgentConfig.KubeConfig)
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

	if err := createCNIConf(); err != nil {
		return err
	}

	if err := createFlannelConf(config); err != nil {
		return err
	}

	return flannel(ctx, config.AgentConfig.KubeConfig)
}

func createCNIConf() error {
	return util.WriteFile("/etc/cni/net.d/10-flannel.conflist", cniConf)
}

func createFlannelConf(config *config.NodeConfig) error {
	return util.WriteFile("/etc/kube-flannel/net-conf.json",
		strings.Replace(netJson, "%CIDR", config.AgentConfig.ClusterCIDR.String(), -1))
}
