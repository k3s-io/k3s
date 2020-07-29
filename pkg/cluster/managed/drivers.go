package managed

import (
	"context"
	"net"
	"net/http"

	"github.com/rancher/k3s/pkg/clientaccess"
	"github.com/rancher/k3s/pkg/daemons/config"
)

var (
	defaultDriver string
	drivers       []Driver
)

type Driver interface {
	IsInitialized(ctx context.Context, config *config.Control) (bool, error)
	Register(ctx context.Context, config *config.Control, l net.Listener, handler http.Handler) (net.Listener, http.Handler, error)
	Reset(ctx context.Context, clientAccessInfo *clientaccess.Info) error
	Start(ctx context.Context, clientAccessInfo *clientaccess.Info) error
	Test(ctx context.Context, clientAccessInfo *clientaccess.Info) error
	EndpointName() string
}

func RegisterDriver(d Driver) {
	drivers = append(drivers, d)
}

func Registered() []Driver {
	return drivers
}

func Default() string {
	if defaultDriver == "" && len(drivers) == 1 {
		return drivers[0].EndpointName()
	}
	return defaultDriver
}
