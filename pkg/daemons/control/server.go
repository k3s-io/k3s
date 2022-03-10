package control

import (
	"context"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/cluster"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/daemons/control/deps"
	"github.com/rancher/k3s/pkg/daemons/executor"
	"github.com/rancher/k3s/pkg/util"
	"github.com/rancher/k3s/pkg/version"
	"github.com/rancher/wrangler-api/pkg/generated/controllers/rbac"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/kubeapiserver/authorizer/modes"
	proxyutil "k8s.io/kubernetes/pkg/proxy/util"

	// for client metric registration
	_ "k8s.io/component-base/metrics/prometheus/restclient"
)

var localhostIP = net.ParseIP("127.0.0.1")

func Server(ctx context.Context, cfg *config.Control) error {
	rand.Seed(time.Now().UTC().UnixNano())

	if err := prepare(ctx, cfg); err != nil {
		return errors.Wrap(err, "preparing server")
	}

	cfg.Runtime.Tunnel = setupTunnel()
	proxyutil.DisableProxyHostnameCheck = true

	basicAuth, err := basicAuthenticator(cfg.Runtime.PasswdFile)
	if err != nil {
		return err
	}
	cfg.Runtime.Authenticator = basicAuth

	if !cfg.DisableAPIServer {
		go waitForAPIServerHandlers(ctx, cfg.Runtime)

		if err := apiServer(ctx, cfg); err != nil {
			return err
		}

		if err := waitForAPIServerInBackground(ctx, cfg.Runtime); err != nil {
			return err
		}
	}

	if !cfg.DisableScheduler {
		if err := scheduler(cfg); err != nil {
			return err
		}
	}
	if !cfg.DisableControllerManager {
		if err := controllerManager(cfg); err != nil {
			return err
		}
	}

	if !cfg.DisableCCM {
		if err := cloudControllerManager(ctx, cfg); err != nil {
			return err
		}
	}

	return nil
}

func controllerManager(cfg *config.Control) error {
	runtime := cfg.Runtime
	argsMap := map[string]string{
		"kubeconfig":                       runtime.KubeConfigController,
		"service-account-private-key-file": runtime.ServiceKey,
		"allocate-node-cidrs":              "true",
		"cluster-cidr":                     util.JoinIPNets(cfg.ClusterIPRanges),
		"root-ca-file":                     runtime.ServerCA,
		"port":                             "10252",
		"profiling":                        "false",
		"address":                          localhostIP.String(),
		"bind-address":                     localhostIP.String(),
		"secure-port":                      "0",
		"use-service-account-credentials":  "true",
		"cluster-signing-kube-apiserver-client-cert-file": runtime.ClientCA,
		"cluster-signing-kube-apiserver-client-key-file":  runtime.ClientCAKey,
		"cluster-signing-kubelet-client-cert-file":        runtime.ClientCA,
		"cluster-signing-kubelet-client-key-file":         runtime.ClientCAKey,
		"cluster-signing-kubelet-serving-cert-file":       runtime.ServerCA,
		"cluster-signing-kubelet-serving-key-file":        runtime.ServerCAKey,
		"cluster-signing-legacy-unknown-cert-file":        runtime.ClientCA,
		"cluster-signing-legacy-unknown-key-file":         runtime.ClientCAKey,
	}
	if cfg.NoLeaderElect {
		argsMap["leader-elect"] = "false"
	}
	if !cfg.DisableCCM {
		argsMap["configure-cloud-routes"] = "false"
		argsMap["controllers"] = "*,-service,-route,-cloud-node-lifecycle"
	}

	args := config.GetArgsList(argsMap, cfg.ExtraControllerArgs)
	logrus.Infof("Running kube-controller-manager %s", config.ArgString(args))

	return executor.ControllerManager(cfg.Runtime.APIServerReady, args)
}

