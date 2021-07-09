// +build !no_embedded_executor

package executor

import (
	"context"
	"net/http"

	"github.com/rancher/k3s/pkg/cli/cmds"
	daemonconfig "github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	ccm "k8s.io/cloud-provider"
	cloudprovider "k8s.io/cloud-provider"
	ccmapp "k8s.io/cloud-provider/app"
	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"
	ccmopt "k8s.io/cloud-provider/options"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/kubernetes/cmd/kube-apiserver/app"
	cmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	proxy "k8s.io/kubernetes/cmd/kube-proxy/app"
	sapp "k8s.io/kubernetes/cmd/kube-scheduler/app"
	kubelet "k8s.io/kubernetes/cmd/kubelet/app"

	// registering k3s cloud provider
	_ "github.com/rancher/k3s/pkg/cloudprovider"
)

func init() {
	executor = Embedded{}
}

type Embedded struct{}

func (Embedded) Bootstrap(ctx context.Context, nodeConfig *daemonconfig.Node, cfg cmds.Agent) error {
	return nil
}

func (Embedded) Kubelet(args []string) error {
	command := kubelet.NewKubeletCommand(context.Background())
	command.SetArgs(args)

	go func() {
		logrus.Fatalf("kubelet exited: %v", command.Execute())
	}()

	return nil
}

func (Embedded) KubeProxy(args []string) error {
	command := proxy.NewProxyCommand()
	command.SetArgs(args)

	go func() {
		logrus.Fatalf("kube-proxy exited: %v", command.Execute())
	}()

	return nil
}

func (Embedded) APIServer(ctx context.Context, etcdReady <-chan struct{}, args []string) (authenticator.Request, http.Handler, error) {
	<-etcdReady
	command := app.NewAPIServerCommand(ctx.Done())
	command.SetArgs(args)

	go func() {
		logrus.Fatalf("apiserver exited: %v", command.Execute())
	}()

	startupConfig := <-app.StartupConfig
	return startupConfig.Authenticator, startupConfig.Handler, nil
}

func (Embedded) Scheduler(apiReady <-chan struct{}, args []string) error {
	command := sapp.NewSchedulerCommand()
	command.SetArgs(args)

	go func() {
		<-apiReady
		logrus.Fatalf("scheduler exited: %v", command.Execute())
	}()

	return nil
}

func (Embedded) ControllerManager(apiReady <-chan struct{}, args []string) error {
	command := cmapp.NewControllerManagerCommand()
	command.SetArgs(args)

	go func() {
		<-apiReady
		logrus.Fatalf("controller-manager exited: %v", command.Execute())
	}()

	return nil
}

func (Embedded) CloudControllerManager(ccmRBACReady <-chan struct{}, args []string) error {
	ccmOptions, err := ccmopt.NewCloudControllerManagerOptions()
	if err != nil {
		logrus.Fatalf("unable to initialize command options: %v", err)
	}

	cloudInitializer := func(config *cloudcontrollerconfig.CompletedConfig) cloudprovider.Interface {
		cloud, err := ccm.InitCloudProvider(version.Program, "")
		if err != nil {
			logrus.Fatalf("Cloud provider could not be initialized: %v", err)
		}
		if cloud == nil {
			logrus.Fatalf("Cloud provider is nil")
		}

		cloud.Initialize(config.ClientBuilder, make(chan struct{}))
		if informerUserCloud, ok := cloud.(ccm.InformerUser); ok {
			informerUserCloud.SetInformers(config.SharedInformers)
		}

		return cloud
	}

	controllerInitializers := ccmapp.DefaultInitFuncConstructors
	delete(controllerInitializers, "service")
	delete(controllerInitializers, "route")

	command := ccmapp.NewCloudControllerManagerCommand(ccmOptions, cloudInitializer, controllerInitializers, cliflag.NamedFlagSets{}, wait.NeverStop)
	command.SetArgs(args)

	go func() {
		<-ccmRBACReady
		logrus.Fatalf("cloud-controller-manager exited: %v", command.Execute())
	}()

	return nil
}
