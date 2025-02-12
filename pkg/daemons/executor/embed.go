//go:build !no_embedded_executor
// +build !no_embedded_executor

package executor

import (
	"context"
	"flag"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/k3s-io/k3s/pkg/agent/containerd"
	"github.com/k3s-io/k3s/pkg/agent/cridockerd"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	cloudprovider "k8s.io/cloud-provider"
	ccmapp "k8s.io/cloud-provider/app"
	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"
	"k8s.io/cloud-provider/names"
	ccmopt "k8s.io/cloud-provider/options"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	apiapp "k8s.io/kubernetes/cmd/kube-apiserver/app"
	cmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	proxy "k8s.io/kubernetes/cmd/kube-proxy/app"
	sapp "k8s.io/kubernetes/cmd/kube-scheduler/app"
	kubelet "k8s.io/kubernetes/cmd/kubelet/app"

	// registering k3s cloud provider
	_ "github.com/k3s-io/k3s/pkg/cloudprovider"
)

func init() {
	executor = &Embedded{}
}

func (e *Embedded) Bootstrap(ctx context.Context, nodeConfig *daemonconfig.Node, cfg cmds.Agent) error {
	e.nodeConfig = nodeConfig

	go func() {
		// Ensure that the log verbosity remains set to the configured level by resetting it at 1-second intervals
		// for the first 2 minutes that K3s is starting up. This is necessary because each of the Kubernetes
		// components will initialize klog and reset the verbosity flag when they are starting.
		logCtx, cancel := context.WithTimeout(ctx, time.Second*120)
		defer cancel()

		klog.InitFlags(nil)
		for {
			flag.Set("v", strconv.Itoa(cmds.LogConfig.VLevel))

			select {
			case <-time.After(time.Second):
			case <-logCtx.Done():
				return
			}
		}
	}()

	return nil
}

func (e *Embedded) Kubelet(ctx context.Context, args []string) error {
	command := kubelet.NewKubeletCommand(context.Background())
	command.SetArgs(args)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				logrus.WithField("stack", string(debug.Stack())).Fatalf("kubelet panic: %v", err)
			}
		}()
		// The embedded executor doesn't need the kubelet to come up to host any components, and
		// having it come up on servers before the apiserver is available causes a lot of log spew.
		// Agents don't have access to the server's apiReady channel, so just wait directly.
		if err := util.WaitForAPIServerReady(ctx, e.nodeConfig.AgentConfig.KubeConfigKubelet, util.DefaultAPIServerReadyTimeout); err != nil {
			logrus.Fatalf("Kubelet failed to wait for apiserver ready: %v", err)
		}
		err := command.ExecuteContext(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			logrus.Errorf("kubelet exited: %v", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()

	return nil
}

func (e *Embedded) KubeProxy(ctx context.Context, args []string) error {
	command := proxy.NewProxyCommand()
	command.SetArgs(daemonconfig.GetArgs(platformKubeProxyArgs(e.nodeConfig), args))

	go func() {
		defer func() {
			if err := recover(); err != nil {
				logrus.WithField("stack", string(debug.Stack())).Fatalf("kube-proxy panic: %v", err)
			}
		}()
		err := command.ExecuteContext(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			logrus.Errorf("kube-proxy exited: %v", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()

	return nil
}

func (*Embedded) APIServerHandlers(ctx context.Context) (authenticator.Request, http.Handler, error) {
	startupConfig := <-apiapp.StartupConfig
	return startupConfig.Authenticator, startupConfig.Handler, nil
}

func (*Embedded) APIServer(ctx context.Context, etcdReady <-chan struct{}, args []string) error {
	command := apiapp.NewAPIServerCommand(ctx.Done())
	command.SetArgs(args)

	go func() {
		<-etcdReady
		defer func() {
			if err := recover(); err != nil {
				logrus.WithField("stack", string(debug.Stack())).Fatalf("apiserver panic: %v", err)
			}
		}()
		err := command.ExecuteContext(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			logrus.Errorf("apiserver exited: %v", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()

	return nil
}

func (e *Embedded) Scheduler(ctx context.Context, apiReady <-chan struct{}, args []string) error {
	command := sapp.NewSchedulerCommand(ctx.Done())
	command.SetArgs(args)

	go func() {
		<-apiReady
		defer func() {
			if err := recover(); err != nil {
				logrus.WithField("stack", string(debug.Stack())).Fatalf("scheduler panic: %v", err)
			}
		}()
		err := command.ExecuteContext(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			logrus.Errorf("scheduler exited: %v", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()

	return nil
}

func (*Embedded) ControllerManager(ctx context.Context, apiReady <-chan struct{}, args []string) error {
	command := cmapp.NewControllerManagerCommand()
	command.SetArgs(args)

	go func() {
		<-apiReady
		defer func() {
			if err := recover(); err != nil {
				logrus.WithField("stack", string(debug.Stack())).Fatalf("controller-manager panic: %v", err)
			}
		}()
		err := command.ExecuteContext(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			logrus.Errorf("controller-manager exited: %v", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()

	return nil
}

func (*Embedded) CloudControllerManager(ctx context.Context, ccmRBACReady <-chan struct{}, args []string) error {
	ccmOptions, err := ccmopt.NewCloudControllerManagerOptions()
	if err != nil {
		logrus.Fatalf("unable to initialize command options: %v", err)
	}

	cloudInitializer := func(config *cloudcontrollerconfig.CompletedConfig) cloudprovider.Interface {
		cloud, err := cloudprovider.InitCloudProvider(version.Program, config.ComponentConfig.KubeCloudShared.CloudProvider.CloudConfigFile)
		if err != nil {
			logrus.Fatalf("Cloud provider could not be initialized: %v", err)
		}
		if cloud == nil {
			logrus.Fatalf("Cloud provider is nil")
		}
		return cloud
	}

	controllerAliases := names.CCMControllerAliases()

	command := ccmapp.NewCloudControllerManagerCommand(
		ccmOptions,
		cloudInitializer,
		ccmapp.DefaultInitFuncConstructors,
		controllerAliases,
		cliflag.NamedFlagSets{},
		ctx.Done())
	command.SetArgs(args)

	go func() {
		<-ccmRBACReady
		defer func() {
			if err := recover(); err != nil {
				logrus.WithField("stack", string(debug.Stack())).Fatalf("cloud-controller-manager panic: %v", err)
			}
		}()
		err := command.ExecuteContext(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			logrus.Errorf("cloud-controller-manager exited: %v", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()

	return nil
}

func (e *Embedded) CurrentETCDOptions() (InitialOptions, error) {
	return InitialOptions{}, nil
}

func (e *Embedded) Containerd(ctx context.Context, cfg *daemonconfig.Node) error {
	return containerd.Run(ctx, cfg)
}

func (e *Embedded) Docker(ctx context.Context, cfg *daemonconfig.Node) error {
	return cridockerd.Run(ctx, cfg)
}