func scheduler(cfg *config.Control) error {
	runtime := cfg.Runtime
	argsMap := map[string]string{
		"kubeconfig":   runtime.KubeConfigScheduler,
		"port":         "10251",
		"address":      "127.0.0.1",
		"bind-address": "127.0.0.1",
		"secure-port":  "0",
		"profiling":    "false",
	}
	if cfg.NoLeaderElect {
		argsMap["leader-elect"] = "false"
	}
	args := config.GetArgsList(argsMap, cfg.ExtraSchedulerAPIArgs)

	logrus.Infof("Running kube-scheduler %s", config.ArgString(args))
	return executor.Scheduler(cfg.Runtime.APIServerReady, args)
}

func apiServer(ctx context.Context, cfg *config.Control) error {
	runtime := cfg.Runtime

	argsMap := make(map[string]string)
	setupStorageBackend(argsMap, cfg)

	certDir := filepath.Join(cfg.DataDir, "tls", "temporary-certs")
	os.MkdirAll(certDir, 0700)

	argsMap["cert-dir"] = certDir
	argsMap["allow-privileged"] = "true"
	argsMap["authorization-mode"] = strings.Join([]string{modes.ModeNode, modes.ModeRBAC}, ",")
	argsMap["service-account-signing-key-file"] = runtime.ServiceKey
	argsMap["service-cluster-ip-range"] = util.JoinIPNets(cfg.ServiceIPRanges)
	argsMap["service-node-port-range"] = cfg.ServiceNodePortRange.String()
	argsMap["advertise-port"] = strconv.Itoa(cfg.AdvertisePort)
	if cfg.AdvertiseIP != "" {
		argsMap["advertise-address"] = cfg.AdvertiseIP
	}
	argsMap["insecure-port"] = "0"
	argsMap["secure-port"] = strconv.Itoa(cfg.APIServerPort)
	if cfg.APIServerBindAddress == "" {
		argsMap["bind-address"] = localhostIP.String()
	} else {
		argsMap["bind-address"] = cfg.APIServerBindAddress
	}
	argsMap["tls-cert-file"] = runtime.ServingKubeAPICert
	argsMap["tls-private-key-file"] = runtime.ServingKubeAPIKey
	argsMap["service-account-key-file"] = runtime.ServiceKey
	argsMap["service-account-issuer"] = "https://kubernetes.default.svc." + cfg.ClusterDomain
	argsMap["api-audiences"] = "https://kubernetes.default.svc." + cfg.ClusterDomain + "," + version.Program
	argsMap["kubelet-certificate-authority"] = runtime.ServerCA
	argsMap["kubelet-client-certificate"] = runtime.ClientKubeAPICert
	argsMap["kubelet-client-key"] = runtime.ClientKubeAPIKey
	argsMap["requestheader-client-ca-file"] = runtime.RequestHeaderCA
	argsMap["requestheader-allowed-names"] = deps.RequestHeaderCN
	argsMap["proxy-client-cert-file"] = runtime.ClientAuthProxyCert
	argsMap["proxy-client-key-file"] = runtime.ClientAuthProxyKey
	argsMap["requestheader-extra-headers-prefix"] = "X-Remote-Extra-"
	argsMap["requestheader-group-headers"] = "X-Remote-Group"
	argsMap["requestheader-username-headers"] = "X-Remote-User"
	argsMap["client-ca-file"] = runtime.ClientCA
	argsMap["enable-admission-plugins"] = "NodeRestriction"
	argsMap["anonymous-auth"] = "false"
	argsMap["profiling"] = "false"
	if cfg.EncryptSecrets {
		argsMap["encryption-provider-config"] = runtime.EncryptionConfig
	}
	args := config.GetArgsList(argsMap, cfg.ExtraAPIArgs)

	logrus.Infof("Running kube-apiserver %s", config.ArgString(args))

	return executor.APIServer(ctx, runtime.ETCDReady, args)
}

