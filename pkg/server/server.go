package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	helmchart "github.com/k3s-io/helm-controller/pkg/controllers/chart"
	helmcommon "github.com/k3s-io/helm-controller/pkg/controllers/common"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/clientaccess"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/control"
	"github.com/k3s-io/k3s/pkg/datadir"
	"github.com/k3s-io/k3s/pkg/deploy"
	"github.com/k3s-io/k3s/pkg/node"
	"github.com/k3s-io/k3s/pkg/nodepassword"
	"github.com/k3s-io/k3s/pkg/rootlessports"
	"github.com/k3s-io/k3s/pkg/secretsencrypt"
	"github.com/k3s-io/k3s/pkg/server/handlers"
	"github.com/k3s-io/k3s/pkg/static"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/util/permissions"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/pkg/errors"
	"github.com/rancher/wrangler/v3/pkg/apply"
	v1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/leader"
	"github.com/rancher/wrangler/v3/pkg/resolvehome"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

func ResolveDataDir(dataDir string) (string, error) {
	dataDir, err := datadir.Resolve(dataDir)
	return filepath.Join(dataDir, "server"), err
}

func StartServer(ctx context.Context, config *Config, cfg *cmds.Server) error {
	if err := setupDataDirAndChdir(&config.ControlConfig); err != nil {
		return err
	}

	if err := setNoProxyEnv(&config.ControlConfig); err != nil {
		return err
	}

	if err := control.Server(ctx, &config.ControlConfig); err != nil {
		return errors.Wrap(err, "starting kubernetes")
	}

	wg := &sync.WaitGroup{}
	wg.Add(len(config.StartupHooks))

	config.ControlConfig.Runtime.Handler = handlers.NewHandler(ctx, &config.ControlConfig, cfg)
	config.ControlConfig.Runtime.StartupHooksWg = wg

	shArgs := cmds.StartupHookArgs{
		APIServerReady:       config.ControlConfig.Runtime.APIServerReady,
		KubeConfigSupervisor: config.ControlConfig.Runtime.KubeConfigSupervisor,
		Skips:                config.ControlConfig.Skips,
		Disables:             config.ControlConfig.Disables,
	}
	for _, hook := range config.StartupHooks {
		if err := hook(ctx, wg, shArgs); err != nil {
			return errors.Wrap(err, "startup hook")
		}
	}
	go startOnAPIServerReady(ctx, config)

	if err := printTokens(&config.ControlConfig); err != nil {
		return err
	}

	return writeKubeConfig(config.ControlConfig.Runtime.ServerCA, config)
}

func startOnAPIServerReady(ctx context.Context, config *Config) {
	select {
	case <-ctx.Done():
		return
	case <-config.ControlConfig.Runtime.APIServerReady:
		if err := runControllers(ctx, config); err != nil {
			logrus.Fatalf("failed to start controllers: %v", err)
		}
	}
}

func runControllers(ctx context.Context, config *Config) error {
	controlConfig := &config.ControlConfig

	sc, err := NewContext(ctx, config, true)
	if err != nil {
		return errors.Wrap(err, "failed to create new server context")
	}

	controlConfig.Runtime.StartupHooksWg.Wait()
	if err := stageFiles(ctx, sc, controlConfig); err != nil {
		return errors.Wrap(err, "failed to stage files")
	}

	// run migration before we set controlConfig.Runtime.Core
	if err := nodepassword.MigrateFile(
		sc.Core.Core().V1().Secret(),
		sc.Core.Core().V1().Node(),
		controlConfig.Runtime.NodePasswdFile); err != nil {
		logrus.Warn(errors.Wrap(err, "error migrating node-password file"))
	}
	controlConfig.Runtime.K8s = sc.K8s
	controlConfig.Runtime.K3s = sc.K3s
	controlConfig.Runtime.Event = sc.Event
	controlConfig.Runtime.Core = sc.Core

	for name, cb := range controlConfig.Runtime.ClusterControllerStarts {
		go runOrDie(ctx, name, cb)
	}

	for _, controller := range config.Controllers {
		if err := controller(ctx, sc); err != nil {
			return errors.Wrapf(err, "failed to start %s controller", util.GetFunctionName(controller))
		}
	}

	if err := sc.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start wranger controllers")
	}

	if !controlConfig.DisableAPIServer {
		controlConfig.Runtime.LeaderElectedClusterControllerStarts[version.Program] = func(ctx context.Context) {
			apiserverControllers(ctx, sc, config)
		}
	}

	go setNodeLabelsAndAnnotations(ctx, sc.Core.Core().V1().Node(), config)

	go setClusterDNSConfig(ctx, config, sc.Core.Core().V1().ConfigMap())

	if controlConfig.NoLeaderElect {
		for name, cb := range controlConfig.Runtime.LeaderElectedClusterControllerStarts {
			go runOrDie(ctx, name, cb)
		}
	} else {
		for name, cb := range controlConfig.Runtime.LeaderElectedClusterControllerStarts {
			go leader.RunOrDie(ctx, "", name, sc.K8s, cb)
		}
	}

	return nil
}

