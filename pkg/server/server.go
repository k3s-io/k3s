package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	net2 "net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/dynamiclistener"
	"github.com/rancher/helm-controller/pkg/helm"
	"github.com/rancher/k3s/pkg/clientaccess"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/daemons/control"
	"github.com/rancher/k3s/pkg/datadir"
	"github.com/rancher/k3s/pkg/deploy"
	"github.com/rancher/k3s/pkg/node"
	"github.com/rancher/k3s/pkg/rootlessports"
	"github.com/rancher/k3s/pkg/servicelb"
	"github.com/rancher/k3s/pkg/static"
	"github.com/rancher/k3s/pkg/tls"
	"github.com/rancher/wrangler/pkg/leader"
	"github.com/rancher/wrangler/pkg/resolvehome"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/net"
)

func resolveDataDir(dataDir string) (string, error) {
	dataDir, err := datadir.Resolve(dataDir)
	return filepath.Join(dataDir, "server"), err
}

func StartServer(ctx context.Context, config *Config) (string, error) {
	if err := setupDataDirAndChdir(&config.ControlConfig); err != nil {
		return "", err
	}

	if err := setNoProxyEnv(&config.ControlConfig); err != nil {
		return "", err
	}

	if err := control.Server(ctx, &config.ControlConfig); err != nil {
		return "", errors.Wrap(err, "starting kubernetes")
	}

	certs, err := startWrangler(ctx, config)
	if err != nil {
		return "", errors.Wrap(err, "starting tls server")
	}

	ip := net2.ParseIP(config.TLSConfig.BindAddress)
	if ip == nil {
		ip, err = net.ChooseHostInterface()
		if err != nil {
			ip = net2.ParseIP("127.0.0.1")
		}
	}
	printTokens(certs, ip.String(), &config.TLSConfig, &config.ControlConfig)

	writeKubeConfig(certs, &config.TLSConfig, config)

	return certs, nil
}

func startWrangler(ctx context.Context, config *Config) (string, error) {
	var (
		err           error
		tlsServer     dynamiclistener.ServerInterface
		tlsConfig     = &config.TLSConfig
		controlConfig = &config.ControlConfig
	)

	tlsConfig.Handler = router(controlConfig, controlConfig.Runtime.Tunnel, func() (string, error) {
		if tlsServer == nil {
			return "", nil
		}
		return tlsServer.CACert()
	})

	sc, err := newContext(ctx, controlConfig.Runtime.KubeConfigSystem)
	if err != nil {
		return "", err
	}

	if err := stageFiles(ctx, sc, controlConfig); err != nil {
		return "", err
	}

	tlsServer, err = tls.NewServer(ctx, sc.K3s.K3s().V1().ListenerConfig(), *tlsConfig)
	if err != nil {
		return "", err
	}

	if err := sc.Start(ctx); err != nil {
		return "", err
	}

	certs := ""
	for certs == "" {
		certs, err = tlsServer.CACert()
		if err != nil {
			logrus.Infof("waiting to generate CA certs")
			time.Sleep(time.Second)
			continue
		}
	}

	go leader.RunOrDie(ctx, "", "k3s", sc.K8s, func(ctx context.Context) {
		if err := masterControllers(ctx, sc, config); err != nil {
			panic(err)
		}
		if err := sc.Start(ctx); err != nil {
			panic(err)
		}
	})

	return certs, nil
}

func masterControllers(ctx context.Context, sc *Context, config *Config) error {
	if err := node.Register(ctx, sc.Core.Core().V1().ConfigMap(), sc.Core.Core().V1().Node()); err != nil {
		return err
	}

	helm.Register(ctx, sc.Apply,
		sc.Helm.Helm().V1().HelmChart(),
		sc.Batch.Batch().V1().Job(),
		sc.Auth.Rbac().V1().ClusterRoleBinding(),
		sc.Core.Core().V1().ServiceAccount(),
		sc.Core.Core().V1().ConfigMap())
	if err := servicelb.Register(ctx,
		sc.K8s,
		sc.Apply,
		sc.Apps.Apps().V1().DaemonSet(),
		sc.Apps.Apps().V1().Deployment(),
		sc.Core.Core().V1().Node(),
		sc.Core.Core().V1().Pod(),
		sc.Core.Core().V1().Service(),
		sc.Core.Core().V1().Endpoints(),
		!config.DisableServiceLB, config.Rootless); err != nil {
		return err
	}

	if !config.DisableServiceLB && config.Rootless {
		return rootlessports.Register(ctx, sc.Core.Core().V1().Service(), config.TLSConfig.HTTPSPort)
	}

	return nil
}

func stageFiles(ctx context.Context, sc *Context, controlConfig *config.Control) error {
	dataDir := filepath.Join(controlConfig.DataDir, "static")
	if err := static.Stage(dataDir); err != nil {
		return err
	}

	dataDir = filepath.Join(controlConfig.DataDir, "manifests")
	templateVars := map[string]string{
		"%{CLUSTER_DNS}%":    controlConfig.ClusterDNS.String(),
		"%{CLUSTER_DOMAIN}%": controlConfig.ClusterDomain,
	}

	if err := deploy.Stage(dataDir, templateVars, controlConfig.Skips); err != nil {
		return err
	}

	return deploy.WatchFiles(ctx, sc.Apply, sc.K3s.K3s().V1().Addon(), dataDir)
}

