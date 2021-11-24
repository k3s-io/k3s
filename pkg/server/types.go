package server

import (
	"context"

	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/daemons/config"
)

type Config struct {
	LeaderControllers CustomControllers
	Controllers       CustomControllers
	StartupHooks      []cmds.StartupHook
	ControlConfig     config.Control
	SupervisorPort    int
	Rootless          bool
	DisableAgent      bool
	DisableServiceLB  bool
}

type CustomControllers []func(ctx context.Context, sc *Context) error
