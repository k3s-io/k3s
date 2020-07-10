package crictl

import (
	"github.com/kubernetes-sigs/cri-tools/cmd/crictl"
	"github.com/rancher/spur/cli"
)

func Run(ctx *cli.Context) error {
	crictl.Main()
	return nil
}
