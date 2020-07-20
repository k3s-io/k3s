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
	"github.com/rancher/k3s/pkg/agent/proxy"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/clientaccess"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/daemons/control"
	"github.com/rancher/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/net"
)

const (
	DefaultPodManifestPath = "pod-manifests"
)

func Get(ctx context.Context, agent cmds.Agent, proxy proxy.Proxy) *config.Node {
	for {
		agentConfig, err := get(&agent, proxy)
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

		req.Header.Set(version.Program+"-Node-Name", nodeName)
		nodePassword, err := ensureNodePassword(nodePasswordFile)
		if err != nil {
			return nil, err
		}
		req.Header.Set(version.Program+"-Node-Password", nodePassword)

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("Node password rejected, duplicate hostname or contents of '%s' may not match server node-passwd entry, try enabling a unique node name with the --with-node-id flag", nodePasswordFile)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%s: %s", u, resp.Status)
		}

		return ioutil.ReadAll(resp.Body)
	}
}

func ensureNodeID(nodeIDFile string) (string, error) {
	if _, err := os.Stat(nodeIDFile); err == nil {
		id, err := ioutil.ReadFile(nodeIDFile)
		return strings.TrimSpace(string(id)), err
	}
	id := make([]byte, 4, 4)
	_, err := cryptorand.Read(id)
	if err != nil {
		return "", err
	}
	nodeID := hex.EncodeToString(id)
	return nodeID, ioutil.WriteFile(nodeIDFile, []byte(nodeID+"\n"), 0644)
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

func upgradeOldNodePasswordPath(oldNodePasswordFile, newNodePasswordFile string) {
	password, err := ioutil.ReadFile(oldNodePasswordFile)
	if err != nil {
		return
	}
	if err := ioutil.WriteFile(newNodePasswordFile, password, 0600); err != nil {
		logrus.Warnf("Unable to write password file: %v", err)
		return
	}
	if err := os.Remove(oldNodePasswordFile); err != nil {
		logrus.Warnf("Unable to remove old password file: %v", err)
		return
	}
}

func getServingCert(nodeName, servingCertFile, servingKeyFile, nodePasswordFile string, info *clientaccess.Info) (*tls.Certificate, error) {
	servingCert, err := Request("/v1-"+version.Program+"/serving-kubelet.crt", info, getNodeNamedCrt(nodeName, nodePasswordFile))
	if err != nil {
		return nil, err
	}

	servingCert, servingKey := splitCertKeyPEM(servingCert)

	if err := ioutil.WriteFile(servingCertFile, servingCert, 0600); err != nil {
		return nil, errors.Wrapf(err, "failed to write node cert")
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

func getHostFile(filename, keyFile string, info *clientaccess.Info) error {
	basename := filepath.Base(filename)
	fileBytes, err := clientaccess.Get("/v1-"+version.Program+"/"+basename, info)
	if err != nil {
		return err
	}
	if keyFile == "" {
		if err := ioutil.WriteFile(filename, fileBytes, 0600); err != nil {
			return errors.Wrapf(err, "failed to write cert %s", filename)
		}
	} else {
		fileBytes, keyBytes := splitCertKeyPEM(fileBytes)
		if err := ioutil.WriteFile(filename, fileBytes, 0600); err != nil {
			return errors.Wrapf(err, "failed to write cert %s", filename)
		}
		if err := ioutil.WriteFile(keyFile, keyBytes, 0600); err != nil {
			return errors.Wrapf(err, "failed to write key %s", filename)
		}
	}
	return nil
}

func splitCertKeyPEM(bytes []byte) (certPem []byte, keyPem []byte) {
	for {
		b, rest := pem.Decode(bytes)
		if b == nil {
			break
		}
		bytes = rest

		if strings.Contains(b.Type, "PRIVATE KEY") {
			keyPem = append(keyPem, pem.EncodeToMemory(b)...)
		} else {
			certPem = append(certPem, pem.EncodeToMemory(b)...)
		}
	}

	return
}

func getNodeNamedHostFile(filename, keyFile, nodeName, nodePasswordFile string, info *clientaccess.Info) error {
	basename := filepath.Base(filename)
	fileBytes, err := Request("/v1-"+version.Program+"/"+basename, info, getNodeNamedCrt(nodeName, nodePasswordFile))
	if err != nil {
		return err
	}
	fileBytes, keyBytes := splitCertKeyPEM(fileBytes)

	if err := ioutil.WriteFile(filename, fileBytes, 0600); err != nil {
		return errors.Wrapf(err, "failed to write cert %s", filename)
	}
	if err := ioutil.WriteFile(keyFile, keyBytes, 0600); err != nil {
		return errors.Wrapf(err, "failed to write key %s", filename)
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

	tmpConf := filepath.Join(os.TempDir(), version.Program+"-resolv.conf")
	if err := ioutil.WriteFile(tmpConf, []byte("nameserver 8.8.8.8\n"), 0444); err != nil {
		logrus.Error(err)
		return ""
	}
	return tmpConf
}

func get(envInfo *cmds.Agent, proxy proxy.Proxy) (*config.Node, error) {
	if envInfo.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	info, err := clientaccess.ParseAndValidateToken(proxy.SupervisorURL(), envInfo.Token)
	if err != nil {
		return nil, err
	}

	controlConfig, err := getConfig(info)
	if err != nil {
		return nil, err
	}

	if controlConfig.SupervisorPort != controlConfig.HTTPSPort {
		if err := proxy.StartAPIServerProxy(controlConfig.HTTPSPort); err != nil {
			return nil, errors.Wrapf(err, "failed to setup access to API Server port %d on at %s", controlConfig.HTTPSPort, proxy.SupervisorURL())
		}
	}

	var flannelIface *sysnet.Interface
	if !envInfo.NoFlannel && len(envInfo.FlannelIface) > 0 {
		flannelIface, err = sysnet.InterfaceByName(envInfo.FlannelIface)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to find interface")
		}
	}

	clientCAFile := filepath.Join(envInfo.DataDir, "client-ca.crt")
	if err := getHostFile(clientCAFile, "", info); err != nil {
		return nil, err
	}

	serverCAFile := filepath.Join(envInfo.DataDir, "server-ca.crt")
	if err := getHostFile(serverCAFile, "", info); err != nil {
		return nil, err
	}

	servingKubeletCert := filepath.Join(envInfo.DataDir, "serving-kubelet.crt")
	servingKubeletKey := filepath.Join(envInfo.DataDir, "serving-kubelet.key")

	nodePasswordRoot := "/"
	if envInfo.Rootless {
		nodePasswordRoot = envInfo.DataDir
	}
	nodeConfigPath := filepath.Join(nodePasswordRoot, "etc", "rancher", "node")
	if err := os.MkdirAll(nodeConfigPath, 0755); err != nil {
		return nil, err
	}

	oldNodePasswordFile := filepath.Join(envInfo.DataDir, "node-password.txt")
	newNodePasswordFile := filepath.Join(nodeConfigPath, "password")
	upgradeOldNodePasswordPath(oldNodePasswordFile, newNodePasswordFile)

	nodeName, nodeIP, err := getHostnameAndIP(*envInfo)
	if err != nil {
		return nil, err
	}

	if envInfo.WithNodeID {
		nodeID, err := ensureNodeID(filepath.Join(nodeConfigPath, "id"))
		if err != nil {
			return nil, err
		}
		nodeName += "-" + nodeID
	}

	servingCert, err := getServingCert(nodeName, servingKubeletCert, servingKubeletKey, newNodePasswordFile, info)
	if err != nil {
		return nil, err
	}

	clientKubeletCert := filepath.Join(envInfo.DataDir, "client-kubelet.crt")
	clientKubeletKey := filepath.Join(envInfo.DataDir, "client-kubelet.key")
	if err := getNodeNamedHostFile(clientKubeletCert, clientKubeletKey, nodeName, newNodePasswordFile, info); err != nil {
		return nil, err
	}

	kubeconfigKubelet := filepath.Join(envInfo.DataDir, "kubelet.kubeconfig")
	if err := control.KubeConfig(kubeconfigKubelet, proxy.APIServerURL(), serverCAFile, clientKubeletCert, clientKubeletKey); err != nil {
		return nil, err
	}

	clientKubeProxyCert := filepath.Join(envInfo.DataDir, "client-kube-proxy.crt")
	clientKubeProxyKey := filepath.Join(envInfo.DataDir, "client-kube-proxy.key")
	if err := getHostFile(clientKubeProxyCert, clientKubeProxyKey, info); err != nil {
		return nil, err
	}

	kubeconfigKubeproxy := filepath.Join(envInfo.DataDir, "kubeproxy.kubeconfig")
	if err := control.KubeConfig(kubeconfigKubeproxy, proxy.APIServerURL(), serverCAFile, clientKubeProxyCert, clientKubeProxyKey); err != nil {
		return nil, err
	}

	clientK3sControllerCert := filepath.Join(envInfo.DataDir, "client-"+version.Program+"-controller.crt")
	clientK3sControllerKey := filepath.Join(envInfo.DataDir, "client-"+version.Program+"-controller.key")
	if err := getHostFile(clientK3sControllerCert, clientK3sControllerKey, info); err != nil {
		return nil, err
	}

	kubeconfigK3sController := filepath.Join(envInfo.DataDir, version.Program+"controller.kubeconfig")
	if err := control.KubeConfig(kubeconfigK3sController, proxy.APIServerURL(), serverCAFile, clientK3sControllerCert, clientK3sControllerKey); err != nil {
		return nil, err
	}

	nodeConfig := &config.Node{
		Docker:                   envInfo.Docker,
		DisableSELinux:           envInfo.DisableSELinux,
		ContainerRuntimeEndpoint: envInfo.ContainerRuntimeEndpoint,
		FlannelBackend:           controlConfig.FlannelBackend,
	}
	nodeConfig.FlannelIface = flannelIface
	nodeConfig.Images = filepath.Join(envInfo.DataDir, "images")
	nodeConfig.AgentConfig.NodeIP = nodeIP
	nodeConfig.AgentConfig.NodeName = nodeName
	nodeConfig.AgentConfig.NodeConfigPath = nodeConfigPath
	nodeConfig.AgentConfig.NodeExternalIP = envInfo.NodeExternalIP
	nodeConfig.AgentConfig.ServingKubeletCert = servingKubeletCert
	nodeConfig.AgentConfig.ServingKubeletKey = servingKubeletKey
	nodeConfig.AgentConfig.ClusterDNS = controlConfig.ClusterDNS
	nodeConfig.AgentConfig.ClusterDomain = controlConfig.ClusterDomain
	nodeConfig.AgentConfig.ResolvConf = locateOrGenerateResolvConf(envInfo)
	nodeConfig.AgentConfig.ClientCA = clientCAFile
	nodeConfig.AgentConfig.ListenAddress = "0.0.0.0"
	nodeConfig.AgentConfig.KubeConfigKubelet = kubeconfigKubelet
	nodeConfig.AgentConfig.KubeConfigKubeProxy = kubeconfigKubeproxy
	nodeConfig.AgentConfig.KubeConfigK3sController = kubeconfigK3sController
	if envInfo.Rootless {
		nodeConfig.AgentConfig.RootDir = filepath.Join(envInfo.DataDir, "kubelet")
	}
	nodeConfig.AgentConfig.PauseImage = envInfo.PauseImage
	nodeConfig.AgentConfig.Snapshotter = envInfo.Snapshotter
	nodeConfig.AgentConfig.IPSECPSK = controlConfig.IPSECPSK
	nodeConfig.AgentConfig.StrongSwanDir = filepath.Join(envInfo.DataDir, "strongswan")
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
	nodeConfig.Certificate = servingCert

	if nodeConfig.FlannelBackend == config.FlannelBackendNone {
		nodeConfig.NoFlannel = true
	} else {
		nodeConfig.NoFlannel = envInfo.NoFlannel
	}

	if !nodeConfig.NoFlannel {
		hostLocal, err := exec.LookPath("host-local")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to find host-local")
		}

		if envInfo.FlannelConf == "" {
			nodeConfig.FlannelConf = filepath.Join(envInfo.DataDir, "etc/flannel/net-conf.json")
		} else {
			nodeConfig.FlannelConf = envInfo.FlannelConf
			nodeConfig.FlannelConfOverride = true
		}
		nodeConfig.AgentConfig.CNIBinDir = filepath.Dir(hostLocal)
		nodeConfig.AgentConfig.CNIConfDir = filepath.Join(envInfo.DataDir, "etc/cni/net.d")
	}

	if !nodeConfig.Docker && nodeConfig.ContainerRuntimeEndpoint == "" {
		nodeConfig.AgentConfig.RuntimeSocket = nodeConfig.Containerd.Address
	} else {
		nodeConfig.AgentConfig.RuntimeSocket = nodeConfig.ContainerRuntimeEndpoint
		nodeConfig.AgentConfig.CNIPlugin = true
	}

	if controlConfig.ClusterIPRange != nil {
		nodeConfig.AgentConfig.ClusterCIDR = *controlConfig.ClusterIPRange
	}

	os.Setenv("NODE_NAME", nodeConfig.AgentConfig.NodeName)

	nodeConfig.AgentConfig.ExtraKubeletArgs = envInfo.ExtraKubeletArgs
	nodeConfig.AgentConfig.ExtraKubeProxyArgs = envInfo.ExtraKubeProxyArgs

	nodeConfig.AgentConfig.NodeTaints = envInfo.Taints
	nodeConfig.AgentConfig.NodeLabels = envInfo.Labels
	nodeConfig.AgentConfig.PrivateRegistry = envInfo.PrivateRegistry
	nodeConfig.AgentConfig.DisableCCM = controlConfig.DisableCCM
	nodeConfig.AgentConfig.DisableNPC = controlConfig.DisableNPC
	nodeConfig.AgentConfig.DisableKubeProxy = controlConfig.DisableKubeProxy
	nodeConfig.AgentConfig.Rootless = envInfo.Rootless
	nodeConfig.AgentConfig.PodManifests = filepath.Join(envInfo.DataDir, DefaultPodManifestPath)
	nodeConfig.DisableSELinux = envInfo.DisableSELinux
	nodeConfig.AgentConfig.ProtectKernelDefaults = envInfo.ProtectKernelDefaults

	return nodeConfig, nil
}

func getConfig(info *clientaccess.Info) (*config.Control, error) {
	data, err := clientaccess.Get("/v1-"+version.Program+"/config", info)
	if err != nil {
		return nil, err
	}

	controlControl := &config.Control{}
	return controlControl, json.Unmarshal(data, controlControl)
}
