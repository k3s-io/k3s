package netpol

import (
	"context"

	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/sirupsen/logrus"
)

func Run(ctx context.Context, nodeConfig *daemonconfig.Node) error {
	logrus.Warnf("Skipping network policy controller start, netpol is not supported on windows")
	return nil
}
