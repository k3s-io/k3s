package kubectl

import (
	"github.com/k3s-io/k3s/pkg/kubectl"
	"github.com/urfave/cli/v2"
)

func Run(ctx *cli.Context) error {
	kubectl.Main()
	return nil
}
