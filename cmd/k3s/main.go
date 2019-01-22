package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/data"
	"github.com/rancher/k3s/pkg/datadir"
	"github.com/rancher/k3s/pkg/untar"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	app := cmds.NewApp()
	app.Commands = []cli.Command{
		cmds.NewServerCommand(wrap("k3s-server", os.Args)),
		cmds.NewAgentCommand(wrap("k3s-agent", os.Args)),
		cmds.NewKubectlCommand(kubectl),
	}

	err := app.Run(os.Args)
	if err != nil {
		logrus.Fatal(err)
	}
}

func kubectl(ctx *cli.Context) error {
	return stageAndRun(ctx, "kubectl", append([]string{"kubectl"}, ctx.Args()...))
}

func wrap(cmd string, args []string) func(ctx *cli.Context) error {
	return func(ctx *cli.Context) error {
		return stageAndRun(ctx, cmd, args)
	}
}

func stageAndRun(cli *cli.Context, cmd string, args []string) error {
	dataDir, err := datadir.Resolve(cli.String("data-dir"))
	if err != nil {
		return err
	}

	asset, dir := getAssetAndDir(dataDir)

	if err := extract(asset, dir); err != nil {
		return errors.Wrap(err, "extracting data")
	}

	if err := os.Setenv("PATH", filepath.Join(dir, "bin")+":"+os.Getenv("PATH")); err != nil {
		return err
	}

	cmd, err = exec.LookPath(cmd)
	if err != nil {
		return err
	}

	return syscall.Exec(cmd, args, os.Environ())
}

func getAssetAndDir(dataDir string) (string, string) {
	asset := data.AssetNames()[0]
	dir := filepath.Join(dataDir, "data", strings.SplitN(filepath.Base(asset), ".", 2)[0])
	return asset, dir
}

func extract(asset, dir string) error {
	logrus.Debugf("Asset dir %s", dir)

	if _, err := os.Stat(dir); err == nil {
		return nil
	}

	logrus.Infof("Preparing data dir %s", dir)

	content, err := data.Asset(asset)
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer(content)

	tempDest := dir + "-tmp"
	defer os.RemoveAll(tempDest)

	os.RemoveAll(tempDest)

	if err := untar.Untar(buf, tempDest); err != nil {
		return err
	}

	return os.Rename(tempDest, dir)
}