// apiServerControllers starts the core controllers, as well as the leader-elected controllers
// that should only run on a control-plane node.
func apiserverControllers(ctx context.Context, sc *Context, config *Config) {
	if err := coreControllers(ctx, sc, config); err != nil {
		panic(err)
	}
	for _, controller := range config.LeaderControllers {
		if err := controller(ctx, sc); err != nil {
			panic(errors.Wrapf(err, "failed to start %s leader controller", util.GetFunctionName(controller)))
		}
	}

	// Re-run informer factory startup after core and leader-elected controllers have started.
	// Additional caches may need to start for the newly added OnChange/OnRemove callbacks.
	if err := sc.Start(ctx); err != nil {
		panic(errors.Wrap(err, "failed to start wranger controllers"))
	}
}

// runOrDie is similar to leader.RunOrDie, except that it runs the callback
// immediately, without performing leader election.
func runOrDie(ctx context.Context, name string, cb leader.Callback) {
	defer func() {
		if err := recover(); err != nil {
			logrus.WithField("stack", string(debug.Stack())).Fatalf("%s controller panic: %v", name, err)
		}
	}()
	cb(ctx)
	<-ctx.Done()
}

// coreControllers starts the following controllers, if they are enabled:
// * Node controller (manages nodes passwords and coredns hosts file)
// * Helm controller
// * Secrets encryption
// * Rootless ports
// These controllers should only be run on nodes with a local apiserver
func coreControllers(ctx context.Context, sc *Context, config *Config) error {
	if err := node.Register(ctx,
		!config.ControlConfig.Skips["coredns"],
		sc.Core.Core().V1().Secret(),
		sc.Core.Core().V1().ConfigMap(),
		sc.Core.Core().V1().Node()); err != nil {
		return err
	}

	// apply SystemDefaultRegistry setting to Helm before starting controllers
	if config.ControlConfig.HelmJobImage != "" {
		helmchart.DefaultJobImage = config.ControlConfig.HelmJobImage
	} else if config.ControlConfig.SystemDefaultRegistry != "" {
		helmchart.DefaultJobImage = config.ControlConfig.SystemDefaultRegistry + "/" + helmchart.DefaultJobImage
	}

	if !config.ControlConfig.DisableHelmController {
		restConfig, err := util.GetRESTConfig(config.ControlConfig.Runtime.KubeConfigSupervisor)
		if err != nil {
			return err
		}
		restConfig.UserAgent = util.GetUserAgent(helmcommon.Name)

		k8s, err := clientset.NewForConfig(restConfig)
		if err != nil {
			return err
		}

		apply := apply.New(k8s, apply.NewClientFactory(restConfig)).WithDynamicLookup().WithSetOwnerReference(false, false)
		helm := sc.Helm.WithAgent(restConfig.UserAgent)
		batch := sc.Batch.WithAgent(restConfig.UserAgent)
		auth := sc.Auth.WithAgent(restConfig.UserAgent)
		core := sc.Core.WithAgent(restConfig.UserAgent)
		helmchart.Register(ctx,
			metav1.NamespaceAll,
			helmcommon.Name,
			"cluster-admin",
			strconv.Itoa(config.ControlConfig.APIServerPort),
			k8s,
			apply,
			util.BuildControllerEventRecorder(k8s, helmcommon.Name, metav1.NamespaceAll),
			helm.V1().HelmChart(),
			helm.V1().HelmChart().Cache(),
			helm.V1().HelmChartConfig(),
			helm.V1().HelmChartConfig().Cache(),
			batch.V1().Job(),
			batch.V1().Job().Cache(),
			auth.V1().ClusterRoleBinding(),
			core.V1().ServiceAccount(),
			core.V1().ConfigMap(),
			core.V1().Secret())
	}

	if config.ControlConfig.Rootless {
		return rootlessports.Register(ctx,
			sc.Core.Core().V1().Service(),
			!config.ControlConfig.DisableServiceLB,
			config.ControlConfig.HTTPSPort)
	}

	return nil
}

