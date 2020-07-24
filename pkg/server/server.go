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

	corev1 "k8s.io/api/core/v1"

	"github.com/pkg/errors"
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
	"github.com/rancher/k3s/pkg/util"
	"github.com/rancher/k3s/pkg/version"
	v1 "github.com/rancher/wrangler-api/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/leader"
	"github.com/rancher/wrangler/pkg/resolvehome"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/net"
)

const MasterRoleLabelKey = "node-role.kubernetes.io/master"

func resolveDataDir(dataDir string) (string, error) {
	dataDir, err := datadir.Resolve(dataDir)
	return filepath.Join(dataDir, "server"), err
}

func StartServer(ctx context.Context, config *Config) error {
	if err := setupDataDirAndChdir(&config.ControlConfig); err != nil {
		return err
	}

	if err := setNoProxyEnv(&config.ControlConfig); err != nil {
		return err
	}

	if err := control.Server(ctx, &config.ControlConfig); err != nil {
		return errors.Wrap(err, "starting kubernetes")
	}

	if err := startWrangler(ctx, config); err != nil {
		return errors.Wrap(err, "starting tls server")
	}

	ip := net2.ParseIP(config.ControlConfig.BindAddress)
	if ip == nil {
		hostIP, err := net.ChooseHostInterface()
		if err == nil {
			ip = hostIP
		} else {
			ip = net2.ParseIP("127.0.0.1")
		}
	}

	if err := printTokens(ip.String(), &config.ControlConfig); err != nil {
		return err
	}

	return writeKubeConfig(config.ControlConfig.Runtime.ServerCA, config)
}

func startWrangler(ctx context.Context, config *Config) error {
	var (
		err           error
		controlConfig = &config.ControlConfig
	)

	ca, err := ioutil.ReadFile(config.ControlConfig.Runtime.ServerCA)
	if err != nil {
		return err
	}

	controlConfig.Runtime.Handler = router(controlConfig, controlConfig.Runtime.Tunnel, ca)

	// Start in background
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-config.ControlConfig.Runtime.APIServerReady:
			if err := runControllers(ctx, config); err != nil {
				logrus.Fatal("failed to start controllers: %v", err)
			}
		}
	}()

	return nil
}

func runControllers(ctx context.Context, config *Config) error {
	controlConfig := &config.ControlConfig

	sc, err := newContext(ctx, controlConfig.Runtime.KubeConfigAdmin)
	if err != nil {
		return err
	}

	if err := stageFiles(ctx, sc, controlConfig); err != nil {
		return err
	}

	controlConfig.Runtime.Core = sc.Core
	if config.ControlConfig.Runtime.ClusterControllerStart != nil {
		if err := config.ControlConfig.Runtime.ClusterControllerStart(ctx); err != nil {
			return errors.Wrapf(err, "starting cluster controllers")
		}
	}

	if err := sc.Start(ctx); err != nil {
		return err
	}

	start := func(ctx context.Context) {
		if err := masterControllers(ctx, sc, config); err != nil {
			panic(err)
		}
		if err := sc.Start(ctx); err != nil {
			panic(err)
		}
	}
	if !config.DisableAgent {
		go setMasterRoleLabel(ctx, sc.Core.Core().V1().Node())
	}

	go setClusterDNSConfig(ctx, config, sc.Core.Core().V1().ConfigMap())

	if controlConfig.NoLeaderElect {
		go func() {
			start(ctx)
			<-ctx.Done()
			logrus.Fatal("controllers exited")
		}()
	} else {
		go leader.RunOrDie(ctx, "", version.Program, sc.K8s, start)
	}

	return nil
}

