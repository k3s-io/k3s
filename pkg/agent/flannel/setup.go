package flannel

import (
	"context"
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
	netJSON = `{
    "Network": "%CIDR%",
    "Backend": {
    "Type": "vxlan"
    }
}
`
)

func Prepare(ctx context.Context, config *config.Node) error {
	if err := createCNIConf(config.AgentConfig.CNIConfDir); err != nil {
		return err
	}

	return createFlannelConf(config)
}

func Run(ctx context.Context, config *config.Node) error {
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

	go func() {
		err := flannel(ctx, config.FlannelIface, config.FlannelConf, config.AgentConfig.KubeConfig)
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

func createFlannelConf(config *config.Node) error {
	if config.FlannelConf == "" {
		return nil
	}
	return util.WriteFile(config.FlannelConf,
		strings.Replace(netJSON, "%CIDR%", config.AgentConfig.ClusterCIDR.String(), -1))
}
