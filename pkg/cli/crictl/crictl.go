package crictl

import (
	"context"
	"os"
	"runtime"

	"github.com/urfave/cli/v3"
	"sigs.k8s.io/cri-tools/cmd/crictl"
)

func Run(ctx context.Context, cmd *cli.Command) error {
	if runtime.GOOS == "windows" {
		os.Args = os.Args[1:]
	}
	crictl.Main()
	return nil
}
