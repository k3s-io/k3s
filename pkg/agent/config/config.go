package config

import (
	"bufio"
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/k3s-io/k3s/pkg/agent/proxy"
	agentutil "github.com/k3s-io/k3s/pkg/agent/util"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/clientaccess"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/control/deps"
	"github.com/k3s-io/k3s/pkg/spegel"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/k3s-io/k3s/pkg/vpn"
	"github.com/pkg/errors"
	certutil "github.com/rancher/dynamiclistener/cert"
	"github.com/rancher/wharfie/pkg/registries"
	"github.com/rancher/wrangler/v3/pkg/slice"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/wait"
	utilsnet "k8s.io/utils/net"
)

const (
	DefaultPodManifestPath = "pod-manifests"
)

// Get returns a pointer to a completed Node configuration struct,
// containing a merging of the local CLI configuration with settings from the server.
// Node configuration includes client certificates, which requires node password verification,
// so this is somewhat computationally expensive on the server side, and is retried with jitter
// to avoid having clients hammer on the server at fixed periods.
// A call to this will bock until agent configuration is successfully returned by the
// server, or the context is cancelled.
func Get(ctx context.Context, agent cmds.Agent, proxy proxy.Proxy) (*config.Node, error) {
	var agentConfig *config.Node
	var err error

	// This would be more clear as wait.PollImmediateUntilWithContext, but that function
	// does not support jittering, so we instead use wait.JitterUntilWithContext, and cancel
	// the context on success.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	wait.JitterUntilWithContext(ctx, func(ctx context.Context) {
		agentConfig, err = get(ctx, &agent, proxy)
		if err != nil {
			logrus.Infof("Waiting to retrieve agent configuration; server is not ready: %v", err)
		} else {
			cancel()
		}
	}, 5*time.Second, 1.0, true)
	return agentConfig, err
}

// KubeProxyDisabled returns a bool indicating whether or not kube-proxy has been disabled in the
// server configuration. The server may not have a complete view of cluster configuration until
// after all startup hooks have completed, so a call to this will block until after the server's
// readyz endpoint returns OK.
func KubeProxyDisabled(ctx context.Context, node *config.Node, proxy proxy.Proxy) bool {
	var disabled bool
	var err error

	_ = wait.PollUntilContextCancel(ctx, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		disabled, err = getKubeProxyDisabled(ctx, node, proxy)
		if err != nil {
			logrus.Infof("Waiting to retrieve kube-proxy configuration; server is not ready: %v", err)
			return false, nil
		}
		return true, nil
	})
	return disabled
}

// WaitForAPIServers returns a list of apiserver endpoints, suitable for seeding client loadbalancer configurations.
// This function will block until it can return a populated list of apiservers, or if the remote server returns
// an error (indicating that it does not support this functionality).
func WaitForAPIServers(ctx context.Context, node *config.Node, proxy proxy.Proxy) []string {
	var addresses []string
	var info *clientaccess.Info
	var err error

	_ = wait.PollUntilContextCancel(ctx, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		if info == nil {
			withCert := clientaccess.WithClientCertificate(node.AgentConfig.ClientKubeletCert, node.AgentConfig.ClientKubeletKey)
			info, err = clientaccess.ParseAndValidateToken(proxy.SupervisorURL(), node.Token, withCert)
			if err != nil {
				logrus.Warnf("Failed to validate server token: %v", err)
				return false, nil
			}
		}
		addresses, err = GetAPIServers(ctx, info)
		if err != nil {
			logrus.Infof("Failed to retrieve list of apiservers from server: %v", err)
			return false, err
		}
		if len(addresses) == 0 {
			logrus.Infof("Waiting for supervisor to provide apiserver addresses")
			return false, nil
		}
		return true, nil
	})
	return addresses
}

type HTTPRequester func(u string, client *http.Client, username, password, token string) ([]byte, error)

func Request(path string, info *clientaccess.Info, requester HTTPRequester) ([]byte, error) {
	u, err := url.Parse(info.BaseURL)
	if err != nil {
		return nil, err
	}
	u.Path = path
	return requester(u.String(), clientaccess.GetHTTPClient(info.CACerts, info.CertFile, info.KeyFile), info.Username, info.Password, info.Token())
}

