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
	"github.com/rancher/k3s/pkg/configfilearg"
	"github.com/rancher/k3s/pkg/data"
	"github.com/rancher/k3s/pkg/datadir"
	"github.com/rancher/k3s/pkg/dataverify"
	"github.com/rancher/k3s/pkg/flock"
	"github.com/rancher/k3s/pkg/untar"
	"github.com/rancher/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	dataDir := findDataDir()
	if runCLIs(dataDir) {
		return
	}

	app := cmds.NewApp()
	app.Commands = []cli.Command{
		cmds.NewServerCommand(wrap(version.Program+"-server", dataDir, os.Args)),
		cmds.NewAgentCommand(wrap(version.Program+"-agent", dataDir, os.Args)),
		cmds.NewKubectlCommand(externalCLIAction("kubectl", dataDir)),
		cmds.NewCRICTL(externalCLIAction("crictl", dataDir)),
		cmds.NewCtrCommand(externalCLIAction("ctr", dataDir)),
		cmds.NewCheckConfigCommand(externalCLIAction("check-config", dataDir)),
		cmds.NewEtcdSnapshotCommand(wrap(version.Program+"-"+cmds.EtcdSnapshotCommand, dataDir, os.Args)),
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func findDataDir() string {
	for i, arg := range os.Args {
		for _, flagName := range []string{"--data-dir", "-d"} {
			if flagName == arg {
				if len(os.Args) > i+1 {
					return os.Args[i+1]
				}
			} else if strings.HasPrefix(arg, flagName+"=") {
				return arg[len(flagName)+1:]
			}
		}
	}
	dataDir := configfilearg.MustFindString(os.Args, "data-dir")
	if dataDir == "" {
		if os.Getuid() == 0 {
			dataDir = datadir.DefaultDataDir
		} else {
			dataDir = datadir.DefaultHomeDataDir
		}
		logrus.Debug("Using default data dir in self-extracting wrapper")
	}
	return dataDir
}

func runCLIs(dataDir string) bool {
	if os.Getenv("CRI_CONFIG_FILE") == "" {
		os.Setenv("CRI_CONFIG_FILE", dataDir+"/agent/etc/crictl.yaml")
	}
	for _, cmd := range []string{"kubectl", "ctr", "crictl"} {
		if filepath.Base(os.Args[0]) == cmd {
			if err := externalCLI(cmd, dataDir, os.Args[1:]); err != nil {
				logrus.Fatal(err)
			}
			return true
		}
	}
	return false
}

func externalCLIAction(cmd, dataDir string) func(cli *cli.Context) error {
	return func(cli *cli.Context) error {
		return externalCLI(cmd, dataDir, cli.Args())
	}
}

func externalCLI(cli, dataDir string, args []string) error {
	dataDir, err := datadir.Resolve(dataDir)
	if err != nil {
		return err
	}
	return stageAndRun(dataDir, cli, append([]string{cli}, args...))
}

func wrap(cmd, dataDir string, args []string) func(ctx *cli.Context) error {
	return func(ctx *cli.Context) error {
		return stageAndRunCLI(ctx, cmd, dataDir, args)
	}
}

func stageAndRunCLI(cli *cli.Context, cmd string, dataDir string, args []string) error {
	dataDir, err := datadir.Resolve(dataDir)
	if err != nil {
		return err
	}

	return stageAndRun(dataDir, cmd, args)
}

func stageAndRun(dataDir, cmd string, args []string) error {
	dir, err := extract(dataDir)
	if err != nil {
		return errors.Wrap(err, "extracting data")
	}
	logrus.Debugf("Asset dir %s", dir)

	if err := os.Setenv("PATH", filepath.Join(dir, "bin")+":"+os.Getenv("PATH")+":"+filepath.Join(dir, "bin/aux")); err != nil {
		return err
	}
	if err := os.Setenv(version.ProgramUpper+"_DATA_DIR", dir); err != nil {
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
		return dir, nil
	}

	asset, dir := getAssetAndDir(dataDir)
	// check if target directory already exists
	if _, err := os.Stat(dir); err == nil {
		return dir, nil
	}

	// acquire a data directory lock
	os.MkdirAll(filepath.Join(dataDir, "data"), 0755)
	lockFile := filepath.Join(dataDir, "data", ".lock")
	logrus.Infof("Acquiring lock file %s", lockFile)
	lock, err := flock.Acquire(lockFile)
	if err != nil {
		return "", err
	}
	defer flock.Release(lock)

	// check again if target directory exists
	if _, err := os.Stat(dir); err == nil {
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
	if err := dataverify.Verify(filepath.Join(tempDest, "bin")); err != nil {
		return "", err
	}

	currentSymLink := filepath.Join(dataDir, "data", "current")
	previousSymLink := filepath.Join(dataDir, "data", "previous")
	if _, err := os.Lstat(currentSymLink); err == nil {
		if err := os.Rename(currentSymLink, previousSymLink); err != nil {
			return "", err
		}
	}
	if err := os.Symlink(dir, currentSymLink); err != nil {
		return "", err
	}
	return dir, os.Rename(tempDest, dir)
}
