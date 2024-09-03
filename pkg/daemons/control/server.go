package control

import (
	"context"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/k3s-io/k3s/pkg/authenticator"
	"github.com/k3s-io/k3s/pkg/cluster"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/control/deps"
	"github.com/k3s-io/k3s/pkg/daemons/executor"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logsapi "k8s.io/component-base/logs/api/v1"
	"k8s.io/kubernetes/pkg/kubeapiserver/authorizer/modes"
	"k8s.io/kubernetes/pkg/registry/core/node"

	// for client metric registration
	_ "k8s.io/component-base/metrics/prometheus/restclient"
)

func Server(ctx context.Context, cfg *config.Control) error {
	rand.Seed(time.Now().UTC().UnixNano())

	logsapi.ReapplyHandling = logsapi.ReapplyHandlingIgnoreUnchanged
	if err := prepare(ctx, cfg); err != nil {
		return errors.Wrap(err, "preparing server")
	}

	tunnel, err := setupTunnel(ctx, cfg)
	if err != nil {
		return errors.Wrap(err, "setup tunnel server")
	}
	cfg.Runtime.Tunnel = tunnel

	node.DisableProxyHostnameCheck = true

	authArgs := []string{
		"--basic-auth-file=" + cfg.Runtime.PasswdFile,
		"--client-ca-file=" + cfg.Runtime.ClientCA,
	}
	auth, err := authenticator.FromArgs(authArgs)
	if err != nil {
		return err
	}
	cfg.Runtime.Authenticator = auth

	if !cfg.DisableAPIServer {
		go waitForAPIServerHandlers(ctx, cfg.Runtime)

		if err := apiServer(ctx, cfg); err != nil {
			return err
		}
	}

	// Wait for an apiserver to become available before starting additional controllers,
	// even if we're not running an apiserver locally.
	if err := waitForAPIServerInBackground(ctx, cfg.Runtime); err != nil {
		return err
	}

	if !cfg.DisableScheduler {
		if err := scheduler(ctx, cfg); err != nil {
			return err
		}
	}
	if !cfg.DisableControllerManager {
		if err := controllerManager(ctx, cfg); err != nil {
			return err
		}
	}

	if !cfg.DisableCCM || !cfg.DisableServiceLB {
		if err := cloudControllerManager(ctx, cfg); err != nil {
			return err
		}
	}

	return nil
}

func controllerManager(ctx context.Context, cfg *config.Control) error {
	runtime := cfg.Runtime
	argsMap := map[string]string{
		"controllers":                      "*,tokencleaner",
		"kubeconfig":                       runtime.KubeConfigController,
		"authorization-kubeconfig":         runtime.KubeConfigController,
		"authentication-kubeconfig":        runtime.KubeConfigController,
		"service-account-private-key-file": runtime.ServiceCurrentKey,
		"allocate-node-cidrs":              "true",
		"service-cluster-ip-range":         util.JoinIPNets(cfg.ServiceIPRanges),
		"cluster-cidr":                     util.JoinIPNets(cfg.ClusterIPRanges),
		"root-ca-file":                     runtime.ServerCA,
		"profiling":                        "false",
		"bind-address":                     cfg.Loopback(false),
		"secure-port":                      "10257",
		"use-service-account-credentials":  "true",
		"cluster-signing-kube-apiserver-client-cert-file": runtime.SigningClientCA,
		"cluster-signing-kube-apiserver-client-key-file":  runtime.ClientCAKey,
		"cluster-signing-kubelet-client-cert-file":        runtime.SigningClientCA,
		"cluster-signing-kubelet-client-key-file":         runtime.ClientCAKey,
		"cluster-signing-kubelet-serving-cert-file":       runtime.SigningServerCA,
		"cluster-signing-kubelet-serving-key-file":        runtime.ServerCAKey,
		"cluster-signing-legacy-unknown-cert-file":        runtime.SigningServerCA,
		"cluster-signing-legacy-unknown-key-file":         runtime.ServerCAKey,
	}
	if cfg.NoLeaderElect {
		argsMap["leader-elect"] = "false"
	}
	if !cfg.DisableCCM {
		argsMap["configure-cloud-routes"] = "false"
		argsMap["controllers"] = argsMap["controllers"] + ",-service,-route,-cloud-node-lifecycle"
	}

	if cfg.VLevel != 0 {
		argsMap["v"] = strconv.Itoa(cfg.VLevel)
	}
	if cfg.VModule != "" {
		argsMap["vmodule"] = cfg.VModule
	}

	args := config.GetArgs(argsMap, cfg.ExtraControllerArgs)
	logrus.Infof("Running kube-controller-manager %s", config.ArgString(args))

	return executor.ControllerManager(ctx, cfg.Runtime.APIServerReady, args)
}

