package netpol

import (
	"context"

	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
)

func Run(ctx context.Context, nodeConfig *daemonconfig.Node) error {
	panic("Netpol is not supported on windows ensure to pass --disable-network-policy")
}
