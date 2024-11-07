package crictl

import (
	"os"
	"runtime"

	"github.com/urfave/cli"
	"sigs.k8s.io/cri-tools/cmd/crictl"
)

func Run(ctx *cli.Context) error {
	if runtime.GOOS == "windows" {
		os.Args = os.Args[1:]
	}
	crictl.Main()
	return nil
}
