//go:build !no_embedded_executor
// +build !no_embedded_executor

package executor

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"github.com/k3s-io/k3s/pkg/agent/containerd"
	"github.com/k3s-io/k3s/pkg/agent/cri"
	"github.com/k3s-io/k3s/pkg/agent/cridockerd"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/signals"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	pkgerrors "github.com/pkg/errors"
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

var once sync.Once

func init() {
	executor = &Embedded{}
}

func (e *Embedded) Bootstrap(ctx context.Context, nodeConfig *daemonconfig.Node, cfg cmds.Agent) error {
	e.apiServerReady = util.APIServerReadyChan(ctx, nodeConfig.AgentConfig.KubeConfigK3sController, util.DefaultAPIServerReadyTimeout)
	e.etcdReady = make(chan struct{})
	e.criReady = make(chan struct{})
	e.nodeConfig = nodeConfig

	go once.Do(func() {
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
	})

	return nil
}

func (e *Embedded) Kubelet(ctx context.Context, args []string) error {
	command := kubelet.NewKubeletCommand(context.Background())
	command.SetArgs(args)

	go func() {
		<-e.APIServerReadyChan()
		defer func() {
			if err := recover(); err != nil {
				logrus.WithField("stack", string(debug.Stack())).Fatalf("kubelet panic: %v", err)
			}
		}()
		err := command.ExecuteContext(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			signals.RequestShutdown(pkgerrors.WithMessage(err, "kubelet exited"))
		}
		signals.RequestShutdown(nil)
	}()

	return nil
}

func (e *Embedded) KubeProxy(ctx context.Context, args []string) error {
	command := proxy.NewProxyCommand()
	command.SetArgs(util.GetArgs(platformKubeProxyArgs(e.nodeConfig), args))

	go func() {
		<-e.APIServerReadyChan()
		defer func() {
			if err := recover(); err != nil {
				logrus.WithField("stack", string(debug.Stack())).Fatalf("kube-proxy panic: %v", err)
			}
		}()
		err := command.ExecuteContext(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			signals.RequestShutdown(pkgerrors.WithMessage(err, "kube-proxy exited"))
		}
		signals.RequestShutdown(nil)
	}()

	return nil
}

func (*Embedded) APIServerHandlers(ctx context.Context) (authenticator.Request, http.Handler, error) {
	startupConfig := <-apiapp.StartupConfig
	return startupConfig.Authenticator, startupConfig.Handler, nil
}

func (e *Embedded) APIServer(ctx context.Context, args []string) error {
	command := apiapp.NewAPIServerCommand(ctx.Done())
	command.SetArgs(args)

	go func() {
		<-e.ETCDReadyChan()
		defer func() {
			if err := recover(); err != nil {
				logrus.WithField("stack", string(debug.Stack())).Fatalf("apiserver panic: %v", err)
			}
		}()
		err := command.ExecuteContext(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			signals.RequestShutdown(pkgerrors.WithMessage(err, "apiserver exited"))
		}
		signals.RequestShutdown(nil)
	}()

	return nil
}

func (e *Embedded) Scheduler(ctx context.Context, nodeReady <-chan struct{}, args []string) error {
	command := sapp.NewSchedulerCommand()
	command.SetArgs(args)

	go func() {
		<-e.APIServerReadyChan()
		<-nodeReady
		defer func() {
			if err := recover(); err != nil {
				logrus.WithField("stack", string(debug.Stack())).Fatalf("scheduler panic: %v", err)
			}
		}()
		err := command.ExecuteContext(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			signals.RequestShutdown(pkgerrors.WithMessage(err, "scheduler exited"))
		}
		signals.RequestShutdown(nil)
	}()

	return nil
}

func (e *Embedded) ControllerManager(ctx context.Context, args []string) error {
	command := cmapp.NewControllerManagerCommand()
	command.SetArgs(args)

	go func() {
		<-e.APIServerReadyChan()
		defer func() {
			if err := recover(); err != nil {
				logrus.WithField("stack", string(debug.Stack())).Fatalf("controller-manager panic: %v", err)
			}
		}()
		err := command.ExecuteContext(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			signals.RequestShutdown(pkgerrors.WithMessage(err, "controller-manager exited"))
		}
		signals.RequestShutdown(nil)
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
			signals.RequestShutdown(pkgerrors.WithMessage(err, "cloud-controller-manager exited"))
		}
		signals.RequestShutdown(nil)
	}()

	return nil
}

func (e *Embedded) CurrentETCDOptions() (InitialOptions, error) {
	return InitialOptions{}, nil
}

func (e *Embedded) Containerd(ctx context.Context, cfg *daemonconfig.Node) error {
	return CloseIfNilErr(containerd.Run(ctx, cfg), e.criReady)
}

func (e *Embedded) Docker(ctx context.Context, cfg *daemonconfig.Node) error {
	return CloseIfNilErr(cridockerd.Run(ctx, cfg), e.criReady)
}

func (e *Embedded) CRI(ctx context.Context, cfg *daemonconfig.Node) error {
	// agentless sets cri socket path to /dev/null to indicate no CRI is needed
	if cfg.ContainerRuntimeEndpoint != "/dev/null" {
		return CloseIfNilErr(cri.WaitForService(ctx, cfg.ContainerRuntimeEndpoint, "CRI"), e.criReady)
	}
	return CloseIfNilErr(nil, e.criReady)
}

func (e *Embedded) APIServerReadyChan() <-chan struct{} {
	if e.apiServerReady == nil {
		panic("executor not bootstrapped")
	}
	return e.apiServerReady
}

func (e *Embedded) ETCDReadyChan() <-chan struct{} {
	if e.etcdReady == nil {
		panic("executor not bootstrapped")
	}
	return e.etcdReady
}

func (e *Embedded) CRIReadyChan() <-chan struct{} {
	if e.criReady == nil {
		panic("executor not bootstrapped")
	}
	return e.criReady
}

func (e Embedded) IsSelfHosted() bool {
	return false
}
