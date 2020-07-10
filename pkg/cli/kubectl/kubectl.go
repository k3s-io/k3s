package kubectl

import (
	"github.com/rancher/k3s/pkg/kubectl"
	"github.com/rancher/spur/cli"
)

func Run(ctx *cli.Context) error {
	kubectl.Main()
	return nil
}