func getNodeNamedCrt(nodeName string, nodeIPs []net.IP, nodePasswordFile string, csr []byte) HTTPRequester {
	return func(u string, client *http.Client, username, password, token string) ([]byte, error) {
		req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(csr))
		if err != nil {
			return nil, err
		}

		if token != "" {
			req.Header.Add("Authorization", "Bearer "+token)
		} else if username != "" {
			req.SetBasicAuth(username, password)
		}

		req.Header.Set(version.Program+"-Node-Name", nodeName)
		nodePassword, err := ensureNodePassword(nodePasswordFile)
		if err != nil {
			return nil, err
		}
		req.Header.Set(version.Program+"-Node-Password", nodePassword)
		req.Header.Set(version.Program+"-Node-IP", util.JoinIPs(nodeIPs))

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		// If we got a 401 Unauthorized response when using client certs, try again without client cert auth.
		// This allows us to fall back from node identity to token when the node resource is deleted.
		if resp.StatusCode == http.StatusUnauthorized {
			if transport, ok := client.Transport.(*http.Transport); ok && transport.TLSClientConfig != nil && len(transport.TLSClientConfig.Certificates) != 0 {
				logrus.Infof("Node authorization rejected, retrying without client certificate authentication")
				transport.TLSClientConfig.Certificates = []tls.Certificate{}
				resp, err = client.Do(req)
				if err != nil {
					return nil, err
				}
				defer resp.Body.Close()
			}
		}

		if resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("Node password rejected, duplicate hostname or contents of '%s' may not match server node-passwd entry, try enabling a unique node name with the --with-node-id flag", nodePasswordFile)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%s: %s", u, resp.Status)
		}

		return io.ReadAll(resp.Body)
	}
}

func ensureNodeID(nodeIDFile string) (string, error) {
	if _, err := os.Stat(nodeIDFile); err == nil {
		id, err := os.ReadFile(nodeIDFile)
		return strings.TrimSpace(string(id)), err
	}
	id := make([]byte, 4, 4)
	_, err := cryptorand.Read(id)
	if err != nil {
		return "", err
	}
	nodeID := hex.EncodeToString(id)
	return nodeID, os.WriteFile(nodeIDFile, []byte(nodeID+"\n"), 0644)
}

func ensureNodePassword(nodePasswordFile string) (string, error) {
	if _, err := os.Stat(nodePasswordFile); err == nil {
		password, err := os.ReadFile(nodePasswordFile)
		return strings.TrimSpace(string(password)), err
	}
	password := make([]byte, 16, 16)
	_, err := cryptorand.Read(password)
	if err != nil {
		return "", err
	}
	nodePassword := hex.EncodeToString(password)

	if err = os.WriteFile(nodePasswordFile, []byte(nodePassword+"\n"), 0600); err != nil {
		return nodePassword, err
	}

	if err = configureACL(nodePasswordFile); err != nil {
		return nodePassword, err
	}

	return nodePassword, nil
}

func upgradeOldNodePasswordPath(oldNodePasswordFile, newNodePasswordFile string) {
	password, err := os.ReadFile(oldNodePasswordFile)
	if err != nil {
		return
	}
	if err := os.WriteFile(newNodePasswordFile, password, 0600); err != nil {
		logrus.Warnf("Unable to write password file: %v", err)
		return
	}
	if err := os.Remove(oldNodePasswordFile); err != nil {
		logrus.Warnf("Unable to remove old password file: %v", err)
		return
	}
}

// getKubeletServingCert fills the kubelet server certificate with content returned
// from the server.  We attempt to POST a CSR to the server, in hopes that it will
// sign the cert using our locally generated key. If the server does not support CSR
// signing, the key generated by the server is used instead.
func getKubeletServingCert(nodeName string, nodeIPs []net.IP, certFile, keyFile, nodePasswordFile string, info *clientaccess.Info) error {
	csr, err := getCSRBytes(keyFile)
	if err != nil {
		return errors.Wrapf(err, "failed to create certificate request %s", certFile)
	}

	basename := filepath.Base(certFile)
	body, err := Request("/v1-"+version.Program+"/"+basename, info, getNodeNamedCrt(nodeName, nodeIPs, nodePasswordFile, csr))
	if err != nil {
		return err
	}

	// Always split the response, as down-level servers may send back a cert+key
	// instead of signing a new cert with our key.  If the response includes a key it
	// must be used instead of the one we signed the CSR with.
	certBytes, keyBytes := splitCertKeyPEM(body)
	if err := os.WriteFile(certFile, certBytes, 0600); err != nil {
		return errors.Wrapf(err, "failed to write cert %s", certFile)
	}
	if len(keyBytes) > 0 {
		if err := os.WriteFile(keyFile, keyBytes, 0600); err != nil {
			return errors.Wrapf(err, "failed to write key %s", keyFile)
		}
	}
	return nil
}

// getHostFile fills a file with content returned from the server.
func getHostFile(filename string, info *clientaccess.Info) error {
	basename := filepath.Base(filename)
	fileBytes, err := info.Get("/v1-" + version.Program + "/" + basename)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filename, fileBytes, 0600); err != nil {
		return errors.Wrapf(err, "failed to write cert %s", filename)
	}
	return nil
}

