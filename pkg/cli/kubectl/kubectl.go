package kubectl

import (
	"context"

	"github.com/k3s-io/k3s/pkg/kubectl"
	"github.com/urfave/cli/v3"
)

func Run(ctx context.Context, cmd *cli.Command) error {
	kubectl.Main()
	return nil
}