func scheduler(ctx context.Context, cfg *config.Control) error {
	runtime := cfg.Runtime
	argsMap := map[string]string{
		"kubeconfig":                runtime.KubeConfigScheduler,
		"authorization-kubeconfig":  runtime.KubeConfigScheduler,
		"authentication-kubeconfig": runtime.KubeConfigScheduler,
		"bind-address":              cfg.Loopback(false),
		"secure-port":               "10259",
		"profiling":                 "false",
	}
	if cfg.NoLeaderElect {
		argsMap["leader-elect"] = "false"
	}

	if cfg.VLevel != 0 {
		argsMap["v"] = strconv.Itoa(cfg.VLevel)
	}
	if cfg.VModule != "" {
		argsMap["vmodule"] = cfg.VModule
	}

	args := config.GetArgs(argsMap, cfg.ExtraSchedulerAPIArgs)

	logrus.Infof("Running kube-scheduler %s", config.ArgString(args))
	return executor.Scheduler(ctx, cfg.Runtime.APIServerReady, args)
}

func apiServer(ctx context.Context, cfg *config.Control) error {
	runtime := cfg.Runtime
	argsMap := map[string]string{}

	setupStorageBackend(argsMap, cfg)

	certDir := filepath.Join(cfg.DataDir, "tls", "temporary-certs")
	os.MkdirAll(certDir, 0700)

	argsMap["cert-dir"] = certDir
	argsMap["allow-privileged"] = "true"
	argsMap["enable-bootstrap-token-auth"] = "true"
	argsMap["authorization-mode"] = strings.Join([]string{modes.ModeNode, modes.ModeRBAC}, ",")
	argsMap["service-account-signing-key-file"] = runtime.ServiceCurrentKey
	argsMap["service-cluster-ip-range"] = util.JoinIPNets(cfg.ServiceIPRanges)
	argsMap["service-node-port-range"] = cfg.ServiceNodePortRange.String()
	argsMap["advertise-port"] = strconv.Itoa(cfg.AdvertisePort)
	if cfg.AdvertiseIP != "" {
		argsMap["advertise-address"] = cfg.AdvertiseIP
	}
	argsMap["secure-port"] = strconv.Itoa(cfg.APIServerPort)
	if cfg.APIServerBindAddress == "" {
		argsMap["bind-address"] = cfg.Loopback(false)
	} else {
		argsMap["bind-address"] = cfg.APIServerBindAddress
	}
	if cfg.EgressSelectorMode != config.EgressSelectorModeDisabled {
		argsMap["enable-aggregator-routing"] = "true"
		argsMap["egress-selector-config-file"] = runtime.EgressSelectorConfig
	}
	argsMap["tls-cert-file"] = runtime.ServingKubeAPICert
	argsMap["tls-private-key-file"] = runtime.ServingKubeAPIKey
	argsMap["service-account-key-file"] = runtime.ServiceKey
	argsMap["service-account-issuer"] = "https://kubernetes.default.svc." + cfg.ClusterDomain
	argsMap["api-audiences"] = "https://kubernetes.default.svc." + cfg.ClusterDomain + "," + version.Program
	argsMap["kubelet-certificate-authority"] = runtime.ServerCA
	argsMap["kubelet-client-certificate"] = runtime.ClientKubeAPICert
	argsMap["kubelet-client-key"] = runtime.ClientKubeAPIKey
	if cfg.FlannelExternalIP {
		argsMap["kubelet-preferred-address-types"] = "ExternalIP,InternalIP,Hostname"
	} else {
		argsMap["kubelet-preferred-address-types"] = "InternalIP,ExternalIP,Hostname"
	}
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
		argsMap["encryption-provider-config-automatic-reload"] = "true"
	}
	if cfg.VLevel != 0 {
		argsMap["v"] = strconv.Itoa(cfg.VLevel)
	}
	if cfg.VModule != "" {
		argsMap["vmodule"] = cfg.VModule
	}

	args := config.GetArgs(argsMap, cfg.ExtraAPIArgs)

	logrus.Infof("Running kube-apiserver %s", config.ArgString(args))

	return executor.APIServer(ctx, runtime.ETCDReady, args)
}

