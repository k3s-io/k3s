package tests

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/control/deps"
	"github.com/k3s-io/k3s/pkg/daemons/executor"
	"k8s.io/apiserver/pkg/authentication/authenticator"
)

// GenerateDataDir creates a temporary directory at "/tmp/k3s/<RANDOM_STRING>/".
// The latest directory created with this function is soft linked to "/tmp/k3s/latest/".
// This allows tests to replicate the "/var/lib/rancher/k3s" directory structure.
func GenerateDataDir(cnf *config.Control) error {
	if err := os.MkdirAll(cnf.DataDir, 0700); err != nil {
		return err
	}
	testDir, err := os.MkdirTemp(cnf.DataDir, "*")
	if err != nil {
		return err
	}
	// Remove old symlink and add new one
	os.Remove(filepath.Join(cnf.DataDir, "latest"))
	if err = os.Symlink(testDir, filepath.Join(cnf.DataDir, "latest")); err != nil {
		return err
	}
	cnf.DataDir = testDir
	cnf.DataDir, err = filepath.Abs(cnf.DataDir)
	if err != nil {
		return err
	}
	return nil
}

// CleanupDataDir removes the associated "/tmp/k3s/<RANDOM_STRING>"
// directory along with the 'latest' symlink that points at it.
func CleanupDataDir(cnf *config.Control) {
	os.Remove(filepath.Join(cnf.DataDir, "..", "latest"))
	os.RemoveAll(cnf.DataDir)
}

// GenerateRuntime creates a temporary data dir and configures
// config.ControlRuntime with all the appropriate certificate keys.
func GenerateRuntime(cnf *config.Control) error {
	// use mock executor that does not actually start things
	executor.Set(&mockExecutor{})

	// reuse ready channel from existing runtime if set
	cnf.Runtime = config.NewRuntime()
	if err := GenerateDataDir(cnf); err != nil {
		return err
	}

	os.MkdirAll(filepath.Join(cnf.DataDir, "etc"), 0700)
	os.MkdirAll(filepath.Join(cnf.DataDir, "tls"), 0700)
	os.MkdirAll(filepath.Join(cnf.DataDir, "cred"), 0700)

	deps.CreateRuntimeCertFiles(cnf)

	cnf.Datastore.ServerTLSConfig.CAFile = cnf.Runtime.ETCDServerCA
	cnf.Datastore.ServerTLSConfig.CertFile = cnf.Runtime.ServerETCDCert
	cnf.Datastore.ServerTLSConfig.KeyFile = cnf.Runtime.ServerETCDKey

	return deps.GenServerDeps(cnf)
}

func ClusterIPNet() *net.IPNet {
	_, clusterIPNet, _ := net.ParseCIDR("10.42.0.0/16")
	return clusterIPNet
}

func ServiceIPNet() *net.IPNet {
	_, serviceIPNet, _ := net.ParseCIDR("10.43.0.0/16")
	return serviceIPNet
}

// mock executor that does not actually start anything

type mockExecutor struct{}

func (m *mockExecutor) Bootstrap(ctx context.Context, nodeConfig *config.Node, cfg cmds.Agent) error {
	return errors.New("not implemented")
}

func (m *mockExecutor) Kubelet(ctx context.Context, args []string) error {
	return errors.New("not implemented")
}

func (m *mockExecutor) KubeProxy(ctx context.Context, args []string) error {
	return errors.New("not implemented")
}

func (m *mockExecutor) APIServerHandlers(ctx context.Context) (authenticator.Request, http.Handler, error) {
	return nil, nil, errors.New("not implemented")
}

func (m *mockExecutor) APIServer(ctx context.Context, etcdReady <-chan struct{}, args []string) error {
	return errors.New("not implemented")
}

func (m *mockExecutor) Scheduler(ctx context.Context, nodeReady <-chan struct{}, args []string) error {
	return errors.New("not implemented")
}

func (m *mockExecutor) ControllerManager(ctx context.Context, args []string) error {
	return errors.New("not implemented")
}

func (m *mockExecutor) CurrentETCDOptions() (executor.InitialOptions, error) {
	return executor.InitialOptions{}, nil
}

func (m *mockExecutor) ETCD(ctx context.Context, args executor.ETCDConfig, extraArgs []string) error {
	embed := &executor.Embedded{}
	return embed.ETCD(ctx, args, extraArgs)
}

func (m *mockExecutor) CloudControllerManager(ctx context.Context, ccmRBACReady <-chan struct{}, args []string) error {
	return errors.New("not implemented")
}

func (m *mockExecutor) Containerd(ctx context.Context, node *config.Node) error {
	return errors.New("not implemented")
}

func (m *mockExecutor) Docker(ctx context.Context, node *config.Node) error {
	return errors.New("not implemented")
}

func (m *mockExecutor) CRI(ctx context.Context, node *config.Node) error {
	return errors.New("not implemented")
}

func (m *mockExecutor) APIServerReadyChan() <-chan struct{} {
	c := make(chan struct{})
	close(c)
	return c
}

func (m *mockExecutor) CRIReadyChan() <-chan struct{} {
	c := make(chan struct{})
	close(c)
	return c
}
