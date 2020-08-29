package crictl

import (
	"github.com/kubernetes-sigs/cri-tools/cmd/crictl"
	"github.com/urfave/cli"
)

func Run(ctx *cli.Context) error {
	crictl.Main()
	return nil
}