func defaults(config *config.Control) {
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

	os.MkdirAll(filepath.Join(config.DataDir, "etc"), 0700)
	os.MkdirAll(filepath.Join(config.DataDir, "tls"), 0700)
	os.MkdirAll(filepath.Join(config.DataDir, "cred"), 0700)

	deps.CreateRuntimeCertFiles(config)

	cluster := cluster.New(config)
	if err := cluster.Bootstrap(ctx, config.ClusterReset); err != nil {
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
	return nil
}

func setupStorageBackend(argsMap map[string]string, cfg *config.Control) {
	argsMap["storage-backend"] = "etcd3"
	// specify the endpoints
	if len(cfg.Datastore.Endpoint) > 0 {
		argsMap["etcd-servers"] = cfg.Datastore.Endpoint
	}
	// storage backend tls configuration
	if len(cfg.Datastore.BackendTLSConfig.CAFile) > 0 {
		argsMap["etcd-cafile"] = cfg.Datastore.BackendTLSConfig.CAFile
	}
	if len(cfg.Datastore.BackendTLSConfig.CertFile) > 0 {
		argsMap["etcd-certfile"] = cfg.Datastore.BackendTLSConfig.CertFile
	}
	if len(cfg.Datastore.BackendTLSConfig.KeyFile) > 0 {
		argsMap["etcd-keyfile"] = cfg.Datastore.BackendTLSConfig.KeyFile
	}
}

func cloudControllerManager(ctx context.Context, cfg *config.Control) error {
	runtime := cfg.Runtime
	argsMap := map[string]string{
		"profiling":                    "false",
		"allocate-node-cidrs":          "true",
		"leader-elect-resource-name":   version.Program + "-cloud-controller-manager",
		"cloud-provider":               version.Program,
		"cloud-config":                 runtime.CloudControllerConfig,
		"cluster-cidr":                 util.JoinIPNets(cfg.ClusterIPRanges),
		"configure-cloud-routes":       "false",
		"controllers":                  "*,-route",
		"kubeconfig":                   runtime.KubeConfigCloudController,
		"authorization-kubeconfig":     runtime.KubeConfigCloudController,
		"authentication-kubeconfig":    runtime.KubeConfigCloudController,
		"node-status-update-frequency": "1m0s",
		"bind-address":                 cfg.Loopback(false),
		"feature-gates":                "CloudDualStackNodeIPs=true",
	}
	if cfg.NoLeaderElect {
		argsMap["leader-elect"] = "false"
	}
	if cfg.DisableCCM {
		argsMap["controllers"] = argsMap["controllers"] + ",-cloud-node,-cloud-node-lifecycle"
		argsMap["secure-port"] = "0"
	}
	if cfg.DisableServiceLB {
		argsMap["controllers"] = argsMap["controllers"] + ",-service"
	}
	if cfg.VLevel != 0 {
		argsMap["v"] = strconv.Itoa(cfg.VLevel)
	}
	if cfg.VModule != "" {
		argsMap["vmodule"] = cfg.VModule
	}

	args := config.GetArgs(argsMap, cfg.ExtraCloudControllerArgs)

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
			case err := <-promise(func() error { return checkForCloudControllerPrivileges(ctx, cfg.Runtime, 5*time.Second) }):
				if err != nil {
					logrus.Infof("Waiting for cloud-controller-manager privileges to become available: %v", err)
					continue
				}
				return
			}
		}
	}()

	return executor.CloudControllerManager(ctx, ccmRBACReady, args)
}

// checkForCloudControllerPrivileges makes a SubjectAccessReview request to the apiserver
// to validate that the embedded cloud controller manager has the required privileges,
// and does not return until the requested access is granted.
// If the CCM RBAC changes, the ResourceAttributes checked for by this function should
// be modified to check for the most recently added privilege.
func checkForCloudControllerPrivileges(ctx context.Context, runtime *config.ControlRuntime, timeout time.Duration) error {
	return util.WaitForRBACReady(ctx, runtime.KubeConfigSupervisor, timeout, authorizationv1.ResourceAttributes{
		Namespace: metav1.NamespaceSystem,
		Verb:      "watch",
		Resource:  "endpointslices",
		Group:     "discovery.k8s.io",
	}, version.Program+"-cloud-controller-manager")
}

func waitForAPIServerHandlers(ctx context.Context, runtime *config.ControlRuntime) {
	auth, handler, err := executor.APIServerHandlers(ctx)
	if err != nil {
		logrus.Fatalf("Failed to get request handlers from apiserver: %v", err)
	}
	runtime.Authenticator = authenticator.Combine(runtime.Authenticator, auth)
	runtime.APIServer = handler
}

func waitForAPIServerInBackground(ctx context.Context, runtime *config.ControlRuntime) error {
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
			case err := <-promise(func() error { return util.WaitForAPIServerReady(ctx, runtime.KubeConfigSupervisor, 30*time.Second) }):
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
