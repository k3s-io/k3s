package executor

import (
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	"github.com/rancher/k3s/pkg/cli/cmds"
	daemonconfig "github.com/rancher/k3s/pkg/daemons/config"
	"k8s.io/apiserver/pkg/authentication/authenticator"
)

var (
	executor Executor
)

type Executor interface {
	Bootstrap(ctx context.Context, nodeConfig *daemonconfig.Node, cfg cmds.Agent) error
	Kubelet(args []string) error
	KubeProxy(args []string) error
	APIServer(ctx context.Context, etcdReady <-chan struct{}, args []string) (authenticator.Request, http.Handler, error)
	Scheduler(apiReady <-chan struct{}, args []string) error
	ControllerManager(apiReady <-chan struct{}, args []string) error
	CurrentETCDOptions() (InitialOptions, error)
	ETCD(args ETCDConfig) error
	CloudControllerManager(ccmRBACReady <-chan struct{}, args []string) error
}

type ETCDConfig struct {
	InitialOptions      `json:",inline"`
	Name                string      `json:"name,omitempty"`
	ListenClientURLs    string      `json:"listen-client-urls,omitempty"`
	ListenMetricsURLs   string      `json:"listen-metrics-urls,omitempty"`
	ListenPeerURLs      string      `json:"listen-peer-urls,omitempty"`
	AdvertiseClientURLs string      `json:"advertise-client-urls,omitempty"`
	DataDir             string      `json:"data-dir,omitempty"`
	SnapshotCount       int         `json:"snapshot-count,omitempty"`
	ServerTrust         ServerTrust `json:"client-transport-security"`
	PeerTrust           PeerTrust   `json:"peer-transport-security"`
	ForceNewCluster     bool        `json:"force-new-cluster,omitempty"`
	HeartbeatInterval   int         `json:"heartbeat-interval"`
	ElectionTimeout     int         `json:"election-timeout"`
	Logger              string      `json:"logger"`
	LogOutputs          []string    `json:"log-outputs"`
}

type ServerTrust struct {
	CertFile       string `json:"cert-file"`
	KeyFile        string `json:"key-file"`
	ClientCertAuth bool   `json:"client-cert-auth"`
	TrustedCAFile  string `json:"trusted-ca-file"`
}

type PeerTrust struct {
	CertFile       string `json:"cert-file"`
	KeyFile        string `json:"key-file"`
	ClientCertAuth bool   `json:"client-cert-auth"`
	TrustedCAFile  string `json:"trusted-ca-file"`
}

type InitialOptions struct {
	AdvertisePeerURL string `json:"initial-advertise-peer-urls,omitempty"`
	Cluster          string `json:"initial-cluster,omitempty"`
	State            string `json:"initial-cluster-state,omitempty"`
}

func (e ETCDConfig) ToConfigFile() (string, error) {
	confFile := filepath.Join(e.DataDir, "config")
	bytes, err := yaml.Marshal(&e)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(e.DataDir, 0700); err != nil {
		return "", err
	}
	return confFile, ioutil.WriteFile(confFile, bytes, 0600)
}

func Set(driver Executor) {
	executor = driver
}

func Bootstrap(ctx context.Context, nodeConfig *daemonconfig.Node, cfg cmds.Agent) error {
	return executor.Bootstrap(ctx, nodeConfig, cfg)
}

func Kubelet(args []string) error {
	return executor.Kubelet(args)
}

func KubeProxy(args []string) error {
	return executor.KubeProxy(args)
}

func APIServer(ctx context.Context, etcdReady <-chan struct{}, args []string) (authenticator.Request, http.Handler, error) {
	return executor.APIServer(ctx, etcdReady, args)
}

func Scheduler(apiReady <-chan struct{}, args []string) error {
	return executor.Scheduler(apiReady, args)
}

func ControllerManager(apiReady <-chan struct{}, args []string) error {
	return executor.ControllerManager(apiReady, args)
}

func CurrentETCDOptions() (InitialOptions, error) {
	return executor.CurrentETCDOptions()
}

func ETCD(args ETCDConfig) error {
	return executor.ETCD(args)
}

func CloudControllerManager(ccmRBACReady <-chan struct{}, args []string) error {
	return executor.CloudControllerManager(ccmRBACReady, args)
}
