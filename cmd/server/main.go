package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/reexec"
	crictl2 "github.com/kubernetes-sigs/cri-tools/cmd/crictl"
	"github.com/rancher/k3s/pkg/cli/agent"
	"github.com/rancher/k3s/pkg/cli/cert"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/cli/crictl"
	"github.com/rancher/k3s/pkg/cli/ctr"
	"github.com/rancher/k3s/pkg/cli/etcdsnapshot"
	"github.com/rancher/k3s/pkg/cli/kubectl"
	"github.com/rancher/k3s/pkg/cli/secretsencrypt"
	"github.com/rancher/k3s/pkg/cli/server"
	"github.com/rancher/k3s/pkg/configfilearg"
	"github.com/rancher/k3s/pkg/containerd"
	ctr2 "github.com/rancher/k3s/pkg/ctr"
	kubectl2 "github.com/rancher/k3s/pkg/kubectl"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func init() {
	reexec.Register("containerd", containerd.Main)
	reexec.Register("kubectl", kubectl2.Main)
	reexec.Register("crictl", crictl2.Main)
	reexec.Register("ctr", ctr2.Main)
}

func main() {
	cmd := os.Args[0]
	os.Args[0] = filepath.Base(os.Args[0])
	if reexec.Init() {
		return
	}
	os.Args[0] = cmd

	app := cmds.NewApp()
	app.Commands = []cli.Command{
		cmds.NewServerCommand(server.Run),
		cmds.NewAgentCommand(agent.Run),
		cmds.NewKubectlCommand(kubectl.Run),
		cmds.NewCRICTL(crictl.Run),
		cmds.NewCtrCommand(ctr.Run),
		cmds.NewEtcdSnapshotCommand(etcdsnapshot.Run,
			cmds.NewEtcdSnapshotSubcommands(
				etcdsnapshot.Delete,
				etcdsnapshot.List,
				etcdsnapshot.Prune,
				etcdsnapshot.Run),
		),
		cmds.NewSecretsEncryptCommand(cli.ShowAppHelp,
			cmds.NewSecretsEncryptSubcommands(
				secretsencrypt.Status,
				secretsencrypt.Enable,
				secretsencrypt.Disable,
				secretsencrypt.Prepare,
				secretsencrypt.Rotate,
				secretsencrypt.Reencrypt),
		),
		cmds.NewCertCommand(
			cmds.NewCertSubcommands(
				cert.Run),
		),
	}

	if err := app.Run(configfilearg.MustParse(os.Args)); err != nil && !errors.Is(err, context.Canceled) {
		logrus.Fatal(err)
	}
}