func masterControllers(ctx context.Context, sc *Context, config *Config) error {
	if !config.ControlConfig.Skips["coredns"] {
		if err := node.Register(ctx, sc.Core.Core().V1().ConfigMap(), sc.Core.Core().V1().Node()); err != nil {
			return err
		}
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

	if config.Rootless {
		return rootlessports.Register(ctx, sc.Core.Core().V1().Service(), !config.DisableServiceLB, config.ControlConfig.HTTPSPort)
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
		"%{CLUSTER_DNS}%":                controlConfig.ClusterDNS.String(),
		"%{CLUSTER_DOMAIN}%":             controlConfig.ClusterDomain,
		"%{DEFAULT_LOCAL_STORAGE_PATH}%": controlConfig.DefaultLocalStoragePath,
	}

	if err := deploy.Stage(dataDir, templateVars, controlConfig.Skips); err != nil {
		return err
	}

	return deploy.WatchFiles(ctx, sc.Apply, sc.K3s.K3s().V1().Addon(), controlConfig.Disables, dataDir)
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

func printTokens(advertiseIP string, config *config.Control) error {
	var (
		nodeFile string
	)

	if advertiseIP == "" {
		advertiseIP = "127.0.0.1"
	}

	if len(config.Runtime.ServerToken) > 0 {
		p := filepath.Join(config.DataDir, "token")
		if err := writeToken(config.Runtime.ServerToken, p, config.Runtime.ServerCA); err == nil {
			logrus.Infof("Node token is available at %s", p)
			nodeFile = p
		}

		// backwards compatibility
		np := filepath.Join(config.DataDir, "node-token")
		if !isSymlink(np) {
			if err := os.RemoveAll(np); err != nil {
				return err
			}
			if err := os.Symlink(p, np); err != nil {
				return err
			}
		}
	}

	if len(nodeFile) > 0 {
		printToken(config.SupervisorPort, advertiseIP, "To join node to cluster:", "agent")
	}

	return nil
}

func writeKubeConfig(certs string, config *Config) error {
	ip := config.ControlConfig.BindAddress
	if ip == "" {
		ip = "127.0.0.1"
	}
	url := fmt.Sprintf("https://%s:%d", ip, config.ControlConfig.HTTPSPort)
	kubeConfig, err := HomeKubeConfig(true, config.Rootless)
	def := true
	if err != nil {
		kubeConfig = filepath.Join(config.ControlConfig.DataDir, "kubeconfig-"+version.Program+".yaml")
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

	if err = clientaccess.WriteClientKubeConfig(kubeConfig, url, config.ControlConfig.Runtime.ServerCA, config.ControlConfig.Runtime.ClientAdminCert,
		config.ControlConfig.Runtime.ClientAdminKey); err == nil {
		logrus.Infof("Wrote kubeconfig %s", kubeConfig)
	} else {
		logrus.Errorf("Failed to generate kubeconfig: %v", err)
		return err
	}

	if config.ControlConfig.KubeConfigMode != "" {
		mode, err := strconv.ParseInt(config.ControlConfig.KubeConfigMode, 8, 0)
		if err == nil {
			util.SetFileModeForPath(kubeConfig, os.FileMode(mode))
		} else {
			logrus.Errorf("failed to set %s to mode %s: %v", kubeConfig, os.FileMode(mode), err)
		}
	} else {
		util.SetFileModeForPath(kubeConfig, os.FileMode(0600))
	}

	if kubeConfigSymlink != kubeConfig {
		if err := writeConfigSymlink(kubeConfig, kubeConfigSymlink); err != nil {
			logrus.Errorf("failed to write kubeconfig symlink: %v", err)
		}
	}

	if def {
		logrus.Infof("Run: %s kubectl", filepath.Base(os.Args[0]))
	}

	return nil
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

	logrus.Infof("%s %s %s -s https://%s:%d -t ${NODE_TOKEN}", prefix, version.Program, cmd, ip, httpsPort)
}

func FormatToken(token string, certFile string) (string, error) {
	if len(token) == 0 {
		return token, nil
	}

	prefix := "K10"
	if len(certFile) > 0 {
		bytes, err := ioutil.ReadFile(certFile)
		if err != nil {
			return "", nil
		}
		digest := sha256.Sum256(bytes)
		prefix = "K10" + hex.EncodeToString(digest[:]) + "::"
	}

	return prefix + token, nil
}

func writeToken(token, file, certs string) error {
	if len(token) == 0 {
		return nil
	}

	token, err := FormatToken(token, certs)
	if err != nil {
		return err
	}
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

func setMasterRoleLabel(ctx context.Context, nodes v1.NodeClient) error {
	for {
		nodeName := os.Getenv("NODE_NAME")
		node, err := nodes.Get(nodeName, metav1.GetOptions{})
		if err != nil {
			logrus.Infof("Waiting for master node %s startup: %v", nodeName, err)
			time.Sleep(1 * time.Second)
			continue
		}
		if v, ok := node.Labels[MasterRoleLabelKey]; ok && v == "true" {
			break
		}
		if node.Labels == nil {
			node.Labels = make(map[string]string)
		}
		node.Labels[MasterRoleLabelKey] = "true"
		_, err = nodes.Update(node)
		if err == nil {
			logrus.Infof("master role label has been set succesfully on node: %s", nodeName)
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil
}

func setClusterDNSConfig(ctx context.Context, controlConfig *Config, configMap v1.ConfigMapClient) error {
	nodeName := os.Getenv("NODE_NAME")
	// check if configmap already exists
	_, err := configMap.Get("kube-system", "cluster-dns", metav1.GetOptions{})
	if err == nil {
		logrus.Infof("cluster dns configmap already exists")
		return nil
	}
	clusterDNS := controlConfig.ControlConfig.ClusterDNS
	clusterDomain := controlConfig.ControlConfig.ClusterDomain
	c := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-dns",
			Namespace: "kube-system",
		},
		Data: map[string]string{
			"clusterDNS":    clusterDNS.String(),
			"clusterDomain": clusterDomain,
		},
	}
	for {
		_, err = configMap.Create(c)
		if err == nil {
			logrus.Infof("cluster dns configmap has been set successfully")
			break
		}
		logrus.Infof("Waiting for master node %s startup: %v", nodeName, err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil
}
