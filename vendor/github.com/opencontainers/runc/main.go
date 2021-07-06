package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/opencontainers/runc/libcontainer/logs"
	"github.com/opencontainers/runc/libcontainer/seccomp"
	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// version will be populated by the Makefile, read from
// VERSION file of the source code.
var version = ""

// gitCommit will be the hash that the binary was built from
// and will be populated by the Makefile
var gitCommit = ""

const (
	specConfig = "config.json"
	usage      = `Open Container Initiative runtime

runc is a command line client for running applications packaged according to
the Open Container Initiative (OCI) format and is a compliant implementation of the
Open Container Initiative specification.

runc integrates well with existing process supervisors to provide a production
container runtime environment for applications. It can be used with your
existing process monitoring tools and the container will be spawned as a
direct child of the process supervisor.

Containers are configured using bundles. A bundle for a container is a directory
that includes a specification file named "` + specConfig + `" and a root filesystem.
The root filesystem contains the contents of the container.

To start a new instance of a container:

    # runc run [ -b bundle ] <container-id>

Where "<container-id>" is your name for the instance of the container that you
are starting. The name you provide for the container instance must be unique on
your host. Providing the bundle directory using "-b" is optional. The default
value for "bundle" is the current directory.`
)

func main() {
	app := cli.NewApp()
	app.Name = "runc"
	app.Usage = usage

	var v []string
	if version != "" {
		v = append(v, version)
	}
	if gitCommit != "" {
		v = append(v, "commit: "+gitCommit)
	}
	v = append(v, "spec: "+specs.Version)
	v = append(v, "go: "+runtime.Version())

	major, minor, micro := seccomp.Version()
	if major+minor+micro > 0 {
		v = append(v, fmt.Sprintf("libseccomp: %d.%d.%d", major, minor, micro))
	}
	app.Version = strings.Join(v, "\n")

	xdgRuntimeDir := ""
	root := "/run/runc"
	if shouldHonorXDGRuntimeDir() {
		if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
			root = runtimeDir + "/runc"
			xdgRuntimeDir = root
		}
	}

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug output for logging",
		},
		cli.StringFlag{
			Name:  "log",
			Value: "",
			Usage: "set the log file path where internal debug information is written",
		},
		cli.StringFlag{
			Name:  "log-format",
			Value: "text",
			Usage: "set the format used by logs ('text' (default), or 'json')",
		},
		cli.StringFlag{
			Name:  "root",
			Value: root,
			Usage: "root directory for storage of container state (this should be located in tmpfs)",
		},
		cli.StringFlag{
			Name:  "criu",
			Value: "criu",
			Usage: "path to the criu binary used for checkpoint and restore",
		},
		cli.BoolFlag{
			Name:  "systemd-cgroup",
			Usage: "enable systemd cgroup support, expects cgroupsPath to be of form \"slice:prefix:name\" for e.g. \"system.slice:runc:434234\"",
		},
		cli.StringFlag{
			Name:  "rootless",
			Value: "auto",
			Usage: "ignore cgroup permission errors ('true', 'false', or 'auto')",
		},
	}
	app.Commands = []cli.Command{
		checkpointCommand,
		createCommand,
		deleteCommand,
		eventsCommand,
		execCommand,
		initCommand,
		killCommand,
		listCommand,
		pauseCommand,
		psCommand,
		restoreCommand,
		resumeCommand,
		runCommand,
		specCommand,
		startCommand,
		stateCommand,
		updateCommand,
	}
	app.Before = func(context *cli.Context) error {
		if !context.IsSet("root") && xdgRuntimeDir != "" {
			// According to the XDG specification, we need to set anything in
			// XDG_RUNTIME_DIR to have a sticky bit if we don't want it to get
			// auto-pruned.
			if err := os.MkdirAll(root, 0700); err != nil {
				fmt.Fprintln(os.Stderr, "the path in $XDG_RUNTIME_DIR must be writable by the user")
				fatal(err)
			}
			if err := os.Chmod(root, os.FileMode(0o700)|os.ModeSticky); err != nil {
				fmt.Fprintln(os.Stderr, "you should check permission of the path in $XDG_RUNTIME_DIR")
				fatal(err)
			}
		}
		if err := reviseRootDir(context); err != nil {
			return err
		}
		// let init configure logging on its own
		if args := context.Args(); args != nil && args.First() == "init" {
			return nil
		}
		return logs.ConfigureLogging(createLogConfig(context))
	}

	// If the command returns an error, cli takes upon itself to print
	// the error on cli.ErrWriter and exit.
	// Use our own writer here to ensure the log gets sent to the right location.
	cli.ErrWriter = &FatalWriter{cli.ErrWriter}
	if err := app.Run(os.Args); err != nil {
		fatal(err)
	}
}

type FatalWriter struct {
	cliErrWriter io.Writer
}

func (f *FatalWriter) Write(p []byte) (n int, err error) {
	logrus.Error(string(p))
	if !logrusToStderr() {
		return f.cliErrWriter.Write(p)
	}
	return len(p), nil
}

func createLogConfig(context *cli.Context) logs.Config {
	logFilePath := context.GlobalString("log")
	logPipeFd := 0
	if logFilePath == "" {
		logPipeFd = 2
	}
	config := logs.Config{
		LogPipeFd:   logPipeFd,
		LogLevel:    logrus.InfoLevel,
		LogFilePath: logFilePath,
		LogFormat:   context.GlobalString("log-format"),
		LogCaller:   context.GlobalBool("debug"),
	}
	if context.GlobalBool("debug") {
		config.LogLevel = logrus.DebugLevel
	}

	return config
}
