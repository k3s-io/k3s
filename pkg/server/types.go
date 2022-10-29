package server

import (
	"context"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/daemons/config"
)

type Config struct {
	DisableAgent      bool
	ControlConfig     config.Control
	SupervisorPort    int
	StartupHooks      []cmds.StartupHook
	LeaderControllers CustomControllers
	Controllers       CustomControllers
}

type CustomControllers []func(ctx context.Context, sc *Context) error