func stageFiles(ctx context.Context, sc *Context, controlConfig *config.Control) error {
	if controlConfig.DisableAPIServer {
		return nil
	}
	dataDir := filepath.Join(controlConfig.DataDir, "static")
	if err := static.Stage(dataDir); err != nil {
		return err
	}
	dataDir = filepath.Join(controlConfig.DataDir, "manifests")

	dnsIPFamilyPolicy := "SingleStack"
	if len(controlConfig.ClusterDNSs) > 1 {
		dnsIPFamilyPolicy = "RequireDualStack"
	}

	templateVars := map[string]string{
		"%{CLUSTER_DNS}%":                 controlConfig.ClusterDNS.String(),
		"%{CLUSTER_DNS_LIST}%":            fmt.Sprintf("[%s]", util.JoinIPs(controlConfig.ClusterDNSs)),
		"%{CLUSTER_DNS_IPFAMILYPOLICY}%":  dnsIPFamilyPolicy,
		"%{CLUSTER_DOMAIN}%":              controlConfig.ClusterDomain,
		"%{DEFAULT_LOCAL_STORAGE_PATH}%":  controlConfig.DefaultLocalStoragePath,
		"%{SYSTEM_DEFAULT_REGISTRY}%":     registryTemplate(controlConfig.SystemDefaultRegistry),
		"%{SYSTEM_DEFAULT_REGISTRY_RAW}%": controlConfig.SystemDefaultRegistry,
		"%{PREFERRED_ADDRESS_TYPES}%":     addrTypesPrioTemplate(controlConfig.FlannelExternalIP),
	}

	skip := controlConfig.Skips
	if err := deploy.Stage(dataDir, templateVars, skip); err != nil {
		return err
	}

	restConfig, err := util.GetRESTConfig(controlConfig.Runtime.KubeConfigSupervisor)
	if err != nil {
		return err
	}
	restConfig.UserAgent = util.GetUserAgent("deploy")

	k8s, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	apply := apply.New(k8s, apply.NewClientFactory(restConfig)).WithDynamicLookup()
	k3s := sc.K3s.WithAgent(restConfig.UserAgent)

	return deploy.WatchFiles(ctx,
		k8s,
		apply,
		k3s.V1().Addon(),
		controlConfig.Disables,
		dataDir)
}

// registryTemplate behaves like the system_default_registry template in Rancher helm charts,
// and returns the registry value with a trailing forward slash if the registry string is not empty.
// If it is empty, it is passed through as a no-op.
func registryTemplate(registry string) string {
	if registry == "" {
		return registry
	}
	return registry + "/"
}

// addressTypesTemplate prioritizes ExternalIP addresses if we are in the multi-cloud env where
// cluster traffic flows over the external IPs only
func addrTypesPrioTemplate(flannelExternal bool) string {
	if flannelExternal {
		return "ExternalIP,InternalIP,Hostname"
	}

	return "InternalIP,ExternalIP,Hostname"
}

func HomeKubeConfig(write, rootless bool) (string, error) {
	if write {
		if permissions.IsPrivileged() == nil && !rootless {
			return datadir.GlobalConfig, nil
		}
		return resolvehome.Resolve(datadir.HomeConfig)
	}

	if _, err := os.Stat(datadir.GlobalConfig); err == nil {
		return datadir.GlobalConfig, nil
	}

	return resolvehome.Resolve(datadir.HomeConfig)
}

func printTokens(config *config.Control) error {
	var serverTokenFile string
	if config.Runtime.ServerToken != "" {
		serverTokenFile = filepath.Join(config.DataDir, "token")
		if err := handlers.WriteToken(config.Runtime.ServerToken, serverTokenFile, config.Runtime.ServerCA); err != nil {
			return err
		}

		// backwards compatibility
		np := filepath.Join(config.DataDir, "node-token")
		if !isSymlink(np) {
			if err := os.RemoveAll(np); err != nil {
				return err
			}
			if err := os.Symlink(serverTokenFile, np); err != nil {
				return err
			}
		}

		logrus.Infof("Server node token is available at %s", serverTokenFile)
		printToken(config.SupervisorPort, config.BindAddressOrLoopback(true, true), "To join server node to cluster:", "server", "SERVER_NODE_TOKEN")
	}

	var agentTokenFile string
	if config.Runtime.AgentToken != "" {
		if config.AgentToken != "" {
			agentTokenFile = filepath.Join(config.DataDir, "agent-token")
			if isSymlink(agentTokenFile) {
				if err := os.RemoveAll(agentTokenFile); err != nil {
					return err
				}
			}
			if err := handlers.WriteToken(config.Runtime.AgentToken, agentTokenFile, config.Runtime.ServerCA); err != nil {
				return err
			}
		} else if serverTokenFile != "" {
			agentTokenFile = filepath.Join(config.DataDir, "agent-token")
			if !isSymlink(agentTokenFile) {
				if err := os.RemoveAll(agentTokenFile); err != nil {
					return err
				}
				if err := os.Symlink(serverTokenFile, agentTokenFile); err != nil {
					return err
				}
			}
		}
	}

	if agentTokenFile != "" {
		logrus.Infof("Agent node token is available at %s", agentTokenFile)
		printToken(config.SupervisorPort, config.BindAddressOrLoopback(true, true), "To join agent node to cluster:", "agent", "AGENT_NODE_TOKEN")
	}

	return nil
}

