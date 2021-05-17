//go:generate go run pkg/codegen/cleanup/main.go
//go:generate rm -rf pkg/generated
//go:generate go run pkg/codegen/main.go
//go:generate go fmt pkg/deploy/zz_generated_bindata.go
//go:generate go fmt pkg/static/zz_generated_bindata.go

package main

import (
	"os"

	"github.com/rancher/k3s/pkg/cli/agent"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/cli/crictl"
	"github.com/rancher/k3s/pkg/cli/etcdsnapshot"
	"github.com/rancher/k3s/pkg/cli/kubectl"
	"github.com/rancher/k3s/pkg/cli/server"
	"github.com/rancher/k3s/pkg/configfilearg"
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
		cmds.NewEtcdSnapshotCommand(etcdsnapshot.Run,
			cmds.NewEtcdSnapshotSubcommands(
				etcdsnapshot.Delete,
				etcdsnapshot.List,
				etcdsnapshot.Prune,
				etcdsnapshot.Run),
		),
	}

	if err := app.Run(configfilearg.MustParse(os.Args)); err != nil {
		logrus.Fatal(err)
	}
}
