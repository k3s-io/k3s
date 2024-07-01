package agent

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/k3s-io/k3s/pkg/agent/config"
	"github.com/k3s-io/k3s/pkg/agent/proxy"
	"github.com/k3s-io/k3s/pkg/agent/util"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/executor"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/otiai10/copy"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-base/logs"
	logsapi "k8s.io/component-base/logs/api/v1"
	logsv1 "k8s.io/component-base/logs/api/v1"
	_ "k8s.io/component-base/metrics/prometheus/restclient" // for client metric registration
	_ "k8s.io/component-base/metrics/prometheus/version"    // for version metric registration
	kubeletconfig "k8s.io/kubelet/config/v1beta1"
	"k8s.io/kubernetes/pkg/util/taints"
	utilsnet "k8s.io/utils/net"
	utilsptr "k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"
)

func Agent(ctx context.Context, nodeConfig *daemonconfig.Node, proxy proxy.Proxy) error {
	rand.Seed(time.Now().UTC().UnixNano())
	logsapi.ReapplyHandling = logsapi.ReapplyHandlingIgnoreUnchanged
	logs.InitLogs()
	defer logs.FlushLogs()

	if err := startKubelet(ctx, &nodeConfig.AgentConfig); err != nil {
		return errors.Wrap(err, "failed to start kubelet")
	}

	go func() {
		if !config.KubeProxyDisabled(ctx, nodeConfig, proxy) {
			if err := startKubeProxy(ctx, &nodeConfig.AgentConfig); err != nil {
				logrus.Fatalf("Failed to start kube-proxy: %v", err)
			}
		}
	}()

	return nil
}

func startKubeProxy(ctx context.Context, cfg *daemonconfig.Agent) error {
	argsMap := kubeProxyArgs(cfg)
	args := daemonconfig.GetArgs(argsMap, cfg.ExtraKubeProxyArgs)
	logrus.Infof("Running kube-proxy %s", daemonconfig.ArgString(args))
	return executor.KubeProxy(ctx, args)
}

func startKubelet(ctx context.Context, cfg *daemonconfig.Agent) error {
	argsMap, defaultConfig, err := kubeletArgsAndConfig(cfg)
	if err != nil {
		return errors.Wrap(err, "prepare default configuration drop-in")
	}

	extraArgs, err := extractConfigArgs(cfg.KubeletConfigDir, cfg.ExtraKubeletArgs, defaultConfig)
	if err != nil {
		return errors.Wrap(err, "prepare user configuration drop-ins")
	}

	if err := writeKubeletConfig(cfg.KubeletConfigDir, defaultConfig); err != nil {
		return errors.Wrap(err, "generate default kubelet configuration drop-in")
	}

	args := daemonconfig.GetArgs(argsMap, extraArgs)
	logrus.Infof("Running kubelet %s", daemonconfig.ArgString(args))

	return executor.Kubelet(ctx, args)
}

// ImageCredProvAvailable checks to see if the kubelet image credential provider bin dir and config
// files exist and are of the correct types. This is exported so that it may be used by downstream projects.
func ImageCredProvAvailable(cfg *daemonconfig.Agent) bool {
	if info, err := os.Stat(cfg.ImageCredProvBinDir); err != nil || !info.IsDir() {
		logrus.Debugf("Kubelet image credential provider bin directory check failed: %v", err)
		return false
	}
	if info, err := os.Stat(cfg.ImageCredProvConfig); err != nil || info.IsDir() {
		logrus.Debugf("Kubelet image credential provider config file check failed: %v", err)
		return false
	}
	return true
}

// extractConfigArgs strips out any --config or --config-dir flags from the
// provided args list, and if set, copies the content of the file or dir into
// the target drop-in directory.
func extractConfigArgs(path string, extraArgs []string, config *kubeletconfig.KubeletConfiguration) ([]string, error) {
	args := make([]string, 0, len(extraArgs))
	strippedArgs := map[string]string{}
	var skipVal bool
	for i := range extraArgs {
		if skipVal {
			skipVal = false
			continue
		}

		var val string
		key := strings.TrimPrefix(extraArgs[i], "--")
		if k, v, ok := strings.Cut(key, "="); ok {
			// key=val pair
			key = k
			val = v
		} else if len(extraArgs) > i+1 {
			// key in this arg, value in next arg
			val = extraArgs[i+1]
			skipVal = true
		}

		switch key {
		case "config", "config-dir":
			if val == "" {
				return nil, fmt.Errorf("value required for kubelet-arg --%s", key)
			}
			strippedArgs[key] = val
		default:
			args = append(args, extraArgs[i])
		}
	}

	// copy the config file into our managed config dir, unless its already in there
	if strippedArgs["config"] != "" && !strings.HasPrefix(strippedArgs["config"], path) {
		src := strippedArgs["config"]
		dest := filepath.Join(path, "10-cli-config.conf")
		if err := util.CopyFile(src, dest, false); err != nil {
			return nil, errors.Wrapf(err, "copy config %q into managed drop-in dir %q", src, dest)
		}
	}
	// copy the config-dir into our managed config dir, unless its already in there
	if strippedArgs["config-dir"] != "" && !strings.HasPrefix(strippedArgs["config-dir"], path) {
		src := strippedArgs["config-dir"]
		dest := filepath.Join(path, "20-cli-config-dir")
		if err := copy.Copy(src, dest, copy.Options{PreserveOwner: true}); err != nil {
			return nil, errors.Wrapf(err, "copy config-dir %q into managed drop-in dir %q", src, dest)
		}
	}
	return args, nil
}