// getClientCert fills a client certificate with content returned from the server.
// We attempt to POST a CSR to the server, in hopes that it will sign the cert using
// our locally generated key. If the server does not support CSR signing, the key
// generated by the server is used instead.
func getClientCert(certFile, keyFile string, info *clientaccess.Info) error {
	csr, err := getCSRBytes(keyFile)
	if err != nil {
		return errors.Wrapf(err, "failed to create certificate request %s", certFile)
	}

	basename := filepath.Base(certFile)
	fileBytes, err := info.Post("/v1-"+version.Program+"/"+basename, csr)
	if err != nil {
		return err
	}

	// Always split the response, as down-level servers may send back a cert+key
	// instead of signing a new cert with our key.  If the response includes a key it
	// must be used instead of the one we signed the CSR with.
	certBytes, keyBytes := splitCertKeyPEM(fileBytes)
	if err := os.WriteFile(certFile, certBytes, 0600); err != nil {
		return errors.Wrapf(err, "failed to write cert %s", certFile)
	}
	if len(keyBytes) > 0 {
		if err := os.WriteFile(keyFile, keyBytes, 0600); err != nil {
			return errors.Wrapf(err, "failed to write key %s", keyFile)
		}
	}
	return nil
}

func getCSRBytes(keyFile string) ([]byte, error) {
	keyBytes, _, err := certutil.LoadOrGenerateKeyFile(keyFile, false)
	if err != nil {
		return nil, err
	}
	key, err := certutil.ParsePrivateKeyPEM(keyBytes)
	if err != nil {
		return nil, err
	}
	return x509.CreateCertificateRequest(cryptorand.Reader, &x509.CertificateRequest{}, key)
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

// getKubeletClientCert fills the kubelet client certificate with content returned
// from the server.  We attempt to POST a CSR to the server, in hopes that it will
// sign the cert using our locally generated key. If the server does not support CSR
// signing, the key generated by the server is used instead.
func getKubeletClientCert(certFile, keyFile, nodeName string, nodeIPs []net.IP, nodePasswordFile string, info *clientaccess.Info) error {
	csr, err := getCSRBytes(keyFile)
	if err != nil {
		return errors.Wrapf(err, "failed to create certificate request %s", certFile)
	}

	basename := filepath.Base(certFile)
	body, err := Request("/v1-"+version.Program+"/"+basename, info, getNodeNamedCrt(nodeName, nodeIPs, nodePasswordFile, csr))
	if err != nil {
		return err
	}

	// Always split the response, as down-level servers may send back a cert+key
	// instead of signing a new cert with our key.  If the response includes a key it
	// must be used instead of the one we signed the CSR with.
	certBytes, keyBytes := splitCertKeyPEM(body)
	if err := os.WriteFile(certFile, certBytes, 0600); err != nil {
		return errors.Wrapf(err, "failed to write cert %s", certFile)
	}
	if len(keyBytes) > 0 {
		if err := os.WriteFile(keyFile, keyBytes, 0600); err != nil {
			return errors.Wrapf(err, "failed to write key %s", keyFile)
		}
	}
	return nil
}

func isValidResolvConf(resolvConfFile string) bool {
	file, err := os.Open(resolvConfFile)
	if err != nil {
		return false
	}
	defer file.Close()

	nameserver := regexp.MustCompile(`^nameserver\s+([^\s]*)`)
	scanner := bufio.NewScanner(file)
	foundNameserver := false
	for scanner.Scan() {
		ipMatch := nameserver.FindStringSubmatch(scanner.Text())
		if len(ipMatch) == 2 {
			ip := net.ParseIP(ipMatch[1])
			if ip == nil || !ip.IsGlobalUnicast() {
				return false
			} else {
				foundNameserver = true
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return false
	}
	return foundNameserver
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

	resolvConf := filepath.Join(envInfo.DataDir, "agent", "etc", "resolv.conf")
	if err := agentutil.WriteFile(resolvConf, "nameserver 8.8.8.8\n"); err != nil {
		logrus.Errorf("Failed to write %s: %v", resolvConf, err)
		return ""
	}
	logrus.Warnf("Host resolv.conf includes loopback or multicast nameservers - kubelet will use autogenerated resolv.conf with nameserver 8.8.8.8")
	return resolvConf
}

func get(ctx context.Context, envInfo *cmds.Agent, proxy proxy.Proxy) (*config.Node, error) {
	if envInfo.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
	clientKubeletCert := filepath.Join(envInfo.DataDir, "agent", "client-kubelet.crt")
	clientKubeletKey := filepath.Join(envInfo.DataDir, "agent", "client-kubelet.key")
	withCert := clientaccess.WithClientCertificate(clientKubeletCert, clientKubeletKey)
	info, err := clientaccess.ParseAndValidateToken(proxy.SupervisorURL(), envInfo.Token, withCert)
	if err != nil {
		return nil, err
	}

	controlConfig, err := getConfig(info)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve configuration from server")
	}
	// If the supervisor and externally-facing apiserver are not on the same port, tell the proxy where to find the apiserver.
	if controlConfig.SupervisorPort != controlConfig.HTTPSPort {
		isIPv6 := utilsnet.IsIPv6(net.ParseIP(util.GetFirstValidIPString(envInfo.NodeIP)))
		if err := proxy.SetAPIServerPort(controlConfig.HTTPSPort, isIPv6); err != nil {
			return nil, errors.Wrapf(err, "failed to set apiserver port to %d", controlConfig.HTTPSPort)
		}
	}
	apiServerURL := proxy.APIServerURL()

	var flannelIface *net.Interface
	if controlConfig.FlannelBackend != config.FlannelBackendNone && len(envInfo.FlannelIface) > 0 {
		flannelIface, err = net.InterfaceByName(envInfo.FlannelIface)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to find interface %s", envInfo.FlannelIface)
		}
	}

	clientCAFile := filepath.Join(envInfo.DataDir, "agent", "client-ca.crt")
	if err := getHostFile(clientCAFile, info); err != nil {
		return nil, err
	}

	serverCAFile := filepath.Join(envInfo.DataDir, "agent", "server-ca.crt")
	if err := getHostFile(serverCAFile, info); err != nil {
		return nil, err
	}

	servingKubeletCert := filepath.Join(envInfo.DataDir, "agent", "serving-kubelet.crt")
	servingKubeletKey := filepath.Join(envInfo.DataDir, "agent", "serving-kubelet.key")

	nodePasswordRoot := "/"
	if envInfo.Rootless {
		nodePasswordRoot = filepath.Join(envInfo.DataDir, "agent")
	}
	nodeConfigPath := filepath.Join(nodePasswordRoot, "etc", "rancher", "node")
	if err := os.MkdirAll(nodeConfigPath, 0755); err != nil {
		return nil, err
	}

	oldNodePasswordFile := filepath.Join(envInfo.DataDir, "agent", "node-password.txt")
	newNodePasswordFile := filepath.Join(nodeConfigPath, "password")
	upgradeOldNodePasswordPath(oldNodePasswordFile, newNodePasswordFile)

	nodeName, nodeIPs, err := util.GetHostnameAndIPs(envInfo.NodeName, envInfo.NodeIP)
	if err != nil {
		return nil, err
	}

	// If there is a VPN, we must overwrite NodeIP and flannel interface
	var vpnInfo vpn.VPNInfo
	if envInfo.VPNAuth != "" {
		vpnInfo, err = vpn.GetVPNInfo(envInfo.VPNAuth)
		if err != nil {
			return nil, err
		}

		// Pass ipv4, ipv6 or both depending on nodeIPs mode
		var vpnIPs []net.IP
		if utilsnet.IsIPv4(nodeIPs[0]) && vpnInfo.IPv4Address != nil {
			vpnIPs = append(vpnIPs, vpnInfo.IPv4Address)
			if vpnInfo.IPv6Address != nil {
				vpnIPs = append(vpnIPs, vpnInfo.IPv6Address)
			}
		} else if utilsnet.IsIPv6(nodeIPs[0]) && vpnInfo.IPv6Address != nil {
			vpnIPs = append(vpnIPs, vpnInfo.IPv6Address)
			if vpnInfo.IPv4Address != nil {
				vpnIPs = append(vpnIPs, vpnInfo.IPv4Address)
			}
		} else {
			return nil, errors.Errorf("address family mismatch when assigning VPN addresses to node: node=%v, VPN ipv4=%v ipv6=%v", nodeIPs, vpnInfo.IPv4Address, vpnInfo.IPv6Address)
		}

		// Overwrite nodeip and flannel interface and throw a warning if user explicitly set those parameters
		if len(vpnIPs) != 0 {
			logrus.Infof("Node-ip changed to %v due to VPN", vpnIPs)
			if len(envInfo.NodeIP) != 0 {
				logrus.Warn("VPN provider overrides configured node-ip parameter")
			}
			if len(envInfo.NodeExternalIP) != 0 {
				logrus.Warn("VPN provider overrides node-external-ip parameter")
			}
			nodeIPs = vpnIPs
			flannelIface, err = net.InterfaceByName(vpnInfo.VPNInterface)
			if err != nil {
				return nil, errors.Wrapf(err, "unable to find vpn interface: %s", vpnInfo.VPNInterface)
			}
		}
	}

	if controlConfig.ClusterIPRange != nil {
		if utilsnet.IPFamilyOfCIDR(controlConfig.ClusterIPRange) != utilsnet.IPFamilyOf(nodeIPs[0]) && len(nodeIPs) > 1 {
			firstNodeIP := nodeIPs[0]
			nodeIPs[0] = nodeIPs[1]
			nodeIPs[1] = firstNodeIP
		}
	}

	nodeExternalIPs, err := util.ParseStringSliceToIPs(envInfo.NodeExternalIP)
	if err != nil {
		return nil, fmt.Errorf("invalid node-external-ip: %w", err)
	}

	if envInfo.WithNodeID {
		nodeID, err := ensureNodeID(filepath.Join(nodeConfigPath, "id"))
		if err != nil {
			return nil, err
		}
		nodeName += "-" + nodeID
	}

	os.Setenv("NODE_NAME", nodeName)

	// Ensure that the kubelet's server certificate is valid for all configured node IPs.  Note
	// that in the case of an external CCM, additional IPs may be added by the infra provider
	// that the cert will not be valid for, as they are not present in the list collected here.
	nodeExternalAndInternalIPs := append(nodeIPs, nodeExternalIPs...)

	// Ask the server to sign our kubelet server cert.
	if err := getKubeletServingCert(nodeName, nodeExternalAndInternalIPs, servingKubeletCert, servingKubeletKey, newNodePasswordFile, info); err != nil {
		return nil, errors.Wrap(err, servingKubeletCert)
	}

	// Ask the server to sign our kubelet client cert.
	if err := getKubeletClientCert(clientKubeletCert, clientKubeletKey, nodeName, nodeIPs, newNodePasswordFile, info); err != nil {
		return nil, errors.Wrap(err, clientKubeletCert)
	}

	// Generate a kubeconfig for the kubelet.
	kubeconfigKubelet := filepath.Join(envInfo.DataDir, "agent", "kubelet.kubeconfig")
	if err := deps.KubeConfig(kubeconfigKubelet, apiServerURL, serverCAFile, clientKubeletCert, clientKubeletKey); err != nil {
		return nil, err
	}

	clientKubeProxyCert := filepath.Join(envInfo.DataDir, "agent", "client-kube-proxy.crt")
	clientKubeProxyKey := filepath.Join(envInfo.DataDir, "agent", "client-kube-proxy.key")

	// Ask the server to sign our kube-proxy client cert.
	if err := getClientCert(clientKubeProxyCert, clientKubeProxyKey, info); err != nil {
		return nil, errors.Wrap(err, clientKubeProxyCert)
	}

	// Generate a kubeconfig for kube-proxy.
	kubeconfigKubeproxy := filepath.Join(envInfo.DataDir, "agent", "kubeproxy.kubeconfig")
	if err := deps.KubeConfig(kubeconfigKubeproxy, apiServerURL, serverCAFile, clientKubeProxyCert, clientKubeProxyKey); err != nil {
		return nil, err
	}

	clientK3sControllerCert := filepath.Join(envInfo.DataDir, "agent", "client-"+version.Program+"-controller.crt")
	clientK3sControllerKey := filepath.Join(envInfo.DataDir, "agent", "client-"+version.Program+"-controller.key")

	// Ask the server to sign our agent controller client cert.
	if err := getClientCert(clientK3sControllerCert, clientK3sControllerKey, info); err != nil {
		return nil, errors.Wrap(err, clientK3sControllerCert)
	}

	// Generate a kubeconfig for the agent controller.
	kubeconfigK3sController := filepath.Join(envInfo.DataDir, "agent", version.Program+"controller.kubeconfig")
	if err := deps.KubeConfig(kubeconfigK3sController, apiServerURL, serverCAFile, clientK3sControllerCert, clientK3sControllerKey); err != nil {
		return nil, err
	}

	// Ensure kubelet config dir exists
	kubeletConfigDir := filepath.Join(envInfo.DataDir, "agent", "etc", "kubelet.conf.d")
	if err := os.MkdirAll(kubeletConfigDir, 0700); err != nil {
		return nil, err
	}

	nodeConfig := &config.Node{
		Docker:                   envInfo.Docker,
		SELinux:                  envInfo.EnableSELinux,
		ContainerRuntimeEndpoint: envInfo.ContainerRuntimeEndpoint,
		ImageServiceEndpoint:     envInfo.ImageServiceEndpoint,
		EnablePProf:              envInfo.EnablePProf,
		EmbeddedRegistry:         controlConfig.EmbeddedRegistry,
		FlannelBackend:           controlConfig.FlannelBackend,
		FlannelIPv6Masq:          controlConfig.FlannelIPv6Masq,
		FlannelExternalIP:        controlConfig.FlannelExternalIP,
		EgressSelectorMode:       controlConfig.EgressSelectorMode,
		ServerHTTPSPort:          controlConfig.HTTPSPort,
		SupervisorPort:           controlConfig.SupervisorPort,
		SupervisorMetrics:        controlConfig.SupervisorMetrics,
		Token:                    info.String(),
	}
	nodeConfig.FlannelIface = flannelIface
	nodeConfig.Images = filepath.Join(envInfo.DataDir, "agent", "images")
	nodeConfig.AgentConfig.NodeName = nodeName
	nodeConfig.AgentConfig.NodeConfigPath = nodeConfigPath
	nodeConfig.AgentConfig.ClientKubeletCert = clientKubeletCert
	nodeConfig.AgentConfig.ClientKubeletKey = clientKubeletKey
	nodeConfig.AgentConfig.ServingKubeletCert = servingKubeletCert
	nodeConfig.AgentConfig.ServingKubeletKey = servingKubeletKey
	nodeConfig.AgentConfig.ClusterDNS = controlConfig.ClusterDNS
	nodeConfig.AgentConfig.ClusterDomain = controlConfig.ClusterDomain
	nodeConfig.AgentConfig.ResolvConf = locateOrGenerateResolvConf(envInfo)
	nodeConfig.AgentConfig.ClientCA = clientCAFile
	nodeConfig.AgentConfig.KubeletConfigDir = kubeletConfigDir
	nodeConfig.AgentConfig.KubeConfigKubelet = kubeconfigKubelet
	nodeConfig.AgentConfig.KubeConfigKubeProxy = kubeconfigKubeproxy
	nodeConfig.AgentConfig.KubeConfigK3sController = kubeconfigK3sController
	nodeConfig.AgentConfig.Snapshotter = envInfo.Snapshotter
	nodeConfig.AgentConfig.IPSECPSK = controlConfig.IPSECPSK
	nodeConfig.Containerd.Config = filepath.Join(envInfo.DataDir, "agent", "etc", "containerd", "config.toml")
	nodeConfig.Containerd.Root = filepath.Join(envInfo.DataDir, "agent", "containerd")
	nodeConfig.CRIDockerd.Root = filepath.Join(envInfo.DataDir, "agent", "cri-dockerd")
	nodeConfig.Containerd.Opt = filepath.Join(envInfo.DataDir, "agent", "containerd")
	nodeConfig.Containerd.Log = filepath.Join(envInfo.DataDir, "agent", "containerd", "containerd.log")
	nodeConfig.Containerd.Registry = filepath.Join(envInfo.DataDir, "agent", "etc", "containerd", "certs.d")
	nodeConfig.Containerd.NoDefault = envInfo.ContainerdNoDefault
	nodeConfig.Containerd.NonrootDevices = envInfo.ContainerdNonrootDevices
	nodeConfig.Containerd.Debug = envInfo.Debug
	nodeConfig.Containerd.Template = filepath.Join(envInfo.DataDir, "agent", "etc", "containerd")

	if envInfo.Rootless {
		nodeConfig.AgentConfig.RootDir = filepath.Join(envInfo.DataDir, "agent", "kubelet")
	}

	if envInfo.BindAddress != "" {
		nodeConfig.AgentConfig.ListenAddress = envInfo.BindAddress
	} else {
		listenAddress, _, _, err := util.GetDefaultAddresses(nodeIPs[0])
		if err != nil {
			return nil, errors.Wrap(err, "cannot configure IPv4/IPv6 node-ip")
		}
		nodeConfig.AgentConfig.ListenAddress = listenAddress
	}

	nodeConfig.AgentConfig.NodeIP = nodeIPs[0].String()
	nodeConfig.AgentConfig.NodeIPs = nodeIPs
	nodeConfig.AgentConfig.NodeExternalIPs = nodeExternalIPs

	// if configured, set NodeExternalIP to the first IPv4 address, for legacy clients
	// unless only IPv6 address given
	if len(nodeConfig.AgentConfig.NodeExternalIPs) > 0 {
		nodeConfig.AgentConfig.NodeExternalIP = nodeConfig.AgentConfig.NodeExternalIPs[0].String()
	}

	var nodeExternalDNSs []string
	for _, dnsString := range envInfo.NodeExternalDNS.Value() {
		nodeExternalDNSs = append(nodeExternalDNSs, strings.Split(dnsString, ",")...)
	}
	nodeConfig.AgentConfig.NodeExternalDNSs = nodeExternalDNSs

	var nodeInternalDNSs []string
	for _, dnsString := range envInfo.NodeInternalDNS.Value() {
		nodeInternalDNSs = append(nodeInternalDNSs, strings.Split(dnsString, ",")...)
	}
	nodeConfig.AgentConfig.NodeInternalDNSs = nodeInternalDNSs

	nodeConfig.NoFlannel = nodeConfig.FlannelBackend == config.FlannelBackendNone
	if !nodeConfig.NoFlannel {
		hostLocal, err := exec.LookPath("host-local")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to find host-local")
		}

		if envInfo.FlannelConf == "" {
			nodeConfig.FlannelConfFile = filepath.Join(envInfo.DataDir, "agent", "etc", "flannel", "net-conf.json")
		} else {
			nodeConfig.FlannelConfFile = envInfo.FlannelConf
			nodeConfig.FlannelConfOverride = true
		}
		nodeConfig.AgentConfig.CNIBinDir = filepath.Dir(hostLocal)
		nodeConfig.AgentConfig.CNIConfDir = filepath.Join(envInfo.DataDir, "agent", "etc", "cni", "net.d")
		nodeConfig.AgentConfig.FlannelCniConfFile = envInfo.FlannelCniConfFile

		// It does not make sense to use VPN without its flannel backend
		if envInfo.VPNAuth != "" {
			nodeConfig.FlannelBackend = vpnInfo.ProviderName
		}
	}

	if nodeConfig.ImageServiceEndpoint != "" {
		nodeConfig.AgentConfig.ImageServiceSocket = nodeConfig.ImageServiceEndpoint
	}

	if nodeConfig.ContainerRuntimeEndpoint != "" {
		nodeConfig.AgentConfig.RuntimeSocket = nodeConfig.ContainerRuntimeEndpoint
	} else if nodeConfig.Docker {
		if err := applyCRIDockerdOSSpecificConfig(nodeConfig); err != nil {
			return nil, err
		}
		nodeConfig.AgentConfig.CNIPlugin = true
		nodeConfig.AgentConfig.RuntimeSocket = nodeConfig.CRIDockerd.Address
	} else {
		if err := applyContainerdOSSpecificConfig(nodeConfig); err != nil {
			return nil, err
		}
		if err := applyContainerdQoSClassConfigFileIfPresent(envInfo, &nodeConfig.Containerd); err != nil {
			return nil, err
		}
		nodeConfig.AgentConfig.RuntimeSocket = nodeConfig.Containerd.Address
	}

	if controlConfig.ClusterIPRange != nil {
		nodeConfig.AgentConfig.ClusterCIDR = controlConfig.ClusterIPRange
		nodeConfig.AgentConfig.ClusterCIDRs = []*net.IPNet{controlConfig.ClusterIPRange}
	}

	if len(controlConfig.ClusterIPRanges) > 0 {
		nodeConfig.AgentConfig.ClusterCIDRs = controlConfig.ClusterIPRanges
	}

	if controlConfig.ServiceIPRange != nil {
		nodeConfig.AgentConfig.ServiceCIDR = controlConfig.ServiceIPRange
		nodeConfig.AgentConfig.ServiceCIDRs = []*net.IPNet{controlConfig.ServiceIPRange}
	}

	if len(controlConfig.ServiceIPRanges) > 0 {
		nodeConfig.AgentConfig.ServiceCIDRs = controlConfig.ServiceIPRanges
	}

	if controlConfig.ServiceNodePortRange != nil {
		nodeConfig.AgentConfig.ServiceNodePortRange = *controlConfig.ServiceNodePortRange
	}

	if len(controlConfig.ClusterDNSs) == 0 {
		nodeConfig.AgentConfig.ClusterDNSs = []net.IP{controlConfig.ClusterDNS}
	} else {
		nodeConfig.AgentConfig.ClusterDNSs = controlConfig.ClusterDNSs
	}

	nodeConfig.AgentConfig.PauseImage = envInfo.PauseImage
	nodeConfig.AgentConfig.AirgapExtraRegistry = envInfo.AirgapExtraRegistry
	nodeConfig.AgentConfig.SystemDefaultRegistry = controlConfig.SystemDefaultRegistry

	// Apply SystemDefaultRegistry to PauseImage and AirgapExtraRegistry
	if controlConfig.SystemDefaultRegistry != "" {
		if nodeConfig.AgentConfig.PauseImage != "" && !strings.HasPrefix(nodeConfig.AgentConfig.PauseImage, controlConfig.SystemDefaultRegistry) {
			nodeConfig.AgentConfig.PauseImage = controlConfig.SystemDefaultRegistry + "/" + nodeConfig.AgentConfig.PauseImage
		}
		if !slice.ContainsString(nodeConfig.AgentConfig.AirgapExtraRegistry, controlConfig.SystemDefaultRegistry) {
			nodeConfig.AgentConfig.AirgapExtraRegistry = append(nodeConfig.AgentConfig.AirgapExtraRegistry, controlConfig.SystemDefaultRegistry)
		}
	}

	nodeConfig.AgentConfig.ExtraKubeletArgs = envInfo.ExtraKubeletArgs
	nodeConfig.AgentConfig.ExtraKubeProxyArgs = envInfo.ExtraKubeProxyArgs
	nodeConfig.AgentConfig.NodeTaints = envInfo.Taints
	nodeConfig.AgentConfig.NodeLabels = envInfo.Labels
	nodeConfig.AgentConfig.ImageCredProvBinDir = envInfo.ImageCredProvBinDir
	nodeConfig.AgentConfig.ImageCredProvConfig = envInfo.ImageCredProvConfig
	nodeConfig.AgentConfig.DisableCCM = controlConfig.DisableCCM
	nodeConfig.AgentConfig.DisableNPC = controlConfig.DisableNPC
	nodeConfig.AgentConfig.MinTLSVersion = controlConfig.MinTLSVersion
	nodeConfig.AgentConfig.CipherSuites = controlConfig.CipherSuites
	nodeConfig.AgentConfig.Rootless = envInfo.Rootless
	nodeConfig.AgentConfig.PodManifests = filepath.Join(envInfo.DataDir, "agent", DefaultPodManifestPath)
	nodeConfig.AgentConfig.ProtectKernelDefaults = envInfo.ProtectKernelDefaults
	nodeConfig.AgentConfig.DisableServiceLB = envInfo.DisableServiceLB
	nodeConfig.AgentConfig.VLevel = cmds.LogConfig.VLevel
	nodeConfig.AgentConfig.VModule = cmds.LogConfig.VModule
	nodeConfig.AgentConfig.LogFile = cmds.LogConfig.LogFile
	nodeConfig.AgentConfig.AlsoLogToStderr = cmds.LogConfig.AlsoLogToStderr

	privRegistries, err := registries.GetPrivateRegistries(envInfo.PrivateRegistry)
	if err != nil {
		return nil, err
	}
	nodeConfig.AgentConfig.Registry = privRegistries.Registry

	if nodeConfig.EmbeddedRegistry {
		psk, err := hex.DecodeString(controlConfig.IPSECPSK)
		if err != nil {
			return nil, err
		}
		if len(psk) < 32 {
			return nil, errors.New("insufficient PSK bytes")
		}

		conf := spegel.DefaultRegistry
		conf.ExternalAddress = nodeConfig.AgentConfig.NodeIP
		conf.InternalAddress = controlConfig.Loopback(false)
		conf.RegistryPort = strconv.Itoa(controlConfig.SupervisorPort)
		conf.ClientCAFile = clientCAFile
		conf.ClientCertFile = clientK3sControllerCert
		conf.ClientKeyFile = clientK3sControllerKey
		conf.ServerCAFile = serverCAFile
		conf.ServerCertFile = servingKubeletCert
		conf.ServerKeyFile = servingKubeletKey
		conf.PSK = psk[:32]
		conf.InjectMirror(nodeConfig)
	}

	if err := validateNetworkConfig(nodeConfig); err != nil {
		return nil, err
	}

	return nodeConfig, nil
}

