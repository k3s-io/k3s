package server

import (
	"context"

	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/daemons/config"
)

type Config struct {
	DisableAgent      bool
	DisableServiceLB  bool
	ControlConfig     config.Control
	Rootless          bool
	SupervisorPort    int
	StartupHooks      []cmds.StartupHook
	LeaderControllers CustomControllers
	Controllers       CustomControllers
}

type CustomControllers []func(ctx context.Context, sc *Context) error