func defaults(config *config.Control) {
	if config.ClusterIPRange == nil {
		_, clusterIPNet, _ := net.ParseCIDR("10.42.0.0/16")
		config.ClusterIPRange = clusterIPNet
	}

	if config.ServiceIPRange == nil {
		_, serviceIPNet, _ := net.ParseCIDR("10.43.0.0/16")
		config.ServiceIPRange = serviceIPNet
	}

	if len(config.ClusterDNS) == 0 {
		config.ClusterDNS = net.ParseIP("10.43.0.10")
	}

	if config.AdvertisePort == 0 {
		config.AdvertisePort = config.HTTPSPort
	}

	if config.APIServerPort == 0 {
		if config.HTTPSPort != 0 {
			config.APIServerPort = config.HTTPSPort + 1
		} else {
			config.APIServerPort = 6444
		}
	}

	if config.DataDir == "" {
		config.DataDir = "./management-state"
	}
}

func prepare(ctx context.Context, config *config.Control) error {
	var err error

	defaults(config)

	if err := os.MkdirAll(config.DataDir, 0700); err != nil {
		return err
	}

	config.DataDir, err = filepath.Abs(config.DataDir)
	if err != nil {
		return err
	}

	os.MkdirAll(filepath.Join(config.DataDir, "tls"), 0700)
	os.MkdirAll(filepath.Join(config.DataDir, "cred"), 0700)

	runtime := config.Runtime
	runtime.ClientCA = filepath.Join(config.DataDir, "tls", "client-ca.crt")
	runtime.ClientCAKey = filepath.Join(config.DataDir, "tls", "client-ca.key")
	runtime.ServerCA = filepath.Join(config.DataDir, "tls", "server-ca.crt")
	runtime.ServerCAKey = filepath.Join(config.DataDir, "tls", "server-ca.key")
	runtime.RequestHeaderCA = filepath.Join(config.DataDir, "tls", "request-header-ca.crt")
	runtime.RequestHeaderCAKey = filepath.Join(config.DataDir, "tls", "request-header-ca.key")
	runtime.IPSECKey = filepath.Join(config.DataDir, "cred", "ipsec.psk")

	runtime.ServiceKey = filepath.Join(config.DataDir, "tls", "service.key")
	runtime.PasswdFile = filepath.Join(config.DataDir, "cred", "passwd")
	runtime.NodePasswdFile = filepath.Join(config.DataDir, "cred", "node-passwd")

	runtime.KubeConfigAdmin = filepath.Join(config.DataDir, "cred", "admin.kubeconfig")
	runtime.KubeConfigController = filepath.Join(config.DataDir, "cred", "controller.kubeconfig")
	runtime.KubeConfigScheduler = filepath.Join(config.DataDir, "cred", "scheduler.kubeconfig")
	runtime.KubeConfigAPIServer = filepath.Join(config.DataDir, "cred", "api-server.kubeconfig")
	runtime.KubeConfigCloudController = filepath.Join(config.DataDir, "cred", "cloud-controller.kubeconfig")

	runtime.ClientAdminCert = filepath.Join(config.DataDir, "tls", "client-admin.crt")
	runtime.ClientAdminKey = filepath.Join(config.DataDir, "tls", "client-admin.key")
	runtime.ClientControllerCert = filepath.Join(config.DataDir, "tls", "client-controller.crt")
	runtime.ClientControllerKey = filepath.Join(config.DataDir, "tls", "client-controller.key")
	runtime.ClientCloudControllerCert = filepath.Join(config.DataDir, "tls", "client-"+version.Program+"-cloud-controller.crt")
	runtime.ClientCloudControllerKey = filepath.Join(config.DataDir, "tls", "client-"+version.Program+"-cloud-controller.key")
	runtime.ClientSchedulerCert = filepath.Join(config.DataDir, "tls", "client-scheduler.crt")
	runtime.ClientSchedulerKey = filepath.Join(config.DataDir, "tls", "client-scheduler.key")
	runtime.ClientKubeAPICert = filepath.Join(config.DataDir, "tls", "client-kube-apiserver.crt")
	runtime.ClientKubeAPIKey = filepath.Join(config.DataDir, "tls", "client-kube-apiserver.key")
	runtime.ClientKubeProxyCert = filepath.Join(config.DataDir, "tls", "client-kube-proxy.crt")
	runtime.ClientKubeProxyKey = filepath.Join(config.DataDir, "tls", "client-kube-proxy.key")
	runtime.ClientK3sControllerCert = filepath.Join(config.DataDir, "tls", "client-"+version.Program+"-controller.crt")
	runtime.ClientK3sControllerKey = filepath.Join(config.DataDir, "tls", "client-"+version.Program+"-controller.key")

	runtime.ServingKubeAPICert = filepath.Join(config.DataDir, "tls", "serving-kube-apiserver.crt")
	runtime.ServingKubeAPIKey = filepath.Join(config.DataDir, "tls", "serving-kube-apiserver.key")

	runtime.ClientKubeletKey = filepath.Join(config.DataDir, "tls", "client-kubelet.key")
	runtime.ServingKubeletKey = filepath.Join(config.DataDir, "tls", "serving-kubelet.key")

	runtime.ClientAuthProxyCert = filepath.Join(config.DataDir, "tls", "client-auth-proxy.crt")
	runtime.ClientAuthProxyKey = filepath.Join(config.DataDir, "tls", "client-auth-proxy.key")

	runtime.ETCDServerCA = filepath.Join(config.DataDir, "tls", "etcd", "server-ca.crt")
	runtime.ETCDServerCAKey = filepath.Join(config.DataDir, "tls", "etcd", "server-ca.key")
	runtime.ETCDPeerCA = filepath.Join(config.DataDir, "tls", "etcd", "peer-ca.crt")
	runtime.ETCDPeerCAKey = filepath.Join(config.DataDir, "tls", "etcd", "peer-ca.key")
	runtime.ServerETCDCert = filepath.Join(config.DataDir, "tls", "etcd", "server-client.crt")
	runtime.ServerETCDKey = filepath.Join(config.DataDir, "tls", "etcd", "server-client.key")
	runtime.PeerServerClientETCDCert = filepath.Join(config.DataDir, "tls", "etcd", "peer-server-client.crt")
	runtime.PeerServerClientETCDKey = filepath.Join(config.DataDir, "tls", "etcd", "peer-server-client.key")
	runtime.ClientETCDCert = filepath.Join(config.DataDir, "tls", "etcd", "client.crt")
	runtime.ClientETCDKey = filepath.Join(config.DataDir, "tls", "etcd", "client.key")

	if config.EncryptSecrets {
		runtime.EncryptionConfig = filepath.Join(config.DataDir, "cred", "encryption-config.json")
	}
	deps.CreateRuntimeCertFiles(config)

	cluster := cluster.New(config)

	if err := cluster.Bootstrap(ctx, false); err != nil {
		return err
	}

	if err := deps.GenServerDeps(config); err != nil {
		return err
	}

	ready, err := cluster.Start(ctx)
	if err != nil {
		return err
	}

	config.Runtime.ETCDReady = ready
	config.Runtime.EtcdConfig = cluster.EtcdConfig
	return nil
}

