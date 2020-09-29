package containerd

import (
	"github.com/rancher/k3s/pkg/containerd"
	"github.com/urfave/cli"
)

func Run(ctx *cli.Context) error {
	containerd.Main()
	return nil
}
