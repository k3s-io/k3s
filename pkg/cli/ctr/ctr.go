package ctr

import (
	"context"

	"github.com/k3s-io/k3s/pkg/ctr"
	"github.com/urfave/cli/v3"
)

func Run(ctx context.Context, cmd *cli.Command) error {
	ctr.Main()
	return nil
}