// GetAPIServers attempts to return a list of apiservers from the server.
func GetAPIServers(ctx context.Context, info *clientaccess.Info) ([]string, error) {
	data, err := info.Get("/v1-" + version.Program + "/apiservers")
	if err != nil {
		return nil, err
	}

	endpoints := []string{}
	return endpoints, json.Unmarshal(data, &endpoints)
}

// getKubeProxyDisabled attempts to return the DisableKubeProxy setting from the server configuration data.
// It first checks the server readyz endpoint, to ensure that the configuration has stabilized before use.
func getKubeProxyDisabled(ctx context.Context, node *config.Node, proxy proxy.Proxy) (bool, error) {
	withCert := clientaccess.WithClientCertificate(node.AgentConfig.ClientKubeletCert, node.AgentConfig.ClientKubeletKey)
	info, err := clientaccess.ParseAndValidateToken(proxy.SupervisorURL(), node.Token, withCert)
	if err != nil {
		return false, err
	}

	// 500 error indicates that the health check has failed; other errors (for example 401 Unauthorized)
	// indicate that the server is down-level and doesn't support readyz, so we should just use whatever
	// the server has for us.
	if err := getReadyz(info); err != nil && strings.HasSuffix(err.Error(), "500 Internal Server Error") {
		return false, err
	}

	controlConfig, err := getConfig(info)
	if err != nil {
		return false, errors.Wrap(err, "failed to retrieve configuration from server")
	}

	return controlConfig.DisableKubeProxy, nil
}

