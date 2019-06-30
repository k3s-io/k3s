package ctr

import (
	"github.com/rancher/k3s/pkg/ctr"
	"github.com/urfave/cli"
)

func Run(ctx *cli.Context) error {
	ctr.Main()
	return nil
}
