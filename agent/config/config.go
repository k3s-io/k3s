package config

import (
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/pkg/clientaccess"
	"github.com/rancher/rio/pkg/daemons/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/json"
	net2 "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/util/cert"
)

type envInfo struct {
	ServerURL string
	Token     string
	DataDir   string
	NodeIP    string
	NodeName  string
}

func Get() *config.Node {
	for {
		agentConfig, err := get()
		if err != nil {
			logrus.Error(err)
			time.Sleep(5 * time.Second)
			continue
		}
		return agentConfig
	}
}

func getEnvInfo() (*envInfo, error) {
	u := os.Getenv("K3S_URL")
	if u == "" {
		return nil, fmt.Errorf("K3S_URL env var is required")
	}

	_, err := url.Parse(u)
	if err != nil {
		return nil, fmt.Errorf("K3S_URL [%s] is invalid: %v", u, err)
	}

	t := os.Getenv("K3S_TOKEN")
	if t == "" {
		return nil, fmt.Errorf("K3S_TOKEN env var is required")
	}

	dataDir := os.Getenv("K3S_DATA_DIR")
	if dataDir == "" {
		return nil, fmt.Errorf("K3S_DATA_DIR is required")
	}

	return &envInfo{
		ServerURL: u,
		Token:     t,
		DataDir:   dataDir,
		NodeIP:    os.Getenv("K3S_NODE_IP"),
		NodeName:  os.Getenv("NODE_NAME"),
	}, nil
}

func getNodeCert(info *clientaccess.Info) (*tls.Certificate, error) {
	nodeCert, err := clientaccess.Get("/v1-k3s/node.cert", info)
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

func getHostnameAndIP(info envInfo) (string, string, error) {
	ip := info.NodeIP
	if ip == "" {
		hostIP, err := net2.ChooseHostInterface()
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
		hostname = strings.Split(hostname, ".")[0]

		d := md5.Sum([]byte(ip))
		name = hostname + "-" + hex.EncodeToString(d[:])[:8]
	}

	return name, ip, nil
}

func localAddress(controlConfig *config.Control) string {
	return fmt.Sprintf("127.0.0.1:%d", controlConfig.AdvertisePort)
}

func writeKubeConfig(envInfo *envInfo, info clientaccess.Info, controlConfig *config.Control, nodeCert *tls.Certificate) (string, error) {
	os.MkdirAll(envInfo.DataDir, 0700)
	kubeConfigPath := filepath.Join(envInfo.DataDir, "kubeconfig.yaml")

	info.URL = "https://" + localAddress(controlConfig)
	info.CACerts = pem.EncodeToMemory(&pem.Block{
		Type:  cert.CertificateBlockType,
		Bytes: nodeCert.Certificate[1],
	})

	return kubeConfigPath, info.WriteKubeConfig(kubeConfigPath)
}

func get() (*config.Node, error) {
	envInfo, err := getEnvInfo()
	if err != nil {
		return nil, err
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

	nodeConfig := &controlConfig.NodeConfig
	nodeConfig.LocalAddress = localAddress(controlConfig)
	nodeConfig.AgentConfig.NodeIP = defString(nodeConfig.AgentConfig.NodeIP, nodeIP)
	nodeConfig.AgentConfig.NodeName = defString(nodeConfig.AgentConfig.NodeName, nodeName)
	nodeConfig.AgentConfig.CNIBinDir = defString(nodeConfig.AgentConfig.CNIBinDir, "/usr/share/cni")
	nodeConfig.AgentConfig.CACertPath = clientCA
	nodeConfig.AgentConfig.ListenAddress = defString(nodeConfig.AgentConfig.ListenAddress, "127.0.0.1")
	nodeConfig.AgentConfig.KubeConfig = kubeConfig
	nodeConfig.CACerts = info.CACerts
	nodeConfig.ServerAddress = serverURLParsed.Host
	nodeConfig.Certificate = nodeCert
	if !nodeConfig.Docker {
		nodeConfig.AgentConfig.RuntimeSocket = "/run/k3s/containerd.sock"
	}

	return nodeConfig, nil
}

func defString(val, newVal string) string {
	if val == "" {
		return newVal
	}
	return val
}

func getConfig(info *clientaccess.Info) (*config.Control, error) {
	data, err := clientaccess.Get("/v1-k3s/config", info)
	if err != nil {
		return nil, err
	}

	controlControl := &config.Control{}
	return controlControl, json.Unmarshal(data, controlControl)
}
