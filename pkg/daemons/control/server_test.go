package control

import (
	"context"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/cluster"
	"github.com/k3s-io/k3s/pkg/cluster/managed"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/control/deps"
	"github.com/k3s-io/k3s/pkg/daemons/executor"
	"github.com/k3s-io/k3s/pkg/etcd"
	testutil "github.com/k3s-io/k3s/tests"
	"github.com/k3s-io/k3s/tests/mock"
	pkgerrors "github.com/pkg/errors"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/request/anonymous"
)

func Test_UnitServer(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(context.Context, *testing.T) (*config.Control, error)
		wantErr bool
	}{
		{
			name: "ControlPlane+ETCD",
			setup: func(ctx context.Context, t *testing.T) (*config.Control, error) {
				control, err := mockControl(ctx, t, true)
				if err != nil {
					return nil, err
				}

				control.DisableCCM = true

				executor := mock.NewExecutorWithEmbeddedETCD(t)

				// leader-elect should NOT be disabled when using etcd
				matchLeaderElectArgs := mock.GM(Not(ContainElement(ContainSubstring("--leader-elect=false"))))

				executor.EXPECT().APIServerHandlers(gomock.Any()).MinTimes(1).DoAndReturn(mockHandlers)
				executor.EXPECT().APIServer(gomock.Any(), gomock.Any()).MinTimes(1).Return(nil)
				executor.EXPECT().Scheduler(gomock.Any(), gomock.Any(), matchLeaderElectArgs).MinTimes(1).Return(nil)
				executor.EXPECT().ControllerManager(gomock.Any(), matchLeaderElectArgs).MinTimes(1).Return(nil)
				executor.EXPECT().CloudControllerManager(gomock.Any(), gomock.Any(), matchLeaderElectArgs).MinTimes(1).Return(nil)

				return control, nil
			},
		},
		{
			name: "ETCD Only",
			setup: func(ctx context.Context, t *testing.T) (*config.Control, error) {
				control, err := mockControl(ctx, t, true)
				if err != nil {
					return nil, err
				}

				control.DisableAPIServer = true
				control.DisableCCM = true
				control.DisableControllerManager = true
				control.DisableScheduler = true
				control.DisableServiceLB = true

				mock.NewExecutorWithEmbeddedETCD(t)

				// don't need to test anything else, the mock will fail if we get any unexpected calls to executor methods

				return control, nil
			},
		},
		{
			name: "ControlPlane+Kine",
			setup: func(ctx context.Context, t *testing.T) (*config.Control, error) {
				control, err := mockControl(ctx, t, false)
				if err != nil {
					return nil, err
				}

				control.DisableCCM = true

				executor := mock.NewExecutorWithEmbeddedETCD(t)

				// leader-elect should be disabled when using kine+sqlite
				matchLeaderElectArgs := mock.GM(ContainElement(ContainSubstring("--leader-elect=false")))

				executor.EXPECT().APIServerHandlers(gomock.Any()).MinTimes(1).DoAndReturn(mockHandlers)
				executor.EXPECT().APIServer(gomock.Any(), gomock.Any()).MinTimes(1).Return(nil)
				executor.EXPECT().Scheduler(gomock.Any(), gomock.Any(), matchLeaderElectArgs).MinTimes(1).Return(nil)
				executor.EXPECT().ControllerManager(gomock.Any(), matchLeaderElectArgs).MinTimes(1).Return(nil)
				executor.EXPECT().CloudControllerManager(gomock.Any(), gomock.Any(), matchLeaderElectArgs).MinTimes(1).Return(nil)

				return control, nil
			},
		},
		{
			name: "ControlPlane+Kine with auth config",
			setup: func(ctx context.Context, t *testing.T) (*config.Control, error) {
				control, err := mockControl(ctx, t, false)
				if err != nil {
					return nil, err
				}

				control.DisableCCM = true

				executor := mock.NewExecutorWithEmbeddedETCD(t)

				// authorization-mode and anonymous-auth should not be set when user sets --authorization-config and --authentication-config
				control.ExtraAPIArgs = []string{"authorization-config=/dev/null", "authentication-config=/dev/null"}
				matchAuthArgs := mock.GM(And(
					ContainElement(ContainSubstring("--authorization-config")),
					ContainElement(ContainSubstring("--authentication-config")),
					Not(ContainElement(ContainSubstring("--authorization-mode"))),
					Not(ContainElement(ContainSubstring("--anonymous-auth"))),
				))

				// leader-elect should be disabled when using kine+sqlite
				matchLeaderElectArgs := mock.GM(ContainElement(ContainSubstring("--leader-elect=false")))

				executor.EXPECT().APIServerHandlers(gomock.Any()).MinTimes(1).DoAndReturn(mockHandlers)
				executor.EXPECT().APIServer(gomock.Any(), matchAuthArgs).MinTimes(1).Return(nil)
				executor.EXPECT().Scheduler(gomock.Any(), gomock.Any(), matchLeaderElectArgs).MinTimes(1).Return(nil)
				executor.EXPECT().ControllerManager(gomock.Any(), matchLeaderElectArgs).MinTimes(1).Return(nil)
				executor.EXPECT().CloudControllerManager(gomock.Any(), gomock.Any(), matchLeaderElectArgs).MinTimes(1).Return(nil)

				return control, nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// reset managed etcd driver state for each test
			managed.Clear()
			managed.RegisterDriver(etcd.NewETCD())

			ctx, cancel := context.WithCancel(context.Background())
			wg := &sync.WaitGroup{}
			defer func() {
				// give time for the cluster datastore to finish saving after the cluster is started;
				// it'll panic if the context is cancelled while this is in progress
				time.Sleep(time.Second)
				cancel()
				// give time for etcd to shut down between tests, following context cancellation
				wg.Wait()
			}()

			// generate control config
			cfg, err := tt.setup(ctx, t)
			if err != nil {
				t.Errorf("Setup for Server() failed = %v", err)
				return
			}

			// bootstrap the executor with dummy node config
			nodeConfig := &config.Node{
				AgentConfig: config.Agent{
					KubeConfigK3sController: cfg.Runtime.KubeConfigController,
				},
			}
			if err := executor.Bootstrap(ctx, nodeConfig, cmds.AgentConfig); err != nil {
				t.Errorf("Executor Bootstrap() failed = %v", err)
				return
			}

			// test Server now that everything's set up
			if err := Server(ctx, wg, cfg); (err != nil) != tt.wantErr {
				t.Errorf("Server() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func mockControl(ctx context.Context, t *testing.T, clusterInit bool) (*config.Control, error) {
	control := &config.Control{
		AgentToken:           "agent-token",
		ClusterInit:          clusterInit,
		DataDir:              t.TempDir(),
		ServerNodeName:       "k3s-server-1",
		ServiceNodePortRange: &utilnet.PortRange{Base: 30000, Size: 2048},
		Token:                "token",
		Datastore:            etcd.DefaultEndpointConfig(),
	}

	if err := os.Chdir(control.DataDir); err != nil {
		return nil, err
	}

	os.Setenv("NODE_NAME", control.ServerNodeName)
	testutil.GenerateRuntime(control)

	control.Cluster = cluster.New(control)
	if err := control.Cluster.Bootstrap(ctx, control.ClusterReset); err != nil {
		return nil, pkgerrors.WithMessage(err, "failed to bootstrap cluster data")
	}

	if err := deps.GenServerDeps(control); err != nil {
		return nil, pkgerrors.WithMessage(err, "failed to generate server dependencies")
	}

	return control, nil
}

func mockHandlers(ctx context.Context) (authenticator.Request, http.Handler, error) {
	return &anonymous.Authenticator{}, http.NotFoundHandler(), nil
}