// getConfig returns server configuration data. Note that this may be mutated during system startup; anything that needs
// to ensure stable system state should check the readyz endpoint first. This is required because RKE2 starts up the
// kubelet early, before the apiserver is available.
func getConfig(info *clientaccess.Info) (*config.Control, error) {
	data, err := info.Get("/v1-" + version.Program + "/config")
	if err != nil {
		return nil, err
	}

	controlControl := &config.Control{}
	return controlControl, json.Unmarshal(data, controlControl)
}

// getReadyz returns nil if the server is ready, or an error if not.
func getReadyz(info *clientaccess.Info) error {
	_, err := info.Get("/v1-" + version.Program + "/readyz")
	return err
}

// validateNetworkConfig ensures that the network configuration values provided by the server make sense.
func validateNetworkConfig(nodeConfig *config.Node) error {
	// Old versions of the server do not send enough information to correctly start the NPC. Users
	// need to upgrade the server to at least the same version as the agent, or disable the NPC
	// cluster-wide.
	if nodeConfig.AgentConfig.DisableNPC == false && (nodeConfig.AgentConfig.ServiceCIDR == nil || nodeConfig.AgentConfig.ServiceNodePortRange.Size == 0) {
		return fmt.Errorf("incompatible down-level server detected; servers must be upgraded to at least %s, or restarted with --disable-network-policy", version.Version)
	}

	return nil
}