// writeKubeletConfig marshals the provided KubeletConfiguration object into a
// drop-in config file in the target drop-in directory.
func writeKubeletConfig(path string, config *kubeletconfig.KubeletConfiguration) error {
	b, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(path, "00-"+version.Program+"-defaults.conf"), b, 0600)
}

func defaultKubeletConfig(cfg *daemonconfig.Agent) (*kubeletconfig.KubeletConfiguration, error) {
	bindAddress := "127.0.0.1"
	isIPv6 := utilsnet.IsIPv6(net.ParseIP([]string{cfg.NodeIP}[0]))
	if isIPv6 {
		bindAddress = "::1"
	}

	defaultConfig := &kubeletconfig.KubeletConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kubelet.config.k8s.io/v1beta1",
			Kind:       "KubeletConfiguration",
		},
		CPUManagerReconcilePeriod:        metav1.Duration{Duration: time.Second * 10},
		CgroupDriver:                     "cgroupfs",
		ClusterDomain:                    cfg.ClusterDomain,
		EvictionPressureTransitionPeriod: metav1.Duration{Duration: time.Minute * 5},
		FailSwapOn:                       utilsptr.To(false),
		FileCheckFrequency:               metav1.Duration{Duration: time.Second * 20},
		HTTPCheckFrequency:               metav1.Duration{Duration: time.Second * 20},
		HealthzBindAddress:               bindAddress,
		ImageMinimumGCAge:                metav1.Duration{Duration: time.Minute * 2},
		NodeStatusReportFrequency:        metav1.Duration{Duration: time.Minute * 5},
		NodeStatusUpdateFrequency:        metav1.Duration{Duration: time.Second * 10},
		ProtectKernelDefaults:            cfg.ProtectKernelDefaults,
		ReadOnlyPort:                     0,
		RuntimeRequestTimeout:            metav1.Duration{Duration: time.Minute * 2},
		StreamingConnectionIdleTimeout:   metav1.Duration{Duration: time.Hour * 4},
		SyncFrequency:                    metav1.Duration{Duration: time.Minute},
		VolumeStatsAggPeriod:             metav1.Duration{Duration: time.Minute},
		EvictionHard: map[string]string{
			"imagefs.available": "5%",
			"nodefs.available":  "5%",
		},
		EvictionMinimumReclaim: map[string]string{
			"imagefs.available": "10%",
			"nodefs.available":  "10%",
		},
		Authentication: kubeletconfig.KubeletAuthentication{
			Anonymous: kubeletconfig.KubeletAnonymousAuthentication{
				Enabled: utilsptr.To(false),
			},
			Webhook: kubeletconfig.KubeletWebhookAuthentication{
				Enabled:  utilsptr.To(true),
				CacheTTL: metav1.Duration{Duration: time.Minute * 2},
			},
		},
		Authorization: kubeletconfig.KubeletAuthorization{
			Mode: kubeletconfig.KubeletAuthorizationModeWebhook,
			Webhook: kubeletconfig.KubeletWebhookAuthorization{
				CacheAuthorizedTTL:   metav1.Duration{Duration: time.Minute * 5},
				CacheUnauthorizedTTL: metav1.Duration{Duration: time.Second * 30},
			},
		},
		Logging: logsv1.LoggingConfiguration{
			Format:    "text",
			Verbosity: logsv1.VerbosityLevel(cfg.VLevel),
			FlushFrequency: logsv1.TimeOrMetaDuration{
				Duration:          metav1.Duration{Duration: time.Second * 5},
				SerializeAsString: true,
			},
		},
	}

	if cfg.ListenAddress != "" {
		defaultConfig.Address = cfg.ListenAddress
	}

	if cfg.ClientCA != "" {
		defaultConfig.Authentication.X509.ClientCAFile = cfg.ClientCA
	}

	if cfg.ServingKubeletCert != "" && cfg.ServingKubeletKey != "" {
		defaultConfig.TLSCertFile = cfg.ServingKubeletCert
		defaultConfig.TLSPrivateKeyFile = cfg.ServingKubeletKey
	}

	for _, addr := range cfg.ClusterDNSs {
		defaultConfig.ClusterDNS = append(defaultConfig.ClusterDNS, addr.String())
	}

	if cfg.ResolvConf != "" {
		defaultConfig.ResolverConfig = utilsptr.To(cfg.ResolvConf)
	}

	if cfg.PodManifests != "" && defaultConfig.StaticPodPath == "" {
		defaultConfig.StaticPodPath = cfg.PodManifests
	}
	if err := os.MkdirAll(defaultConfig.StaticPodPath, 0750); err != nil {
		return nil, errors.Wrapf(err, "failed to create static pod manifest dir %s", defaultConfig.StaticPodPath)
	}

	if t, _, err := taints.ParseTaints(cfg.NodeTaints); err != nil {
		return nil, errors.Wrap(err, "failed to parse node taints")
	} else {
		defaultConfig.RegisterWithTaints = t
	}

	logsv1.VModuleConfigurationPflag(&defaultConfig.Logging.VModule).Set(cfg.VModule)

	return defaultConfig, nil
}
