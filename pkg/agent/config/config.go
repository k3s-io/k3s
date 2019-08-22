package config

import (
	"bufio"
	"context"
	cryptorand "crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	sysnet "net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/clientaccess"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/daemons/control"
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

type HTTPRequester func(u string, client *http.Client, username, password string) ([]byte, error)

func Request(path string, info *clientaccess.Info, requester HTTPRequester) ([]byte, error) {
	u, err := url.Parse(info.URL)
	if err != nil {
		return nil, err
	}
	u.Path = path
	username, password, _ := clientaccess.ParseUsernamePassword(info.Token)
	return requester(u.String(), clientaccess.GetHTTPClient(info.CACerts), username, password)
}

func getNodeNamedCrt(nodeName, nodePasswordFile string) HTTPRequester {
	return func(u string, client *http.Client, username, password string) ([]byte, error) {
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}

		if username != "" {
			req.SetBasicAuth(username, password)
		}

		req.Header.Set("K3s-Node-Name", nodeName)
		nodePassword, err := ensureNodePassword(nodePasswordFile)
		if err != nil {
			return nil, err
		}
		req.Header.Set("K3s-Node-Password", nodePassword)

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("Node password rejected, contents of '%s' may not match server passwd entry", nodePasswordFile)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%s: %s", u, resp.Status)
		}

		return ioutil.ReadAll(resp.Body)
	}
}

func ensureNodePassword(nodePasswordFile string) (string, error) {
	if _, err := os.Stat(nodePasswordFile); err == nil {
		password, err := ioutil.ReadFile(nodePasswordFile)
		return strings.TrimSpace(string(password)), err
	}
	password := make([]byte, 16, 16)
	_, err := cryptorand.Read(password)
	if err != nil {
		return "", err
	}
	nodePassword := hex.EncodeToString(password)
	return nodePassword, ioutil.WriteFile(nodePasswordFile, []byte(nodePassword+"\n"), 0600)
}

