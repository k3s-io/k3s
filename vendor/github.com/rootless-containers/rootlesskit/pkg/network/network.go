package network

import (
	"context"

	"github.com/rootless-containers/rootlesskit/pkg/api"
	"github.com/rootless-containers/rootlesskit/pkg/common"
)

// ParentDriver is called from the parent namespace
type ParentDriver interface {
	Info(ctx context.Context) (*api.NetworkDriverInfo, error)
	// MTU returns MTU
	MTU() int
	// ConfigureNetwork sets up Slirp, updates msg, and returns destructor function.
	ConfigureNetwork(childPID int, stateDir string) (netmsg *common.NetworkMessage, cleanup func() error, err error)
}

// ChildDriver is called from the child namespace
type ChildDriver interface {
	// netmsg MAY be modified.
	// devName is like "tap" or "eth0"
	ConfigureNetworkChild(netmsg *common.NetworkMessage) (devName string, err error)
}
