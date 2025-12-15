//go:build !no_embedded_executor

package embed

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"github.com/k3s-io/k3s/pkg/agent/containerd"
	"github.com/k3s-io/k3s/pkg/agent/cri"
	"github.com/k3s-io/k3s/pkg/agent/cridockerd"
	"github.com/k3s-io/k3s/pkg/agent/flannel"
	"github.com/k3s-io/k3s/pkg/agent/netpol"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/executor"
	"github.com/k3s-io/k3s/pkg/executor/embed/etcd"
	"github.com/k3s-io/k3s/pkg/signals"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/k3s-io/k3s/pkg/vpn"
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
	utilsnet "k8s.io/utils/net"

	// registering k3s cloud provider
	_ "github.com/k3s-io/k3s/pkg/cloudprovider"
)

var once sync.Once

func init() {
	executor.Set(&Embedded{})
}

// explicit type check
var _ executor.Executor = &Embedded{}

type Embedded struct {
	apiServerReady <-chan struct{}
	etcdReady      chan struct{}
	criReady       chan struct{}
	nodeConfig     *daemonconfig.Node
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

	if nodeConfig.Flannel.Backend != flannel.BackendNone {
		var err error

		if len(cfg.FlannelIface) > 0 {
			nodeConfig.Flannel.Iface, err = net.InterfaceByName(cfg.FlannelIface)
			if err != nil {
				return pkgerrors.WithMessagef(err, "unable to find interface %s", cfg.FlannelIface)
			}
		}

		// If there is a VPN, we must overwrite NodeIP and flannel interface
		var vpnInfo vpn.VPNInfo
		if cfg.VPNAuth != "" {
			vpnInfo, err = vpn.GetVPNInfo(cfg.VPNAuth)
			if err != nil {
				return err
			}

			// Pass ipv4, ipv6 or both depending on nodeIPs mode
			nodeIPs := nodeConfig.AgentConfig.NodeIPs
			var vpnIPs []net.IP
			if utilsnet.IsIPv4(nodeIPs[0]) && vpnInfo.IPv4Address != nil {
				vpnIPs = append(vpnIPs, vpnInfo.IPv4Address)
				if vpnInfo.IPv6Address != nil {
					vpnIPs = append(vpnIPs, vpnInfo.IPv6Address)
				}
			} else if utilsnet.IsIPv6(nodeIPs[0]) && vpnInfo.IPv6Address != nil {
				vpnIPs = append(vpnIPs, vpnInfo.IPv6Address)
				if vpnInfo.IPv4Address != nil {
					vpnIPs = append(vpnIPs, vpnInfo.IPv4Address)
				}
			} else {
				return fmt.Errorf("address family mismatch when assigning VPN addresses to node: node=%v, VPN ipv4=%v ipv6=%v", nodeIPs, vpnInfo.IPv4Address, vpnInfo.IPv6Address)
			}

			// Overwrite nodeip and flannel interface and throw a warning if user explicitly set those parameters
			if len(vpnIPs) != 0 {
				logrus.Infof("Node-ip changed to %v due to VPN", vpnIPs)
				if len(cfg.NodeIP.Value()) != 0 {
					logrus.Warn("VPN provider overrides configured node-ip parameter")
				}
				if len(cfg.NodeExternalIP.Value()) != 0 {
					logrus.Warn("VPN provider overrides node-external-ip parameter")
				}
				nodeIPs = vpnIPs
				nodeConfig.Flannel.Iface, err = net.InterfaceByName(vpnInfo.VPNInterface)
				if err != nil {
					return pkgerrors.WithMessagef(err, "unable to find vpn interface: %s", vpnInfo.VPNInterface)
				}
			}
		}

		// set paths for embedded flannel if enabled
		hostLocal, err := exec.LookPath("host-local")
		if err != nil {
			return pkgerrors.WithMessagef(err, "failed to find host-local")
		}

		if cfg.FlannelConf == "" {
			nodeConfig.Flannel.ConfFile = filepath.Join(cfg.DataDir, "agent", "etc", "flannel", "net-conf.json")
		} else {
			nodeConfig.Flannel.ConfFile = cfg.FlannelConf
			nodeConfig.Flannel.ConfOverride = true
		}
		nodeConfig.AgentConfig.CNIBinDir = filepath.Dir(hostLocal)
		nodeConfig.AgentConfig.CNIConfDir = filepath.Join(cfg.DataDir, "agent", "etc", "cni", "net.d")

		// It does not make sense to use VPN without its flannel backend
		if cfg.VPNAuth != "" {
			nodeConfig.Flannel.Backend = vpnInfo.ProviderName
		}
	}

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
	command := sapp.NewSchedulerCommand(ctx.Done())
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

func (e *Embedded) CurrentETCDOptions() (executor.InitialOptions, error) {
	return executor.InitialOptions{}, nil
}

func (e *Embedded) ETCD(ctx context.Context, wg *sync.WaitGroup, args *executor.ETCDConfig, extraArgs []string, test executor.TestFunc) error {
	// Start a goroutine to call the provided test function until it returns true.
	// The test function is reponsible for ensuring that the etcd server is up
	// and ready to accept client requests.
	if e.etcdReady != nil {
		go func() {
			for {
				if err := test(ctx, true); err != nil {
					logrus.Infof("Failed to test etcd connection: %v", err)
				} else {
					logrus.Info("Connection to etcd is ready")
					close(e.etcdReady)
					return
				}

				select {
				case <-time.After(5 * time.Second):
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	return etcd.StartETCD(ctx, wg, args, extraArgs)
}

func (e *Embedded) Containerd(ctx context.Context, cfg *daemonconfig.Node) error {
	return executor.CloseIfNilErr(containerd.Run(ctx, cfg), e.criReady)
}

func (e *Embedded) Docker(ctx context.Context, cfg *daemonconfig.Node) error {
	return executor.CloseIfNilErr(cridockerd.Run(ctx, cfg), e.criReady)
}

func (e *Embedded) CRI(ctx context.Context, cfg *daemonconfig.Node) error {
	// agentless sets cri socket path to /dev/null to indicate no CRI is needed
	if cfg.ContainerRuntimeEndpoint != "/dev/null" {
		return executor.CloseIfNilErr(cri.WaitForService(ctx, cfg.ContainerRuntimeEndpoint, "CRI"), e.criReady)
	}
	return executor.CloseIfNilErr(nil, e.criReady)
}

func (e *Embedded) CNI(ctx context.Context, wg *sync.WaitGroup, cfg *daemonconfig.Node) error {
	if cfg.Flannel.Backend != flannel.BackendNone {
		if (cfg.Flannel.ExternalIP) && (len(cfg.AgentConfig.NodeExternalIPs) == 0) {
			logrus.Warnf("Server has flannel-external-ip flag set but this node does not set node-external-ip. Flannel will use internal address when connecting to this node.")
		} else if (cfg.Flannel.ExternalIP) && (cfg.Flannel.Backend != flannel.BackendWireguardNative) {
			logrus.Warnf("Flannel is using external addresses with an insecure backend: %v. Please consider using an encrypting flannel backend.", cfg.Flannel.Backend)
		}
		if err := flannel.Prepare(ctx, cfg); err != nil {
			return err
		}

		if err := flannel.Run(ctx, wg, cfg); err != nil {
			return err
		}
	}

	if !cfg.AgentConfig.DisableNPC {
		if err := netpol.Run(ctx, wg, cfg); err != nil {
			return err
		}
	}

	return nil
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
