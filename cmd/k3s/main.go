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
	if runCLIs() {
		return
	}

	app := cmds.NewApp()
	app.Commands = []cli.Command{
		cmds.NewServerCommand(wrap("k3s-server", os.Args)),
		cmds.NewAgentCommand(wrap("k3s-agent", os.Args)),
		cmds.NewKubectlCommand(externalCLIAction("kubectl")),
		//cmds.NewCtrCommand(externalCLIAction("ctr")),
		cmds.NewCRICTL(externalCLIAction("crictl")),
	}

	err := app.Run(os.Args)
	if err != nil {
		logrus.Fatal(err)
	}
}

func runCLIs() bool {
	for _, cmd := range []string{"kubectl", "ctr", "crictl"} {
		if filepath.Base(os.Args[0]) == cmd {
			if err := externalCLI(cmd, "", os.Args[1:]); err != nil {
				logrus.Fatal(err)
			}
			return true
		}
	}
	return false
}

func externalCLIAction(cmd string) func(cli *cli.Context) error {
	return func(cli *cli.Context) error {
		return externalCLI(cmd, cli.String("data-dir"), cli.Args())
	}
}

func externalCLI(cli, dataDir string, args []string) error {
	dataDir, err := datadir.Resolve(dataDir)
	if err != nil {
		return err
	}
	return stageAndRun(dataDir, cli, append([]string{cli}, args...))
}

func wrap(cmd string, args []string) func(ctx *cli.Context) error {
	return func(ctx *cli.Context) error {
		return stageAndRunCLI(ctx, cmd, args)
	}
}

func stageAndRunCLI(cli *cli.Context, cmd string, args []string) error {
	dataDir, err := datadir.Resolve(cli.String("data-dir"))
	if err != nil {
		return err
	}

	return stageAndRun(dataDir, cmd, args)
}

func stageAndRun(dataDir string, cmd string, args []string) error {
	dir, err := extract(dataDir)
	if err != nil {
		return errors.Wrap(err, "extracting data")
	}

	if err := os.Setenv("PATH", filepath.Join(dir, "bin")+":"+os.Getenv("PATH")+":"+filepath.Join(dir, "bin/aux")); err != nil {
		return err
	}

	cmd, err = exec.LookPath(cmd)
	if err != nil {
		return err
	}

	logrus.Debugf("Running %s %v", cmd, args)
	return syscall.Exec(cmd, args, os.Environ())
}

func getAssetAndDir(dataDir string) (string, string) {
	asset := data.AssetNames()[0]
	dir := filepath.Join(dataDir, "data", strings.SplitN(filepath.Base(asset), ".", 2)[0])
	return asset, dir
}

func extract(dataDir string) (string, error) {
	// first look for global asset folder so we don't create a HOME version if not needed
	_, dir := getAssetAndDir(datadir.DefaultDataDir)
	if _, err := os.Stat(dir); err == nil {
		logrus.Debugf("Asset dir %s", dir)
		return dir, nil
	}

	asset, dir := getAssetAndDir(dataDir)
	if _, err := os.Stat(dir); err == nil {
		logrus.Debugf("Asset dir %s", dir)
		return dir, nil
	}

	logrus.Infof("Preparing data dir %s", dir)

	content, err := data.Asset(asset)
	if err != nil {
		return "", err
	}
	buf := bytes.NewBuffer(content)

	tempDest := dir + "-tmp"
	defer os.RemoveAll(tempDest)

	os.RemoveAll(tempDest)

	if err := untar.Untar(buf, tempDest); err != nil {
		return "", err
	}

	return dir, os.Rename(tempDest, dir)
}
