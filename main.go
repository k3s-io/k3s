//go:generate go run pkg/codegen/main.go
//go:generate go fmt pkg/deploy/zz_generated_bindata.go
//go:generate go fmt pkg/static/zz_generated_bindata.go

package main

import (
	"context"
	"errors"
	"os"

	"github.com/k3s-io/k3s/pkg/cli/agent"
	"github.com/k3s-io/k3s/pkg/cli/cert"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/cli/completion"
	"github.com/k3s-io/k3s/pkg/cli/crictl"
	"github.com/k3s-io/k3s/pkg/cli/etcdsnapshot"
	"github.com/k3s-io/k3s/pkg/cli/kubectl"
	"github.com/k3s-io/k3s/pkg/cli/secretsencrypt"
	"github.com/k3s-io/k3s/pkg/cli/server"
	"github.com/k3s-io/k3s/pkg/configfilearg"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	app := cmds.NewApp()
	app.Commands = []cli.Command{
		cmds.NewServerCommand(server.Run),
		cmds.NewAgentCommand(agent.Run),
		cmds.NewKubectlCommand(kubectl.Run),
		cmds.NewCRICTL(crictl.Run),
		cmds.NewEtcdSnapshotCommands(
			etcdsnapshot.Delete,
			etcdsnapshot.List,
			etcdsnapshot.Prune,
			etcdsnapshot.Save,
		),
		cmds.NewSecretsEncryptCommands(
			secretsencrypt.Status,
			secretsencrypt.Enable,
			secretsencrypt.Disable,
			secretsencrypt.Prepare,
			secretsencrypt.Rotate,
			secretsencrypt.Reencrypt,
			secretsencrypt.RotateKeys,
		),
		cmds.NewCertCommands(
			cert.Check,
			cert.Rotate,
			cert.RotateCA,
		),
		cmds.NewCompletionCommand(completion.Run),
	}

	if err := app.Run(configfilearg.MustParse(os.Args)); err != nil && !errors.Is(err, context.Canceled) {
		logrus.Fatal(err)
	}
}
