package server

import (
	"github.com/rancher/k3s/pkg/daemons/config"
)

type Config struct {
	DisableAgent     bool
	DisableServiceLB bool
	ControlConfig    config.Control
	Rootless         bool
	SupervisorPort   int
}
