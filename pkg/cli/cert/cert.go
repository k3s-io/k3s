package cert

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/erikdubbelboer/gspt"
	"github.com/otiai10/copy"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/daemons/control/deps"
	"github.com/rancher/k3s/pkg/datadir"
	"github.com/rancher/k3s/pkg/server"
	"github.com/rancher/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	adminComponent             = "admin"
	apiServerComponent         = "api-server"
	controllerManagerComponent = "controller-manager"
	schedulerComponent         = "scheduler"
	etcdComponent              = "etcd"
	programControllerComponent = "-controller"
	authProxyComponent         = "auth-proxy"
	cloudControllerComponent   = "cloud-controller"
	kubeletComponent           = "kubelet"
	kubeProxyComponent         = "kube-proxy"
)

func commandSetup(app *cli.Context, cfg *cmds.Server, sc *server.Config) (string, string, error) {
	gspt.SetProcTitle(os.Args[0])

	nodeName := app.String("node-name")
	if nodeName == "" {
		h, err := os.Hostname()
		if err != nil {
			return "", "", err
		}
		nodeName = h
	}

	os.Setenv("NODE_NAME", nodeName)

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
	deps.CreateRuntimeCertFiles(&serverConfig.ControlConfig, serverConfig.ControlConfig.Runtime)

	tlsBackupDir, err := backupCertificates(serverDataDir, agentDataDir)
	if err != nil {
		return err
	}

	if len(cmds.ComponentList) == 0 {
		// rotate all certs
		logrus.Infof("Rotating certificates for all services")
		return rotateAllCerts(tlsBackupDir, filepath.Join(serverDataDir, "tls"), agentDataDir)
	}
	certList := []string{}
	for _, component := range cmds.ComponentList {
		logrus.Infof("Rotating certificates for %s service", component)
		switch component {
		case adminComponent:
			certList = append(certList,
				serverConfig.ControlConfig.Runtime.ClientAdminCert,
				serverConfig.ControlConfig.Runtime.ClientAdminKey)
		case apiServerComponent:
			certList = append(certList,
				serverConfig.ControlConfig.Runtime.ClientKubeAPICert,
				serverConfig.ControlConfig.Runtime.ClientKubeAPIKey,
				serverConfig.ControlConfig.Runtime.ServingKubeAPICert,
				serverConfig.ControlConfig.Runtime.ServingKubeAPIKey)
		case controllerManagerComponent:
			certList = append(certList,
				serverConfig.ControlConfig.Runtime.ClientControllerCert,
				serverConfig.ControlConfig.Runtime.ClientControllerKey)
		case schedulerComponent:
			certList = append(certList,
				serverConfig.ControlConfig.Runtime.ClientSchedulerCert,
				serverConfig.ControlConfig.Runtime.ClientSchedulerKey)
		case etcdComponent:
			certList = append(certList,
				serverConfig.ControlConfig.Runtime.ClientETCDCert,
				serverConfig.ControlConfig.Runtime.ClientETCDKey,
				serverConfig.ControlConfig.Runtime.ServerETCDCert,
				serverConfig.ControlConfig.Runtime.ServerETCDKey,
				serverConfig.ControlConfig.Runtime.PeerServerClientETCDCert,
				serverConfig.ControlConfig.Runtime.PeerServerClientETCDKey)
		case cloudControllerComponent:
			certList = append(certList,
				serverConfig.ControlConfig.Runtime.ClientCloudControllerCert,
				serverConfig.ControlConfig.Runtime.ClientCloudControllerKey)
		case version.Program + programControllerComponent:
			certList = append(certList,
				serverConfig.ControlConfig.Runtime.ClientK3sControllerCert,
				serverConfig.ControlConfig.Runtime.ClientK3sControllerKey,
				filepath.Join(agentDataDir, "client-"+version.Program+"-controller.crt"),
				filepath.Join(agentDataDir, "client-"+version.Program+"-controller.key"))
		case authProxyComponent:
			certList = append(certList,
				serverConfig.ControlConfig.Runtime.ClientAuthProxyCert,
				serverConfig.ControlConfig.Runtime.ClientAuthProxyKey)
		case kubeletComponent:
			certList = append(certList,
				serverConfig.ControlConfig.Runtime.ClientKubeletKey,
				serverConfig.ControlConfig.Runtime.ServingKubeletKey,
				filepath.Join(agentDataDir, "client-kubelet.crt"),
				filepath.Join(agentDataDir, "client-kubelet.key"),
				filepath.Join(agentDataDir, "serving-kubelet.crt"),
				filepath.Join(agentDataDir, "serving-kubelet.key"))
		case kubeProxyComponent:
			certList = append(certList,
				serverConfig.ControlConfig.Runtime.ClientKubeProxyCert,
				serverConfig.ControlConfig.Runtime.ClientKubeProxyKey,
				filepath.Join(agentDataDir, "client-kube-proxy.crt"),
				filepath.Join(agentDataDir, "client-kube-proxy.key"))
		default:
			logrus.Fatalf("%s is not a recognized service", component)
		}
	}

	for _, cert := range certList {
		if err := os.Remove(cert); err == nil {
			logrus.Infof("Certificate %s is processed", cert)
		}
	}
	logrus.Infof("Successfully backed up certificates for all services to path %s, please restart %s server or agent to rotate certificates", tlsBackupDir, version.Program)
	return nil
}

func rotateAllCerts(backupDir string, dirs ...string) error {
	for _, dir := range dirs {
		err := filepath.Walk(dir,
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if (strings.HasSuffix(path, ".crt") || strings.HasSuffix(path, ".key")) &&
					!strings.Contains(path, "-ca") &&
					!strings.Contains(path, "service.key") &&
					!strings.Contains(path, "temporary-certs") &&
					!strings.Contains(path, "containerd") {
					if err := os.Remove(path); err == nil {
						logrus.Infof("Certificate %s is processed", path)
					}
					return nil
				}
				return nil
			})
		if err != nil {
			return err
		}
	}
	// adding the regenerate cert file to rotate dynamic listener cert
	dynamicListenerRegenFilePath := filepath.Join(dirs[0], "dynamic-cert-regenerate")
	if err := ioutil.WriteFile(dynamicListenerRegenFilePath, []byte{}, 0600); err != nil {
		return err
	}
	logrus.Infof("Successfully backed up certificates for all services to path %s, please restart %s server or agent to rotate certificates", backupDir, version.Program)
	return nil
}

func copyFile(src, destDir string) error {
	_, err := os.Stat(src)
	if err == nil {
		input, err := ioutil.ReadFile(src)
		if err != nil {
			return err
		}

		if err := ioutil.WriteFile(filepath.Join(destDir, filepath.Base(src)), input, 0644); err != nil {
			return err
		}
		return nil
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
