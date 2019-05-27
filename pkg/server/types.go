package server

import (
	"github.com/rancher/dynamiclistener"
	"github.com/rancher/k3s/pkg/daemons/config"
)

type Config struct {
	DisableAgent     bool
	DisableServiceLB bool
	TLSConfig        dynamiclistener.UserConfig
	ControlConfig    config.Control
	Rootless         bool
}
