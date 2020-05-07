package executor

import (
	"context"
	"net/http"

	"k8s.io/apiserver/pkg/authentication/authenticator"
)

type Executor interface {
	Kubelet(args []string) error
	KubeProxy(args []string) error
	APIServer(ctx context.Context, args []string) (authenticator.Request, http.Handler, error)
	Scheduler(apiReady <-chan struct{}, args []string) error
	ControllerManager(apiReady <-chan struct{}, args []string) error
}

var (
	executor Executor
)

func Set(driver Executor) {
	executor = driver
}

func Kubelet(args []string) error {
	return executor.Kubelet(args)
}

func KubeProxy(args []string) error {
	return executor.KubeProxy(args)
}

func APIServer(ctx context.Context, args []string) (authenticator.Request, http.Handler, error) {
	return executor.APIServer(ctx, args)
}

func Scheduler(apiReady <-chan struct{}, args []string) error {
	return executor.Scheduler(apiReady, args)
}

func ControllerManager(apiReady <-chan struct{}, args []string) error {
	return executor.ControllerManager(apiReady, args)
}
