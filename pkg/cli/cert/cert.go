package cert

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/erikdubbelboer/gspt"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/control/deps"
	"github.com/k3s-io/k3s/pkg/datadir"
	"github.com/k3s-io/k3s/pkg/server"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/otiai10/copy"
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

var services = []string{
	adminService,
	apiServerService,
	controllerManagerService,
	schedulerService,
	etcdService,
	version.Program + programControllerService,
	authProxyService,
	cloudControllerService,
	kubeletService,
	kubeProxyService,
	version.Program + k3sServerService,
}

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
	deps.CreateRuntimeCertFiles(&serverConfig.ControlConfig)

	if err := validateCertConfig(); err != nil {
		return err
	}

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

func validService(svc string) bool {
	for _, service := range services {
		if svc == service {
			return true
		}
	}
	return false
}

func validateCertConfig() error {
	for _, s := range cmds.ServicesList {
		if !validService(s) {
			return errors.New("Service " + s + " is not recognized")
		}
	}
	return nil
}
