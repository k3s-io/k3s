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
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/daemons/control"
	"github.com/rancher/k3s/pkg/datadir"
	"github.com/rancher/k3s/pkg/deploy"
	"github.com/rancher/k3s/pkg/helm"
	"github.com/rancher/k3s/pkg/rootlessports"
	"github.com/rancher/k3s/pkg/servicelb"
	"github.com/rancher/k3s/pkg/static"
	"github.com/rancher/k3s/pkg/tls"
	appsv1 "github.com/rancher/k3s/types/apis/apps/v1"
	batchv1 "github.com/rancher/k3s/types/apis/batch/v1"
	corev1 "github.com/rancher/k3s/types/apis/core/v1"
	v1 "github.com/rancher/k3s/types/apis/k3s.cattle.io/v1"
	rbacv1 "github.com/rancher/k3s/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/norman"
	"github.com/rancher/norman/pkg/clientaccess"
	"github.com/rancher/norman/pkg/dynamiclistener"
	"github.com/rancher/norman/pkg/resolvehome"
	"github.com/rancher/norman/types"
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

	if err := control.Server(ctx, &config.ControlConfig); err != nil {
		return "", errors.Wrap(err, "starting kubernetes")
	}

	certs, err := startNorman(ctx, config)
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

func startNorman(ctx context.Context, config *Config) (string, error) {
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

	normanConfig := &norman.Config{
		Name:       "k3s",
		KubeConfig: controlConfig.Runtime.KubeConfigSystem,
		Clients: []norman.ClientFactory{
			v1.Factory,
			appsv1.Factory,
			corev1.Factory,
			batchv1.Factory,
			rbacv1.Factory,
		},
		Schemas: []*types.Schemas{
			v1.Schemas,
		},
		CRDs: map[*types.APIVersion][]string{
			&v1.APIVersion: {
				v1.ListenerConfigGroupVersionKind.Kind,
				v1.AddonGroupVersionKind.Kind,
				v1.HelmChartGroupVersionKind.Kind,
			},
		},
		IgnoredKubeConfigEnv: true,
		GlobalSetup: func(ctx context.Context) (context.Context, error) {
			tlsServer, err = tls.NewServer(ctx, v1.ClientsFrom(ctx).ListenerConfig, *tlsConfig)
			return ctx, err
		},
		DisableLeaderElection: true,
		MasterControllers: []norman.ControllerRegister{
			helm.Register,
			func(ctx context.Context) error {
				return servicelb.Register(ctx, norman.GetServer(ctx).K8sClient, !config.DisableServiceLB,
					config.Rootless)
			},
			func(ctx context.Context) error {
				dataDir := filepath.Join(controlConfig.DataDir, "static")
				return static.Stage(dataDir)
			},
			func(ctx context.Context) error {
				dataDir := filepath.Join(controlConfig.DataDir, "manifests")
				templateVars := map[string]string{"%{CLUSTER_DNS}%": controlConfig.ClusterDNS.String(), "%{CLUSTER_DOMAIN}%": controlConfig.ClusterDomain}
				if err := deploy.Stage(dataDir, templateVars, controlConfig.Skips); err != nil {
					return err
				}
				if err := deploy.WatchFiles(ctx, dataDir); err != nil {
					return err
				}
				return nil
			},
			func(ctx context.Context) error {
				if !config.DisableServiceLB && config.Rootless {
					return rootlessports.Register(ctx, config.TLSConfig.HTTPSPort)
				}
				return nil
			},
		},
	}

	if _, _, err = normanConfig.Build(ctx, nil); err != nil {
		return "", err
	}

	for {
		certs, err := tlsServer.CACert()
		if err != nil {
			logrus.Infof("waiting to generate CA certs")
			time.Sleep(time.Second)
			continue
		}
		return certs, nil
	}
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

	if config.ControlConfig.KubeConfigOutput != "" {
		kubeConfig = config.ControlConfig.KubeConfigOutput
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
		os.Chmod(kubeConfig, os.FileMode(0644))
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
