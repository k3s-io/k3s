package mock

import (
	"context"
	"errors"
	"net/http"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/executor"
	"k8s.io/apiserver/pkg/authentication/authenticator"
)

// mock executor that does not actually start anything
type Executor struct{}

func (e *Executor) Bootstrap(ctx context.Context, nodeConfig *config.Node, cfg cmds.Agent) error {
	return errors.New("not implemented")
}

func (e *Executor) Kubelet(ctx context.Context, args []string) error {
	return errors.New("not implemented")
}

func (e *Executor) KubeProxy(ctx context.Context, args []string) error {
	return errors.New("not implemented")
}

func (e *Executor) APIServerHandlers(ctx context.Context) (authenticator.Request, http.Handler, error) {
	return nil, nil, errors.New("not implemented")
}

func (e *Executor) APIServer(ctx context.Context, args []string) error {
	return errors.New("not implemented")
}

func (e *Executor) Scheduler(ctx context.Context, nodeReady <-chan struct{}, args []string) error {
	return errors.New("not implemented")
}

func (e *Executor) ControllerManager(ctx context.Context, args []string) error {
	return errors.New("not implemented")
}

func (e *Executor) CurrentETCDOptions() (executor.InitialOptions, error) {
	return executor.InitialOptions{}, nil
}

func (e *Executor) ETCD(ctx context.Context, args *executor.ETCDConfig, extraArgs []string, test executor.TestFunc) error {
	embed := &executor.Embedded{}
	return embed.ETCD(ctx, args, extraArgs, test)
}

func (e *Executor) CloudControllerManager(ctx context.Context, ccmRBACReady <-chan struct{}, args []string) error {
	return errors.New("not implemented")
}

func (e *Executor) Containerd(ctx context.Context, node *config.Node) error {
	return errors.New("not implemented")
}

func (e *Executor) Docker(ctx context.Context, node *config.Node) error {
	return errors.New("not implemented")
}

func (e *Executor) CRI(ctx context.Context, node *config.Node) error {
	return errors.New("not implemented")
}

func (e *Executor) APIServerReadyChan() <-chan struct{} {
	c := make(chan struct{})
	close(c)
	return c
}

func (e *Executor) ETCDReadyChan() <-chan struct{} {
	c := make(chan struct{})
	close(c)
	return c
}

func (e *Executor) CRIReadyChan() <-chan struct{} {
	c := make(chan struct{})
	close(c)
	return c
}
