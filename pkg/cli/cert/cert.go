package cert

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/erikdubbelboer/gspt"
	"github.com/otiai10/copy"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/datadir"
	"github.com/rancher/k3s/pkg/server"
	"github.com/rancher/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	adminService             = "admin"
	apiServerService         = "api-server"
	controllerManagerService = "controller-manager"
	schedulerService         = "scheduler"
	etcdService              = "etcd"
	programControllerService = "-controller"
	authProxyService         = "auth-proxy"
	cloudControllerService   = "cloud-controller"
	kubeletService           = "kubelet"
	kubeProxyService         = "kube-proxy"
	k3sServerService         = "-server"
)

func commandSetup(app *cli.Context, cfg *cmds.Server, sc *server.Config) (string, string, error) {
	gspt.SetProcTitle(os.Args[0])

	sc.ControlConfig.DataDir = cfg.DataDir
	sc.ControlConfig.Runtime = &config.ControlRuntime{}
	dataDir, err := datadir.Resolve(cfg.DataDir)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(dataDir, "server"), filepath.Join(dataDir, "agent"), err
}

func Run(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return rotate(app, &cmds.ServerConfig)
}

func rotate(app *cli.Context, cfg *cmds.Server) error {
	var serverConfig server.Config

	serverDataDir, agentDataDir, err := commandSetup(app, cfg, &serverConfig)
	if err != nil {
		return err
	}

	serverConfig.ControlConfig.DataDir = serverDataDir
	serverConfig.ControlConfig.Runtime = &config.ControlRuntime{}
	serverConfig.ControlConfig.Runtime.ClientCA = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "client-ca.crt")
	serverConfig.ControlConfig.Runtime.ClientCAKey = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "client-ca.key")
	serverConfig.ControlConfig.Runtime.ServerCA = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "server-ca.crt")
	serverConfig.ControlConfig.Runtime.ServerCAKey = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "server-ca.key")
	serverConfig.ControlConfig.Runtime.RequestHeaderCA = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "request-header-ca.crt")
	serverConfig.ControlConfig.Runtime.RequestHeaderCAKey = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "request-header-ca.key")
	serverConfig.ControlConfig.Runtime.IPSECKey = filepath.Join(serverConfig.ControlConfig.DataDir, "cred", "ipsec.psk")

	serverConfig.ControlConfig.Runtime.ServiceKey = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "service.key")
	serverConfig.ControlConfig.Runtime.PasswdFile = filepath.Join(serverConfig.ControlConfig.DataDir, "cred", "passwd")
	serverConfig.ControlConfig.Runtime.NodePasswdFile = filepath.Join(serverConfig.ControlConfig.DataDir, "cred", "node-passwd")

	serverConfig.ControlConfig.Runtime.KubeConfigAdmin = filepath.Join(serverConfig.ControlConfig.DataDir, "cred", "admin.kubeconfig")
	serverConfig.ControlConfig.Runtime.KubeConfigController = filepath.Join(serverConfig.ControlConfig.DataDir, "cred", "controller.kubeconfig")
	serverConfig.ControlConfig.Runtime.KubeConfigScheduler = filepath.Join(serverConfig.ControlConfig.DataDir, "cred", "scheduler.kubeconfig")
	serverConfig.ControlConfig.Runtime.KubeConfigAPIServer = filepath.Join(serverConfig.ControlConfig.DataDir, "cred", "api-server.kubeconfig")
	serverConfig.ControlConfig.Runtime.KubeConfigCloudController = filepath.Join(serverConfig.ControlConfig.DataDir, "cred", "cloud-controller.kubeconfig")

	serverConfig.ControlConfig.Runtime.ClientAdminCert = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "client-admin.crt")
	serverConfig.ControlConfig.Runtime.ClientAdminKey = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "client-admin.key")
	serverConfig.ControlConfig.Runtime.ClientControllerCert = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "client-controller.crt")
	serverConfig.ControlConfig.Runtime.ClientControllerKey = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "client-controller.key")
	serverConfig.ControlConfig.Runtime.ClientCloudControllerCert = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "client-cloud-controller.crt")
	serverConfig.ControlConfig.Runtime.ClientCloudControllerKey = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "client-cloud-controller.key")
	serverConfig.ControlConfig.Runtime.ClientSchedulerCert = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "client-scheduler.crt")
	serverConfig.ControlConfig.Runtime.ClientSchedulerKey = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "client-scheduler.key")
	serverConfig.ControlConfig.Runtime.ClientKubeAPICert = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "client-kube-apiserver.crt")
	serverConfig.ControlConfig.Runtime.ClientKubeAPIKey = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "client-kube-apiserver.key")
	serverConfig.ControlConfig.Runtime.ClientKubeProxyCert = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "client-kube-proxy.crt")
	serverConfig.ControlConfig.Runtime.ClientKubeProxyKey = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "client-kube-proxy.key")
	serverConfig.ControlConfig.Runtime.ClientK3sControllerCert = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "client-"+version.Program+"-controller.crt")
	serverConfig.ControlConfig.Runtime.ClientK3sControllerKey = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "client-"+version.Program+"-controller.key")

	serverConfig.ControlConfig.Runtime.ServingKubeAPICert = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "serving-kube-apiserver.crt")
	serverConfig.ControlConfig.Runtime.ServingKubeAPIKey = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "serving-kube-apiserver.key")

	serverConfig.ControlConfig.Runtime.ClientKubeletKey = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "client-kubelet.key")
	serverConfig.ControlConfig.Runtime.ServingKubeletKey = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "serving-kubelet.key")

	serverConfig.ControlConfig.Runtime.ClientAuthProxyCert = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "client-auth-proxy.crt")
	serverConfig.ControlConfig.Runtime.ClientAuthProxyKey = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "client-auth-proxy.key")

	serverConfig.ControlConfig.Runtime.ETCDServerCA = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "etcd", "server-ca.crt")
	serverConfig.ControlConfig.Runtime.ETCDServerCAKey = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "etcd", "server-ca.key")
	serverConfig.ControlConfig.Runtime.ETCDPeerCA = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "etcd", "peer-ca.crt")
	serverConfig.ControlConfig.Runtime.ETCDPeerCAKey = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "etcd", "peer-ca.key")
	serverConfig.ControlConfig.Runtime.ServerETCDCert = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "etcd", "server-client.crt")
	serverConfig.ControlConfig.Runtime.ServerETCDKey = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "etcd", "server-client.key")
	serverConfig.ControlConfig.Runtime.PeerServerClientETCDCert = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "etcd", "peer-server-client.crt")
	serverConfig.ControlConfig.Runtime.PeerServerClientETCDKey = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "etcd", "peer-server-client.key")
	serverConfig.ControlConfig.Runtime.ClientETCDCert = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "etcd", "client.crt")
	serverConfig.ControlConfig.Runtime.ClientETCDKey = filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "etcd", "client.key")

	tlsBackupDir, err := backupCertificates(serverDataDir, agentDataDir)
	if err != nil {
		return err
	}

	if len(cmds.ServicesList) == 0 {
		// detecting if the service is an agent or server
		_, err := os.Stat(serverDataDir)
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			logrus.Infof("Agent detected, rotating agent certificates")
			cmds.ServicesList = []string{
				kubeletService,
				kubeProxyService,
				version.Program + programControllerService,
			}
		} else {
			logrus.Infof("Server detected, rotating server certificates")
			cmds.ServicesList = []string{
				adminService,
				etcdService,
				apiServerService,
				controllerManagerService,
				cloudControllerService,
				schedulerService,
				version.Program + k3sServerService,
				version.Program + programControllerService,
				authProxyService,
				kubeletService,
				kubeProxyService,
			}
		}
	}
	fileList := []string{}
	for _, service := range cmds.ServicesList {
		logrus.Infof("Rotating certificates for %s service", service)
		switch service {
		case adminService:
			fileList = append(fileList,
				serverConfig.ControlConfig.Runtime.ClientAdminCert,
				serverConfig.ControlConfig.Runtime.ClientAdminKey)
		case apiServerService:
			fileList = append(fileList,
				serverConfig.ControlConfig.Runtime.ClientKubeAPICert,
				serverConfig.ControlConfig.Runtime.ClientKubeAPIKey,
				serverConfig.ControlConfig.Runtime.ServingKubeAPICert,
				serverConfig.ControlConfig.Runtime.ServingKubeAPIKey)
		case controllerManagerService:
			fileList = append(fileList,
				serverConfig.ControlConfig.Runtime.ClientControllerCert,
				serverConfig.ControlConfig.Runtime.ClientControllerKey)
		case schedulerService:
			fileList = append(fileList,
				serverConfig.ControlConfig.Runtime.ClientSchedulerCert,
				serverConfig.ControlConfig.Runtime.ClientSchedulerKey)
		case etcdService:
			fileList = append(fileList,
				serverConfig.ControlConfig.Runtime.ClientETCDCert,
				serverConfig.ControlConfig.Runtime.ClientETCDKey,
				serverConfig.ControlConfig.Runtime.ServerETCDCert,
				serverConfig.ControlConfig.Runtime.ServerETCDKey,
				serverConfig.ControlConfig.Runtime.PeerServerClientETCDCert,
				serverConfig.ControlConfig.Runtime.PeerServerClientETCDKey)
		case cloudControllerService:
			fileList = append(fileList,
				serverConfig.ControlConfig.Runtime.ClientCloudControllerCert,
				serverConfig.ControlConfig.Runtime.ClientCloudControllerKey)
		case version.Program + k3sServerService:
			dynamicListenerRegenFilePath := filepath.Join(serverDataDir, "tls", "dynamic-cert-regenerate")
			if err := ioutil.WriteFile(dynamicListenerRegenFilePath, []byte{}, 0600); err != nil {
				return err
			}
			logrus.Infof("Rotating dynamic listener certificate")
		case version.Program + programControllerService:
			fileList = append(fileList,
				serverConfig.ControlConfig.Runtime.ClientK3sControllerCert,
				serverConfig.ControlConfig.Runtime.ClientK3sControllerKey,
				filepath.Join(agentDataDir, "client-"+version.Program+"-controller.crt"),
				filepath.Join(agentDataDir, "client-"+version.Program+"-controller.key"))
		case authProxyService:
			fileList = append(fileList,
				serverConfig.ControlConfig.Runtime.ClientAuthProxyCert,
				serverConfig.ControlConfig.Runtime.ClientAuthProxyKey)
		case kubeletService:
			fileList = append(fileList,
				serverConfig.ControlConfig.Runtime.ClientKubeletKey,
				serverConfig.ControlConfig.Runtime.ServingKubeletKey,
				filepath.Join(agentDataDir, "client-kubelet.crt"),
				filepath.Join(agentDataDir, "client-kubelet.key"),
				filepath.Join(agentDataDir, "serving-kubelet.crt"),
				filepath.Join(agentDataDir, "serving-kubelet.key"))
		case kubeProxyService:
			fileList = append(fileList,
				serverConfig.ControlConfig.Runtime.ClientKubeProxyCert,
				serverConfig.ControlConfig.Runtime.ClientKubeProxyKey,
				filepath.Join(agentDataDir, "client-kube-proxy.crt"),
				filepath.Join(agentDataDir, "client-kube-proxy.key"))
		default:
			logrus.Fatalf("%s is not a recognized service", service)
		}
	}

	for _, file := range fileList {
		if err := os.Remove(file); err == nil {
			logrus.Debugf("file %s is deleted", file)
		}
	}
	logrus.Infof("Successfully backed up certificates for all services to path %s, please restart %s server or agent to rotate certificates", tlsBackupDir, version.Program)
	return nil
}