func getServingCert(nodeName, servingCertFile, servingKeyFile, nodePasswordFile string, info *clientaccess.Info) (*tls.Certificate, error) {
	servingCert, err := Request("/v1-k3s/serving-kubelet.crt", info, getNodeNamedCrt(nodeName, nodePasswordFile))
	if err != nil {
		return nil, err
	}
	if err := ioutil.WriteFile(servingCertFile, servingCert, 0600); err != nil {
		return nil, errors.Wrapf(err, "failed to write node cert")
	}

	servingKey, err := clientaccess.Get("/v1-k3s/serving-kubelet.key", info)
	if err != nil {
		return nil, err
	}
	if err := ioutil.WriteFile(servingKeyFile, servingKey, 0600); err != nil {
		return nil, errors.Wrapf(err, "failed to write node key")
	}

	cert, err := tls.X509KeyPair(servingCert, servingKey)
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

func getHostFile(filename string, info *clientaccess.Info) error {
	basename := filepath.Base(filename)
	fileBytes, err := clientaccess.Get("/v1-k3s/"+basename, info)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(filename, fileBytes, 0600); err != nil {
		return errors.Wrapf(err, "failed to write cert %s", filename)
	}
	return nil
}

func getNodeNamedHostFile(filename, nodeName, nodePasswordFile string, info *clientaccess.Info) error {
	basename := filepath.Base(filename)
	fileBytes, err := Request("/v1-k3s/"+basename, info, getNodeNamedCrt(nodeName, nodePasswordFile))
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(filename, fileBytes, 0600); err != nil {
		return errors.Wrapf(err, "failed to write cert %s", filename)
	}
	return nil
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

	// Use lower case hostname to comply with kubernetes constraint:
	// https://github.com/kubernetes/kubernetes/issues/71140
	name = strings.ToLower(name)

	return name, ip, nil
}

func writeKubeConfig(envInfo *cmds.Agent, info clientaccess.Info, tlsCert *tls.Certificate) (string, error) {
	os.MkdirAll(envInfo.DataDir, 0700)
	kubeConfigPath := filepath.Join(envInfo.DataDir, "kubeconfig.yaml")
	info.CACerts = pem.EncodeToMemory(&pem.Block{
		Type:  cert.CertificateBlockType,
		Bytes: tlsCert.Certificate[1],
	})

	return kubeConfigPath, info.WriteKubeConfig(kubeConfigPath)
}

func isValidResolvConf(resolvConfFile string) bool {
	file, err := os.Open(resolvConfFile)
	if err != nil {
		return false
	}
	defer file.Close()

	nameserver := regexp.MustCompile(`^nameserver\s+([^\s]*)`)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		ipMatch := nameserver.FindStringSubmatch(scanner.Text())
		if len(ipMatch) == 2 {
			ip := sysnet.ParseIP(ipMatch[1])
			if ip == nil || !ip.IsGlobalUnicast() {
				return false
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return false
	}
	return true
}

func locateOrGenerateResolvConf(envInfo *cmds.Agent) string {
	if envInfo.ResolvConf != "" {
		return envInfo.ResolvConf
	}
	resolvConfs := []string{"/etc/resolv.conf", "/run/systemd/resolve/resolv.conf"}
	for _, conf := range resolvConfs {
		if isValidResolvConf(conf) {
			return conf
		}
	}

	tmpConf := filepath.Join(os.TempDir(), "k3s-resolv.conf")
	if err := ioutil.WriteFile(tmpConf, []byte("nameserver 8.8.8.8\n"), 0444); err != nil {
		logrus.Error(err)
		return ""
	}
	return tmpConf
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

	nodeName, nodeIP, err := getHostnameAndIP(*envInfo)
	if err != nil {
		return nil, err
	}

	hostLocal, err := exec.LookPath("host-local")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find host-local")
	}

	var flannelIface *sysnet.Interface
	if !envInfo.NoFlannel && len(envInfo.FlannelIface) > 0 {
		flannelIface, err = sysnet.InterfaceByName(envInfo.FlannelIface)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to find interface")
		}
	}

	clientCAFile := filepath.Join(envInfo.DataDir, "client-ca.crt")
	if err := getHostFile(clientCAFile, info); err != nil {
		return nil, err
	}

	serverCAFile := filepath.Join(envInfo.DataDir, "server-ca.crt")
	if err := getHostFile(serverCAFile, info); err != nil {
		return nil, err
	}

	servingKubeletCert := filepath.Join(envInfo.DataDir, "serving-kubelet.crt")
	servingKubeletKey := filepath.Join(envInfo.DataDir, "serving-kubelet.key")
	nodePasswordFile := filepath.Join(envInfo.DataDir, "node-password.txt")
	servingCert, err := getServingCert(nodeName, servingKubeletCert, servingKubeletKey, nodePasswordFile, info)
	if err != nil {
		return nil, err
	}

	kubeconfigNode, err := writeKubeConfig(envInfo, *info, servingCert)
	if err != nil {
		return nil, err
	}

	clientKubeletCert := filepath.Join(envInfo.DataDir, "client-kubelet.crt")
	if err := getNodeNamedHostFile(clientKubeletCert, nodeName, nodePasswordFile, info); err != nil {
		return nil, err
	}

	clientKubeletKey := filepath.Join(envInfo.DataDir, "client-kubelet.key")
	if err := getHostFile(clientKubeletKey, info); err != nil {
		return nil, err
	}

	kubeconfigKubelet := filepath.Join(envInfo.DataDir, "kubelet.kubeconfig")
	if err := control.KubeConfig(kubeconfigKubelet, info.URL, serverCAFile, clientKubeletCert, clientKubeletKey); err != nil {
		return nil, err
	}

	clientKubeProxyCert := filepath.Join(envInfo.DataDir, "client-kube-proxy.crt")
	if err := getHostFile(clientKubeProxyCert, info); err != nil {
		return nil, err
	}

	clientKubeProxyKey := filepath.Join(envInfo.DataDir, "client-kube-proxy.key")
	if err := getHostFile(clientKubeProxyKey, info); err != nil {
		return nil, err
	}

	kubeconfigKubeproxy := filepath.Join(envInfo.DataDir, "kubeproxy.kubeconfig")
	if err := control.KubeConfig(kubeconfigKubeproxy, info.URL, serverCAFile, clientKubeProxyCert, clientKubeProxyKey); err != nil {
		return nil, err
	}

	nodeConfig := &config.Node{
		Docker:                   envInfo.Docker,
		NoFlannel:                envInfo.NoFlannel,
		ContainerRuntimeEndpoint: envInfo.ContainerRuntimeEndpoint,
	}
	nodeConfig.FlannelIface = flannelIface
	nodeConfig.Images = filepath.Join(envInfo.DataDir, "images")
	nodeConfig.AgentConfig.NodeIP = nodeIP
	nodeConfig.AgentConfig.NodeName = nodeName
	nodeConfig.AgentConfig.ServingKubeletCert = servingKubeletCert
	nodeConfig.AgentConfig.ServingKubeletKey = servingKubeletKey
	nodeConfig.AgentConfig.ClusterDNS = controlConfig.ClusterDNS
	nodeConfig.AgentConfig.ClusterDomain = controlConfig.ClusterDomain
	nodeConfig.AgentConfig.ResolvConf = locateOrGenerateResolvConf(envInfo)
	nodeConfig.AgentConfig.ClientCA = clientCAFile
	nodeConfig.AgentConfig.ListenAddress = "0.0.0.0"
	nodeConfig.AgentConfig.KubeConfigNode = kubeconfigNode
	nodeConfig.AgentConfig.KubeConfigKubelet = kubeconfigKubelet
	nodeConfig.AgentConfig.KubeConfigKubeProxy = kubeconfigKubeproxy
	nodeConfig.AgentConfig.RootDir = filepath.Join(envInfo.DataDir, "kubelet")
	nodeConfig.AgentConfig.PauseImage = envInfo.PauseImage
	nodeConfig.CACerts = info.CACerts
	nodeConfig.Containerd.Config = filepath.Join(envInfo.DataDir, "etc/containerd/config.toml")
	nodeConfig.Containerd.Root = filepath.Join(envInfo.DataDir, "containerd")
	nodeConfig.Containerd.Opt = filepath.Join(envInfo.DataDir, "containerd")
	if !envInfo.Debug {
		nodeConfig.Containerd.Log = filepath.Join(envInfo.DataDir, "containerd/containerd.log")
	}
	nodeConfig.Containerd.State = "/run/k3s/containerd"
	nodeConfig.Containerd.Address = filepath.Join(nodeConfig.Containerd.State, "containerd.sock")
	nodeConfig.Containerd.Template = filepath.Join(envInfo.DataDir, "etc/containerd/config.toml.tmpl")
	nodeConfig.ServerAddress = serverURLParsed.Host
	nodeConfig.Certificate = servingCert
	if !nodeConfig.NoFlannel {
		nodeConfig.FlannelConf = filepath.Join(envInfo.DataDir, "etc/flannel/net-conf.json")
		nodeConfig.AgentConfig.CNIBinDir = filepath.Dir(hostLocal)
		nodeConfig.AgentConfig.CNIConfDir = filepath.Join(envInfo.DataDir, "etc/cni/net.d")
	}
	if !nodeConfig.Docker && nodeConfig.ContainerRuntimeEndpoint == "" {
		nodeConfig.AgentConfig.RuntimeSocket = "unix://" + nodeConfig.Containerd.Address
	} else {
		nodeConfig.AgentConfig.RuntimeSocket = "unix://" + nodeConfig.ContainerRuntimeEndpoint
	}
	if controlConfig.ClusterIPRange != nil {
		nodeConfig.AgentConfig.ClusterCIDR = *controlConfig.ClusterIPRange
	}

	os.Setenv("NODE_NAME", nodeConfig.AgentConfig.NodeName)
	v1beta1.KubeletSocket = filepath.Join(envInfo.DataDir, "kubelet/device-plugins/kubelet.sock")

	nodeConfig.AgentConfig.ExtraKubeletArgs = envInfo.ExtraKubeletArgs
	nodeConfig.AgentConfig.ExtraKubeProxyArgs = envInfo.ExtraKubeProxyArgs

	nodeConfig.AgentConfig.NodeTaints = envInfo.Taints
	nodeConfig.AgentConfig.NodeLabels = envInfo.Labels

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
