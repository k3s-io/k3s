package managed

import (
	"context"
	"net/http"
	"sync"

	"github.com/k3s-io/k3s/pkg/clientaccess"
	"github.com/k3s-io/k3s/pkg/daemons/config"
)

var (
	drivers []Driver
)

type Driver interface {
	SetControlConfig(config *config.Control) error
	IsInitialized() (bool, error)
	Register(handler http.Handler) (http.Handler, error)
	Reset(ctx context.Context, wg *sync.WaitGroup, reboostrap func() error) error
	IsReset() (bool, error)
	ResetFile() string
	Start(ctx context.Context, wg *sync.WaitGroup, clientAccessInfo *clientaccess.Info) error
	Restore(ctx context.Context) error
	EndpointName() string
	Snapshot(ctx context.Context) (*SnapshotResult, error)
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

func Default() Driver {
	return drivers[0]
}

func Clear() {
	drivers = []Driver{}
}

// SnapshotResult is returned by the Snapshot function,
// and lists the names of created and deleted snapshots.
type SnapshotResult struct {
	Created []string `json:"created,omitempty"`
	Deleted []string `json:"deleted,omitempty"`
}
