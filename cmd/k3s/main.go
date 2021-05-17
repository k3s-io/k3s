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
	"github.com/rancher/wrangler/pkg/resolvehome"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var criDefaultConfigPath = "/etc/crictl.yaml"

// main entrypoint for the k3s multicall binary
func main() {
	dataDir := findDataDir()

	// Handle direct invocation via symlink alias (multicall binary behavior)
	if runCLIs(dataDir) {
		return
	}

	etcdsnapshotCommand := internalCLIAction(version.Program+"-"+cmds.EtcdSnapshotCommand, dataDir, os.Args)

	// Handle subcommand invocation (k3s server, k3s crictl, etc)
	app := cmds.NewApp()
	app.Commands = []cli.Command{
		cmds.NewServerCommand(internalCLIAction(version.Program+"-server", dataDir, os.Args)),
		cmds.NewAgentCommand(internalCLIAction(version.Program+"-agent", dataDir, os.Args)),
		cmds.NewKubectlCommand(externalCLIAction("kubectl", dataDir)),
		cmds.NewCRICTL(externalCLIAction("crictl", dataDir)),
		cmds.NewCtrCommand(externalCLIAction("ctr", dataDir)),
		cmds.NewCheckConfigCommand(externalCLIAction("check-config", dataDir)),
		cmds.NewEtcdSnapshotCommand(etcdsnapshotCommand,
			cmds.NewEtcdSnapshotSubcommands(
				etcdsnapshotCommand,
				etcdsnapshotCommand,
				etcdsnapshotCommand,
				etcdsnapshotCommand),
		),
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

// findDataDir reads data-dir settings from the CLI args and config file.
// If not found, the default will be used, which varies depending on whether
// k3s is being run as root or not.
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
	if d, err := datadir.Resolve(dataDir); err == nil {
		dataDir = d
	} else {
		logrus.Warnf("Failed to resolve user home directory: %s", err)
	}
	return dataDir
}

// runCLIs handles the case where the binary is being executed as a symlink alias,
// /usr/local/bin/crictl for example. If the executable name is one of the external
// binaries, it calls it directly and returns true. If it's not an external binary,
// it returns false so that standard CLI wrapping can occur.
func runCLIs(dataDir string) bool {
	progName := filepath.Base(os.Args[0])
	switch progName {
	case "crictl", "ctr", "kubectl":
		if err := externalCLI(progName, dataDir, os.Args[1:]); err != nil {
			logrus.Fatal(err)
		}
		return true
	}
	return false
}

// externalCLIAction returns a function that will call an external binary, be used as the Action of a cli.Command.
func externalCLIAction(cmd, dataDir string) func(cli *cli.Context) error {
	return func(cli *cli.Context) error {
		return externalCLI(cmd, dataDir, cli.Args())
	}
}

// externalCLI calls an external binary, fixing up argv[0] to the correct name.
// crictl needs extra help to find its config file so we do that here too.
func externalCLI(cli, dataDir string, args []string) error {
	if cli == "crictl" {
		if os.Getenv("CRI_CONFIG_FILE") == "" {
			os.Setenv("CRI_CONFIG_FILE", findCriConfig(dataDir))
		}
	}
	return stageAndRun(dataDir, cli, append([]string{cli}, args...))
}

// internalCLIAction returns a function that will call a K3s internal command, be used as the Action of a cli.Command.
func internalCLIAction(cmd, dataDir string, args []string) func(ctx *cli.Context) error {
	return func(ctx *cli.Context) error {
		return stageAndRunCLI(ctx, cmd, dataDir, args)
	}
}

// stageAndRunCLI calls an external binary.
func stageAndRunCLI(cli *cli.Context, cmd string, dataDir string, args []string) error {
	return stageAndRun(dataDir, cmd, args)
}

// stageAndRun does the actual work of setting up and calling an external binary.
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

// getAssetAndDir returns the name of the bindata asset, along with a directory path
// derived from the data-dir and bindata asset name.
func getAssetAndDir(dataDir string) (string, string) {
	asset := data.AssetNames()[0]
	dir := filepath.Join(dataDir, "data", strings.SplitN(filepath.Base(asset), ".", 2)[0])
	return asset, dir
}

// extract checks for and if necessary unpacks the bindata archive, returning the unique path
// to the extracted bindata asset.
func extract(dataDir string) (string, error) {
	// first look for global asset folder so we don't create a HOME version if not needed
	_, dir := getAssetAndDir(datadir.DefaultDataDir)
	if _, err := os.Stat(filepath.Join(dir, "bin", "containerd")); err == nil {
		return dir, nil
	}

	asset, dir := getAssetAndDir(dataDir)
	// check if target content already exists
	if _, err := os.Stat(filepath.Join(dir, "bin", "containerd")); err == nil {
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

// findCriConfig returns the path to crictl.yaml
// crictl won't search multiple locations for a config file. It will fall back to looking in
// the same directory as the crictl binary, but that's it. We need to check the various possible
// data-dir locations ourselves and then point it at the right one. We check:
// - the configured data-dir
// - the default user data-dir (assuming we can find the user's home directory)
// - the default system data-dir
// - the default path from upstream crictl
func findCriConfig(dataDir string) string {
	searchList := []string{filepath.Join(dataDir, "agent", criDefaultConfigPath)}

	if homeDataDir, err := resolvehome.Resolve(datadir.DefaultHomeDataDir); err == nil {
		searchList = append(searchList, filepath.Join(homeDataDir, "agent", criDefaultConfigPath))
	} else {
		logrus.Warnf("Failed to resolve user home directory: %s", err)
	}

	searchList = append(searchList, filepath.Join(datadir.DefaultDataDir, "agent", criDefaultConfigPath))
	searchList = append(searchList, criDefaultConfigPath)

	for _, path := range searchList {
		_, err := os.Stat(path)
		if err == nil {
			return path
		}
		if !errors.Is(err, os.ErrNotExist) {
			logrus.Warnf("Failed to %s", err)
		}
	}
	return ""
}
