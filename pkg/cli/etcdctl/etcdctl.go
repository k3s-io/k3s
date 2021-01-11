package etcdctl

import (
	"github.com/rancher/k3s/pkg/etcdctl"
	"github.com/urfave/cli"
)

func Run(ctx *cli.Context) error {
	etcdctl.Main()
	return nil
}
