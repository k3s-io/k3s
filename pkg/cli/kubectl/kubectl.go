package kubectl

import (
	"github.com/rancher/k3s/pkg/kubectl"
	"github.com/urfave/cli"
)

func Run(ctx *cli.Context) error {
	kubectl.Main()
	return nil
}
