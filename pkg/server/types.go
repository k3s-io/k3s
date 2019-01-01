package server

import (
	"github.com/rancher/norman/pkg/dynamiclistener"
	"github.com/rancher/rio/pkg/daemons/config"
)

type Config struct {
	DisableAgent  bool
	TLSConfig     dynamiclistener.UserConfig
	ControlConfig config.Control
}