func setupStorageBackend(argsMap map[string]string, cfg *config.Control) {
	argsMap["storage-backend"] = "etcd3"
	// specify the endpoints
	if len(cfg.Datastore.Endpoint) > 0 {
		argsMap["etcd-servers"] = cfg.Datastore.Endpoint
	}
	// storage backend tls configuration
	if len(cfg.Datastore.CAFile) > 0 {
		argsMap["etcd-cafile"] = cfg.Datastore.CAFile
	}
	if len(cfg.Datastore.CertFile) > 0 {
		argsMap["etcd-certfile"] = cfg.Datastore.CertFile
	}
	if len(cfg.Datastore.KeyFile) > 0 {
		argsMap["etcd-keyfile"] = cfg.Datastore.KeyFile
	}
}

func cloudControllerManager(ctx context.Context, cfg *config.Control) error {
	runtime := cfg.Runtime
	argsMap := map[string]string{
		"profiling":                    "false",
		"allocate-node-cidrs":          "true",
		"cloud-provider":               version.Program,
		"cluster-cidr":                 util.JoinIPNets(cfg.ClusterIPRanges),
		"configure-cloud-routes":       "false",
		"kubeconfig":                   runtime.KubeConfigCloudController,
		"node-status-update-frequency": "1m0s",
		"bind-address":                 "127.0.0.1",
		"port":                         "0",
	}
	if cfg.NoLeaderElect {
		argsMap["leader-elect"] = "false"
	}
	args := config.GetArgsList(argsMap, cfg.ExtraCloudControllerArgs)

	logrus.Infof("Running cloud-controller-manager %s", config.ArgString(args))

	ccmRBACReady := make(chan struct{})

	go func() {
		defer close(ccmRBACReady)

	apiReadyLoop:
		for {
			select {
			case <-ctx.Done():
				return
			case <-cfg.Runtime.APIServerReady:
				break apiReadyLoop
			case <-time.After(30 * time.Second):
				logrus.Infof("Waiting for API server to become available")
			}
		}

		logrus.Infof("Waiting for cloud-controller-manager privileges to become available")
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-promise(func() error { return checkForCloudControllerPrivileges(cfg.Runtime, 5*time.Second) }):
				if err != nil {
					logrus.Infof("Waiting for cloud-controller-manager privileges to become available")
					continue
				}
				return
			}
		}
	}()

	return executor.CloudControllerManager(ccmRBACReady, args)
}

