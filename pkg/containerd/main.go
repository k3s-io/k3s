//go:build ctrd
// +build ctrd

package containerd

import (
	"fmt"
	"os"

	"github.com/containerd/containerd/v2/cmd/containerd/command"
)

func Main() {
	app := command.App()
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "containerd: %s\n", err)
		os.Exit(1)
	}
}