func writeKubeConfig(certs string, config *Config) error {
	ip := config.ControlConfig.BindAddressOrLoopback(false, true)
	port := config.ControlConfig.HTTPSPort
	// on servers without a local apiserver, tunnel access via the loadbalancer
	if config.ControlConfig.DisableAPIServer {
		ip = config.ControlConfig.Loopback(true)
		port = config.ControlConfig.APIServerPort
	}
	url := fmt.Sprintf("https://%s:%d", ip, port)
	kubeConfig, err := HomeKubeConfig(true, config.ControlConfig.Rootless)
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
			logrus.Errorf("Failed to remove kubeconfig symlink")
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
			logrus.Errorf("Failed to set %s to mode %s: %v", kubeConfig, os.FileMode(mode), err)
		}
	} else {
		util.SetFileModeForPath(kubeConfig, os.FileMode(0600))
	}

	if config.ControlConfig.KubeConfigGroup != "" {
		err := util.SetFileGroupForPath(kubeConfig, config.ControlConfig.KubeConfigGroup)
		if err != nil {
			logrus.Errorf("Failed to set %s to group %s: %v", kubeConfig, config.ControlConfig.KubeConfigGroup, err)
		}
	}

	if kubeConfigSymlink != kubeConfig {
		if err := writeConfigSymlink(kubeConfig, kubeConfigSymlink); err != nil {
			logrus.Errorf("Failed to write kubeconfig symlink: %v", err)
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

	config.DataDir, err = ResolveDataDir(config.DataDir)
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

func printToken(httpsPort int, advertiseIP, prefix, cmd, varName string) {
	logrus.Infof("%s %s %s -s https://%s:%d -t ${%s}", prefix, version.Program, cmd, advertiseIP, httpsPort, varName)
}

func setNoProxyEnv(config *config.Control) error {
	splitter := func(c rune) bool {
		return c == ','
	}
	envList := []string{}
	envList = append(envList, strings.FieldsFunc(os.Getenv("NO_PROXY"), splitter)...)
	envList = append(envList, strings.FieldsFunc(os.Getenv("no_proxy"), splitter)...)
	envList = append(envList,
		".svc",
		"."+config.ClusterDomain,
		util.JoinIPNets(config.ClusterIPRanges),
		util.JoinIPNets(config.ServiceIPRanges),
	)
	os.Unsetenv("no_proxy")
	return os.Setenv("NO_PROXY", strings.Join(envList, ","))
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

func setNodeLabelsAndAnnotations(ctx context.Context, nodes v1.NodeClient, config *Config) error {
	if config.DisableAgent || config.ControlConfig.DisableAPIServer {
		return nil
	}
	for {
		nodeName := os.Getenv("NODE_NAME")
		if nodeName == "" {
			logrus.Info("Waiting for control-plane node agent startup")
			time.Sleep(1 * time.Second)
			continue
		}
		node, err := nodes.Get(nodeName, metav1.GetOptions{})
		if err != nil {
			logrus.Infof("Waiting for control-plane node %s startup: %v", nodeName, err)
			time.Sleep(1 * time.Second)
			continue
		}
		if node.Labels == nil {
			node.Labels = make(map[string]string)
		}
		v, ok := node.Labels[util.ControlPlaneRoleLabelKey]
		if !ok || v != "true" {
			node.Labels[util.ControlPlaneRoleLabelKey] = "true"
			node.Labels[util.MasterRoleLabelKey] = "true"
		}

		if config.ControlConfig.EncryptSecrets {
			if err = secretsencrypt.BootstrapEncryptionHashAnnotation(node, config.ControlConfig.Runtime); err != nil {
				logrus.Infof("Unable to set encryption hash annotation %s", err.Error())
				break
			}
		}

		_, err = nodes.Update(node)
		if err == nil {
			logrus.Infof("Labels and annotations have been set successfully on node: %s", nodeName)
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

func setClusterDNSConfig(ctx context.Context, config *Config, configMap v1.ConfigMapClient) error {
	if config.ControlConfig.DisableAPIServer {
		return nil
	}
	// check if configmap already exists
	_, err := configMap.Get("kube-system", "cluster-dns", metav1.GetOptions{})
	if err == nil {
		logrus.Infof("Cluster dns configmap already exists")
		return nil
	}
	clusterDNS := config.ControlConfig.ClusterDNS
	clusterDomain := config.ControlConfig.ClusterDomain
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
			logrus.Infof("Cluster dns configmap has been set successfully")
			break
		}
		logrus.Infof("Waiting for control-plane dns startup: %v", err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil
}
