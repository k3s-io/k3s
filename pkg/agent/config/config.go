package config

import (
	"context"
	"crypto/tls"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/norman/pkg/clientaccess"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/util/cert"
	"k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

func Get(ctx context.Context, agent cmds.Agent) *config.Node {
	for {
		agentConfig, err := get(&agent)
		if err != nil {
			logrus.Error(err)
			select {
			case <-time.After(5 * time.Second):
				continue
			case <-ctx.Done():
				logrus.Fatalf("Interrupted")
			}
		}
		return agentConfig
	}
}

func getNodeCert(info *clientaccess.Info) (*tls.Certificate, error) {
	nodeCert, err := clientaccess.Get("/v1-k3s/node.crt", info)
	if err != nil {
		return nil, err
	}

	nodeKey, err := clientaccess.Get("/v1-k3s/node.key", info)
	if err != nil {
		return nil, err
	}

	cert, err := tls.X509KeyPair(nodeCert, nodeKey)
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

func writeNodeCA(dataDir string, nodeCert *tls.Certificate) (string, error) {
	clientCABytes := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: nodeCert.Certificate[1],
	})

	clientCA := filepath.Join(dataDir, "client-ca.pem")
	if err := ioutil.WriteFile(clientCA, clientCABytes, 0600); err != nil {
		return "", errors.Wrapf(err, "failed to write client CA")
	}

	return clientCA, nil
}

func getHostnameAndIP(info cmds.Agent) (string, string, error) {
	ip := info.NodeIP
	if ip == "" {
		hostIP, err := net.ChooseHostInterface()
		if err != nil {
			return "", "", err
		}
		ip = hostIP.String()
	}

	name := info.NodeName
	if name == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return "", "", err
		}
		name = hostname
	}

	return name, ip, nil
}

func localAddress(controlConfig *config.Control) string {
	return fmt.Sprintf("127.0.0.1:%d", controlConfig.AdvertisePort)
}

func writeKubeConfig(envInfo *cmds.Agent, info clientaccess.Info, controlConfig *config.Control, nodeCert *tls.Certificate) (string, error) {
	os.MkdirAll(envInfo.DataDir, 0700)
	kubeConfigPath := filepath.Join(envInfo.DataDir, "kubeconfig.yaml")

	info.URL = "https://" + localAddress(controlConfig)
	info.CACerts = pem.EncodeToMemory(&pem.Block{
		Type:  cert.CertificateBlockType,
		Bytes: nodeCert.Certificate[1],
	})

	return kubeConfigPath, info.WriteKubeConfig(kubeConfigPath)
}

func get(envInfo *cmds.Agent) (*config.Node, error) {
	if envInfo.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	serverURLParsed, err := url.Parse(envInfo.ServerURL)
	if err != nil {
		return nil, err
	}

	info, err := clientaccess.ParseAndValidateToken(envInfo.ServerURL, envInfo.Token)
	if err != nil {
		return nil, err
	}

	controlConfig, err := getConfig(info)
	if err != nil {
		return nil, err
	}

	nodeCert, err := getNodeCert(info)
	if err != nil {
		return nil, err
	}

	clientCA, err := writeNodeCA(envInfo.DataDir, nodeCert)
	if err != nil {
		return nil, err
	}

	nodeName, nodeIP, err := getHostnameAndIP(*envInfo)
	if err != nil {
		return nil, err
	}

	kubeConfig, err := writeKubeConfig(envInfo, *info, controlConfig, nodeCert)
	if err != nil {
		return nil, err
	}

	hostLocal, err := exec.LookPath("host-local")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find host-local")
	}

	nodeConfig := &config.Node{
		Docker:    envInfo.Docker,
		NoFlannel: envInfo.NoFlannel,
	}
	nodeConfig.LocalAddress = localAddress(controlConfig)
	nodeConfig.AgentConfig.NodeIP = nodeIP
	nodeConfig.AgentConfig.NodeName = nodeName
	nodeConfig.AgentConfig.ClusterDNS = controlConfig.ClusterDNS
	nodeConfig.AgentConfig.CACertPath = clientCA
	nodeConfig.AgentConfig.ListenAddress = "127.0.0.1"
	nodeConfig.AgentConfig.KubeConfig = kubeConfig
	nodeConfig.AgentConfig.RootDir = filepath.Join(envInfo.DataDir, "kubelet")
	nodeConfig.CACerts = info.CACerts
	nodeConfig.Containerd.Config = filepath.Join(envInfo.DataDir, "etc/containerd/config.toml")
	nodeConfig.Containerd.Root = filepath.Join(envInfo.DataDir, "containerd")
	nodeConfig.Containerd.Opt = filepath.Join(envInfo.DataDir, "containerd")
	if !envInfo.Debug {
		nodeConfig.Containerd.Log = filepath.Join(envInfo.DataDir, "containerd/containerd.log")
	}
	nodeConfig.Containerd.State = "/run/k3s/containerd"
	nodeConfig.Containerd.Address = filepath.Join(nodeConfig.Containerd.State, "containerd.sock")
	nodeConfig.ServerAddress = serverURLParsed.Host
	nodeConfig.Certificate = nodeCert
	if !nodeConfig.NoFlannel {
		nodeConfig.FlannelConf = filepath.Join(envInfo.DataDir, "etc/flannel/net-conf.json")
		nodeConfig.AgentConfig.CNIBinDir = filepath.Dir(hostLocal)
		nodeConfig.AgentConfig.CNIConfDir = filepath.Join(envInfo.DataDir, "etc/cni/net.d")
	}
	if !nodeConfig.Docker {
		nodeConfig.AgentConfig.RuntimeSocket = "unix://" + nodeConfig.Containerd.Address
	}
	if controlConfig.ClusterIPRange != nil {
		nodeConfig.AgentConfig.ClusterCIDR = *controlConfig.ClusterIPRange
	}

	os.Setenv("NODE_NAME", nodeConfig.AgentConfig.NodeName)
	v1beta1.KubeletSocket = filepath.Join(envInfo.DataDir, "kubelet/device-plugins/kubelet.sock")

	return nodeConfig, nil
}

func getConfig(info *clientaccess.Info) (*config.Control, error) {
	data, err := clientaccess.Get("/v1-k3s/config", info)
	if err != nil {
		return nil, err
	}

	controlControl := &config.Control{}
	return controlControl, json.Unmarshal(data, controlControl)
}
