package managed

import (
	"context"
	"net/http"

	"github.com/k3s-io/k3s/pkg/clientaccess"
	"github.com/k3s-io/k3s/pkg/daemons/config"
)

var (
	defaultDriver string
	drivers       []Driver
)

type Driver interface {
	IsInitialized(ctx context.Context, config *config.Control) (bool, error)
	Register(ctx context.Context, config *config.Control, handler http.Handler) (http.Handler, error)
	Reset(ctx context.Context, reboostrap func() error) error
	Start(ctx context.Context, clientAccessInfo *clientaccess.Info) error
	Test(ctx context.Context) error
	Restore(ctx context.Context) error
	EndpointName() string
	Snapshot(ctx context.Context, config *config.Control) error
	ReconcileSnapshotData(ctx context.Context) error
	GetMembersClientURLs(ctx context.Context) ([]string, error)
	RemoveSelf(ctx context.Context) error
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
