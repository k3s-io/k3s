package cert

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/erikdubbelboer/gspt"
	"github.com/k3s-io/k3s/pkg/bootstrap"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/clientaccess"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/control/deps"
	"github.com/k3s-io/k3s/pkg/datadir"
	"github.com/k3s-io/k3s/pkg/server"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/otiai10/copy"
	"github.com/pkg/errors"
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

func commandSetup(app *cli.Context, cfg *cmds.Server, sc *server.Config) (string, error) {
	gspt.SetProcTitle(os.Args[0])

	dataDir, err := datadir.Resolve(cfg.DataDir)
	if err != nil {
		return "", err
	}
	sc.ControlConfig.DataDir = filepath.Join(dataDir, "server")

	if cfg.Token == "" {
		fp := filepath.Join(sc.ControlConfig.DataDir, "token")
		tokenByte, err := os.ReadFile(fp)
		if err != nil {
			return "", err
		}
		cfg.Token = string(bytes.TrimRight(tokenByte, "\n"))
	}
	sc.ControlConfig.Token = cfg.Token

	sc.ControlConfig.Runtime = config.NewRuntime(nil)

	return dataDir, nil
}

func Rotate(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return rotate(app, &cmds.ServerConfig)
}

func rotate(app *cli.Context, cfg *cmds.Server) error {
	var serverConfig server.Config

	dataDir, err := commandSetup(app, cfg, &serverConfig)
	if err != nil {
		return err
	}

	deps.CreateRuntimeCertFiles(&serverConfig.ControlConfig)

	if err := validateCertConfig(); err != nil {
		return err
	}

	agentDataDir := filepath.Join(dataDir, "agent")
	tlsBackupDir, err := backupCertificates(serverConfig.ControlConfig.DataDir, agentDataDir)
	if err != nil {
		return err
	}

	if len(cmds.ServicesList) == 0 {
		// detecting if the command is being run on an agent or server
		_, err := os.Stat(serverConfig.ControlConfig.DataDir)
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
			dynamicListenerRegenFilePath := filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "dynamic-cert-regenerate")
			if err := os.WriteFile(dynamicListenerRegenFilePath, []byte{}, 0600); err != nil {
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
		input, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(destDir, filepath.Base(src)), input, 0644)
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

func RotateCA(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return rotateCA(app, &cmds.ServerConfig, &cmds.CertRotateCAConfig)
}

func rotateCA(app *cli.Context, cfg *cmds.Server, sync *cmds.CertRotateCA) error {
	var serverConfig server.Config

	_, err := commandSetup(app, cfg, &serverConfig)
	if err != nil {
		return err
	}

	info, err := clientaccess.ParseAndValidateToken(cmds.ServerConfig.ServerURL, serverConfig.ControlConfig.Token, clientaccess.WithUser("server"))
	if err != nil {
		return err
	}

	// Set up dummy server config for reading new bootstrap data from disk.
	tmpServer := &config.Control{
		Runtime: config.NewRuntime(nil),
		DataDir: sync.CACertPath,
	}
	deps.CreateRuntimeCertFiles(tmpServer)

	// Override these paths so that we don't get warnings when they don't exist, as the user is not expected to provide them.
	tmpServer.Runtime.PasswdFile = "/dev/null"
	tmpServer.Runtime.IPSECKey = "/dev/null"

	buf := &bytes.Buffer{}
	if err := bootstrap.ReadFromDisk(buf, &tmpServer.Runtime.ControlRuntimeBootstrap); err != nil {
		return err
	}

	url := fmt.Sprintf("/v1-%s/cert/cacerts?force=%t", version.Program, sync.Force)
	if err = info.Put(url, buf.Bytes()); err != nil {
		return errors.Wrap(err, "see server log for details")
	}

	fmt.Println("certificates saved to datastore")
	return nil
}
