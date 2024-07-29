package cmds

import (
	"fmt"
	"os"
	"runtime"

	"github.com/k3s-io/k3s/pkg/version"
	"github.com/urfave/cli"
)

var (
	Debug     bool
	DebugFlag = &cli.BoolFlag{
		Name:        "debug",
		Usage:       "(logging) Turn on debug logs",
		Destination: &Debug,
		EnvVar:      version.ProgramUpper + "_DEBUG",
	}
	PreferBundledBin = &cli.BoolFlag{
		Name:  "prefer-bundled-bin",
		Usage: "(experimental) Prefer bundled userspace binaries over host binaries",
	}
)

func init() {
	// hack - force "file,dns" lookup order if go dns is used
	if os.Getenv("RES_OPTIONS") == "" {
		os.Setenv("RES_OPTIONS", " ")
	}
}

func NewApp() *cli.App {
	app := cli.NewApp()
	app.Name = appName
	app.Usage = "Kubernetes, but small and simple"
	app.Version = fmt.Sprintf("%s (%s)", version.Version, version.GitCommit)
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("%s version %s\n", app.Name, app.Version)
		fmt.Printf("go version %s\n", runtime.Version())
	}
	app.Flags = []cli.Flag{
		DebugFlag,
		DataDirFlag,
	}

	return app
}