func checkForCloudControllerPrivileges(runtime *config.ControlRuntime, timeout time.Duration) error {
	restConfig, err := clientcmd.BuildConfigFromFlags("", runtime.KubeConfigAdmin)
	if err != nil {
		return err
	}
	err = wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		crb := rbac.NewFactoryFromConfigOrDie(restConfig).Rbac().V1().ClusterRoleBinding()
		_, err = crb.Get(version.Program+"-cloud-controller-manager", metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})

	if err != nil {
		logrus.Errorf("error encountered waitng for cloud-controller-manager privileges: %v", err)
	}
	return nil
}

func waitForAPIServerHandlers(ctx context.Context, runtime *config.ControlRuntime) {
	auth, handler, err := executor.APIServerHandlers()
	if err != nil {
		logrus.Fatalf("Failed to get request handlers from apiserver: %v", err)
	}
	runtime.Authenticator = combineAuthenticators(runtime.Authenticator, auth)
	runtime.APIServer = handler
}

func waitForAPIServerInBackground(ctx context.Context, runtime *config.ControlRuntime) error {
	restConfig, err := clientcmd.BuildConfigFromFlags("", runtime.KubeConfigAdmin)
	if err != nil {
		return err
	}

	k8sClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	done := make(chan struct{})
	runtime.APIServerReady = done

	go func() {
		defer close(done)

	etcdLoop:
		for {
			select {
			case <-ctx.Done():
				return
			case <-runtime.ETCDReady:
				break etcdLoop
			case <-time.After(30 * time.Second):
				logrus.Infof("Waiting for etcd server to become available")
			}
		}

		logrus.Infof("Waiting for API server to become available")
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-promise(func() error { return util.WaitForAPIServerReady(ctx, k8sClient, 30*time.Second) }):
				if err != nil {
					logrus.Infof("Waiting for API server to become available")
					continue
				}
				return
			}
		}
	}()

	return nil
}

func promise(f func() error) <-chan error {
	c := make(chan error, 1)
	go func() {
		c <- f()
		close(c)
	}()
	return c
}
