package server

import (
	"context"

	"github.com/rancher/k3s/pkg/daemons/config"
)

type Config struct {
	DisableAgent     bool
	DisableServer    bool //etcd only nodes
	DisableETCD      bool //Server only node
	DisableServiceLB bool
	ControlConfig    config.Control
	Rootless         bool
	SupervisorPort   int
	StartupHooks     []func(context.Context, <-chan struct{}, string) error
}
