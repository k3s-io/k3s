package main

import (
	"context"
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
	"github.com/k3s-io/k3s/pkg/daemons/executor"
	"github.com/k3s-io/k3s/pkg/executor/embed"
	"github.com/k3s-io/k3s/pkg/util/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func initExecutor(af cli.ActionFunc) cli.ActionFunc {
	return func(app *cli.Context) error {
		ex, err := embed.New(app.Context, &cmds.AgentConfig)
		if err != nil {
			return errors.WithMessage(err, "failed to initialize executor")
		}
		executor.Set(ex)
		return af(app)
	}
}

func main() {
	app := cmds.NewApp()
	app.DisableSliceFlagSeparator = true
	app.Commands = []*cli.Command{
		cmds.NewServerCommand(initExecutor(server.Run)),
		cmds.NewAgentCommand(initExecutor(agent.Run)),
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
		cmds.NewCompletionCommand(
			completion.Bash,
			completion.Zsh,
		),
	}

	if err := app.Run(configfilearg.MustParse(os.Args)); err != nil && !errors.Is(err, context.Canceled) {
		logrus.Fatal(err)
	}
}
