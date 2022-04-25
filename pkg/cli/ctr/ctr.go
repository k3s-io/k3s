package ctr

import (
	"github.com/k3s-io/k3s/pkg/ctr"
	"github.com/urfave/cli"
)

func Run(ctx *cli.Context) error {
	ctr.Main()
	return nil
}
