package crictl

import (
	"os"
	"runtime"

	"github.com/kubernetes-sigs/cri-tools/cmd/crictl"
	"github.com/urfave/cli"
)

func Run(ctx *cli.Context) error {
	if runtime.GOOS == "windows" {
		os.Args = os.Args[1:]
	}
	crictl.Main()
	return nil
}