func HomeKubeConfig(write, rootless bool) (string, error) {
	if write {
		if os.Getuid() == 0 && !rootless {
			return datadir.GlobalConfig, nil
		}
		return resolvehome.Resolve(datadir.HomeConfig)
	}

	if _, err := os.Stat(datadir.GlobalConfig); err == nil {
		return datadir.GlobalConfig, nil
	}

	return resolvehome.Resolve(datadir.HomeConfig)
}

func printTokens(certs, advertiseIP string, tlsConfig *dynamiclistener.UserConfig, config *config.Control) {
	var (
		nodeFile string
	)

	if advertiseIP == "" {
		advertiseIP = "localhost"
	}

	if len(config.Runtime.NodeToken) > 0 {
		p := filepath.Join(config.DataDir, "node-token")
		if err := writeToken(config.Runtime.NodeToken, p, certs); err == nil {
			logrus.Infof("Node token is available at %s", p)
			nodeFile = p
		}
	}

	if len(nodeFile) > 0 {
		printToken(tlsConfig.HTTPSPort, advertiseIP, "To join node to cluster:", "agent")
	}
}

func writeKubeConfig(certs string, tlsConfig *dynamiclistener.UserConfig, config *Config) {
	clientToken := FormatToken(config.ControlConfig.Runtime.ClientToken, certs)
	ip := tlsConfig.BindAddress
	if ip == "" {
		ip = "localhost"
	}
	url := fmt.Sprintf("https://%s:%d", ip, tlsConfig.HTTPSPort)
	kubeConfig, err := HomeKubeConfig(true, config.Rootless)
	def := true
	if err != nil {
		kubeConfig = filepath.Join(config.ControlConfig.DataDir, "kubeconfig-k3s.yaml")
		def = false
	}
	kubeConfigSymlink := kubeConfig
	if config.ControlConfig.KubeConfigOutput != "" {
		kubeConfig = config.ControlConfig.KubeConfigOutput
	}

	if isSymlink(kubeConfigSymlink) {
		if err := os.Remove(kubeConfigSymlink); err != nil {
			logrus.Errorf("failed to remove kubeconfig symlink")
		}
	}

	if err = clientaccess.AgentAccessInfoToKubeConfig(kubeConfig, url, clientToken); err != nil {
		logrus.Errorf("Failed to generate kubeconfig: %v", err)
	}

	if config.ControlConfig.KubeConfigMode != "" {
		mode, err := strconv.ParseInt(config.ControlConfig.KubeConfigMode, 8, 0)
		if err == nil {
			os.Chmod(kubeConfig, os.FileMode(mode))
		} else {
			logrus.Errorf("failed to set %s to mode %s: %v", kubeConfig, os.FileMode(mode), err)
		}
	} else {
		os.Chmod(kubeConfig, os.FileMode(0600))
	}

	if kubeConfigSymlink != kubeConfig {
		if err := writeConfigSymlink(kubeConfig, kubeConfigSymlink); err != nil {
			logrus.Errorf("failed to write kubeconfig symlink: %v", err)
		}
	}

	logrus.Infof("Wrote kubeconfig %s", kubeConfig)
	if def {
		logrus.Infof("Run: %s kubectl", filepath.Base(os.Args[0]))
	}
}

func setupDataDirAndChdir(config *config.Control) error {
	var (
		err error
	)

	config.DataDir, err = resolveDataDir(config.DataDir)
	if err != nil {
		return err
	}

	dataDir := config.DataDir

	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return errors.Wrapf(err, "can not mkdir %s", dataDir)
	}

	if err := os.Chdir(dataDir); err != nil {
		return errors.Wrapf(err, "can not chdir %s", dataDir)
	}

	return nil
}

func printToken(httpsPort int, advertiseIP, prefix, cmd string) {
	ip := advertiseIP
	if ip == "" {
		hostIP, err := net.ChooseHostInterface()
		if err != nil {
			logrus.Error(err)
		}
		ip = hostIP.String()
	}

	logrus.Infof("%s k3s %s -s https://%s:%d -t ${NODE_TOKEN}", prefix, cmd, ip, httpsPort)
}

func FormatToken(token string, certs string) string {
	if len(token) == 0 {
		return token
	}

	prefix := "K10"
	if len(certs) > 0 {
		digest := sha256.Sum256([]byte(certs))
		prefix = "K10" + hex.EncodeToString(digest[:]) + "::"
	}

	return prefix + token
}

func writeToken(token, file, certs string) error {
	if len(token) == 0 {
		return nil
	}

	token = FormatToken(token, certs)
	return ioutil.WriteFile(file, []byte(token+"\n"), 0600)
}

func setNoProxyEnv(config *config.Control) error {
	envList := strings.Join([]string{
		os.Getenv("NO_PROXY"),
		config.ClusterIPRange.String(),
		config.ServiceIPRange.String(),
	}, ",")
	return os.Setenv("NO_PROXY", envList)
}

func writeConfigSymlink(kubeconfig, kubeconfigSymlink string) error {
	if err := os.Remove(kubeconfigSymlink); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove %s file: %v", kubeconfigSymlink, err)
	}
	if err := os.MkdirAll(filepath.Dir(kubeconfigSymlink), 0755); err != nil {
		return fmt.Errorf("failed to create path for symlink: %v", err)
	}
	if err := os.Symlink(kubeconfig, kubeconfigSymlink); err != nil {
		return fmt.Errorf("failed to create symlink: %v", err)
	}
	return nil
}

func isSymlink(config string) bool {
	if fi, err := os.Lstat(config); err == nil && (fi.Mode()&os.ModeSymlink == os.ModeSymlink) {
		return true
	}
	return false
}