func copyFile(src, destDir string) error {
	_, err := os.Stat(src)
	if err == nil {
		input, err := ioutil.ReadFile(src)
		if err != nil {
			return err
		}
		return ioutil.WriteFile(filepath.Join(destDir, filepath.Base(src)), input, 0644)
	} else if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func backupCertificates(serverDataDir, agentDataDir string) (string, error) {
	serverTLSDir := filepath.Join(serverDataDir, "tls")
	tlsBackupDir := filepath.Join(serverDataDir, "tls-"+strconv.Itoa(int(time.Now().Unix())))

	if _, err := os.Stat(serverTLSDir); err != nil {
		return "", err
	}
	if err := copy.Copy(serverTLSDir, tlsBackupDir); err != nil {
		return "", err
	}
	agentCerts := []string{
		filepath.Join(agentDataDir, "client-"+version.Program+"-controller.crt"),
		filepath.Join(agentDataDir, "client-"+version.Program+"-controller.key"),
		filepath.Join(agentDataDir, "client-kubelet.crt"),
		filepath.Join(agentDataDir, "client-kubelet.key"),
		filepath.Join(agentDataDir, "serving-kubelet.crt"),
		filepath.Join(agentDataDir, "serving-kubelet.key"),
		filepath.Join(agentDataDir, "client-kube-proxy.crt"),
		filepath.Join(agentDataDir, "client-kube-proxy.key"),
	}
	for _, cert := range agentCerts {
		if err := copyFile(cert, tlsBackupDir); err != nil {
			return "", err
		}
	}
	return tlsBackupDir, nil
}
