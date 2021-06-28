package unit_tests

import (
	"net"
	"os"
	"path/filepath"

	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/daemons/control/deps"
	"github.com/rancher/k3s/pkg/version"
)

// All certs and other files are stored in /tmp/k3s/<RANDOM_STRING>/
// for testing
func GenerateTestDataDir(cnf *config.Control) error {
	var err error
	if err = os.MkdirAll(cnf.DataDir, 0700); err != nil {
		return err
	}
	testDir, err := os.MkdirTemp(cnf.DataDir, "*")
	if err != nil {
		return err
	}
	// Remove old symlink and add new one
	os.Remove(filepath.Join(cnf.DataDir, "latest"))
	if err = os.Symlink(testDir, filepath.Join(cnf.DataDir, "latest")); err != nil {
		return err
	}
	cnf.DataDir = testDir
	cnf.DataDir, err = filepath.Abs(cnf.DataDir)
	if err != nil {
		return err
	}
	return nil
}

func CleanupTestDataDir(cnf *config.Control) {
	os.RemoveAll(cnf.DataDir)
}

func GenerateTestRuntime(cnf *config.Control) error {
	var err error

	// Setup defaults for the config
	_, clusterIPNet, _ := net.ParseCIDR("10.42.0.0/16")
	cnf.ClusterIPRange = clusterIPNet

	_, serviceIPNet, _ := net.ParseCIDR("10.43.0.0/16")
	cnf.ServiceIPRange = serviceIPNet
	cnf.ClusterDNS = net.ParseIP("10.43.0.10")
	cnf.AdvertisePort = cnf.HTTPSPort

	runtime := &config.ControlRuntime{}
	if err = GenerateTestDataDir(cnf); err != nil {
		return err
	}

	os.MkdirAll(filepath.Join(cnf.DataDir, "tls"), 0700)
	os.MkdirAll(filepath.Join(cnf.DataDir, "cred"), 0700)

	runtime.ClientCA = filepath.Join(cnf.DataDir, "tls", "client-ca.crt")
	runtime.ClientCAKey = filepath.Join(cnf.DataDir, "tls", "client-ca.key")
	runtime.ServerCA = filepath.Join(cnf.DataDir, "tls", "server-ca.crt")
	runtime.ServerCAKey = filepath.Join(cnf.DataDir, "tls", "server-ca.key")
	runtime.RequestHeaderCA = filepath.Join(cnf.DataDir, "tls", "request-header-ca.crt")
	runtime.RequestHeaderCAKey = filepath.Join(cnf.DataDir, "tls", "request-header-ca.key")
	runtime.IPSECKey = filepath.Join(cnf.DataDir, "cred", "ipsec.psk")

	runtime.ServiceKey = filepath.Join(cnf.DataDir, "tls", "service.key")
	runtime.PasswdFile = filepath.Join(cnf.DataDir, "cred", "passwd")
	runtime.NodePasswdFile = filepath.Join(cnf.DataDir, "cred", "node-passwd")

	runtime.KubeConfigAdmin = filepath.Join(cnf.DataDir, "cred", "admin.kubeconfig")
	runtime.KubeConfigController = filepath.Join(cnf.DataDir, "cred", "controller.kubeconfig")
	runtime.KubeConfigScheduler = filepath.Join(cnf.DataDir, "cred", "scheduler.kubeconfig")
	runtime.KubeConfigAPIServer = filepath.Join(cnf.DataDir, "cred", "api-server.kubeconfig")
	runtime.KubeConfigCloudController = filepath.Join(cnf.DataDir, "cred", "cloud-controller.kubeconfig")

	runtime.ClientAdminCert = filepath.Join(cnf.DataDir, "tls", "client-admin.crt")
	runtime.ClientAdminKey = filepath.Join(cnf.DataDir, "tls", "client-admin.key")
	runtime.ClientControllerCert = filepath.Join(cnf.DataDir, "tls", "client-controller.crt")
	runtime.ClientControllerKey = filepath.Join(cnf.DataDir, "tls", "client-controller.key")
	runtime.ClientCloudControllerCert = filepath.Join(cnf.DataDir, "tls", "client-"+version.Program+"-cloud-controller.crt")
	runtime.ClientCloudControllerKey = filepath.Join(cnf.DataDir, "tls", "client-"+version.Program+"-cloud-controller.key")
	runtime.ClientSchedulerCert = filepath.Join(cnf.DataDir, "tls", "client-scheduler.crt")
	runtime.ClientSchedulerKey = filepath.Join(cnf.DataDir, "tls", "client-scheduler.key")
	runtime.ClientKubeAPICert = filepath.Join(cnf.DataDir, "tls", "client-kube-apiserver.crt")
	runtime.ClientKubeAPIKey = filepath.Join(cnf.DataDir, "tls", "client-kube-apiserver.key")
	runtime.ClientKubeProxyCert = filepath.Join(cnf.DataDir, "tls", "client-kube-proxy.crt")
	runtime.ClientKubeProxyKey = filepath.Join(cnf.DataDir, "tls", "client-kube-proxy.key")
	runtime.ClientK3sControllerCert = filepath.Join(cnf.DataDir, "tls", "client-"+version.Program+"-controller.crt")
	runtime.ClientK3sControllerKey = filepath.Join(cnf.DataDir, "tls", "client-"+version.Program+"-controller.key")

	runtime.ServingKubeAPICert = filepath.Join(cnf.DataDir, "tls", "serving-kube-apiserver.crt")
	runtime.ServingKubeAPIKey = filepath.Join(cnf.DataDir, "tls", "serving-kube-apiserver.key")

	runtime.ClientKubeletKey = filepath.Join(cnf.DataDir, "tls", "client-kubelet.key")
	runtime.ServingKubeletKey = filepath.Join(cnf.DataDir, "tls", "serving-kubelet.key")

	runtime.ClientAuthProxyCert = filepath.Join(cnf.DataDir, "tls", "client-auth-proxy.crt")
	runtime.ClientAuthProxyKey = filepath.Join(cnf.DataDir, "tls", "client-auth-proxy.key")

	runtime.ETCDServerCA = filepath.Join(cnf.DataDir, "tls", "etcd", "server-ca.crt")
	runtime.ETCDServerCAKey = filepath.Join(cnf.DataDir, "tls", "etcd", "server-ca.key")
	runtime.ETCDPeerCA = filepath.Join(cnf.DataDir, "tls", "etcd", "peer-ca.crt")
	runtime.ETCDPeerCAKey = filepath.Join(cnf.DataDir, "tls", "etcd", "peer-ca.key")
	runtime.ServerETCDCert = filepath.Join(cnf.DataDir, "tls", "etcd", "server-client.crt")
	runtime.ServerETCDKey = filepath.Join(cnf.DataDir, "tls", "etcd", "server-client.key")
	runtime.PeerServerClientETCDCert = filepath.Join(cnf.DataDir, "tls", "etcd", "peer-server-client.crt")
	runtime.PeerServerClientETCDKey = filepath.Join(cnf.DataDir, "tls", "etcd", "peer-server-client.key")
	runtime.ClientETCDCert = filepath.Join(cnf.DataDir, "tls", "etcd", "client.crt")
	runtime.ClientETCDKey = filepath.Join(cnf.DataDir, "tls", "etcd", "client.key")

	if cnf.EncryptSecrets {
		runtime.EncryptionConfig = filepath.Join(cnf.DataDir, "cred", "encryption-cnf.json")
	}

	if err := deps.GenServerDeps(cnf, runtime); err != nil {
		return err
	}
	cnf.Runtime = runtime
	return nil
}
