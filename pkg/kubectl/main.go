package kubectl

import (
	goflag "flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/rancher/k3s/pkg/server"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	utilflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	"k8s.io/kubernetes/pkg/kubectl/cmd"
)

func Main() {
	kubenv := os.Getenv("KUBECONFIG")
	if kubenv == "" {
		config, err := server.HomeKubeConfig(false, false)
		if _, serr := os.Stat(config); err == nil && serr == nil {
			os.Setenv("KUBECONFIG", config)
		}
		if err := checkReadConfigPermissions(config); err != nil {
			logrus.Warn(err)
		}
	}

	main()
}

func main() {
	rand.Seed(time.Now().UnixNano())

	command := cmd.NewDefaultKubectlCommand()

	// TODO: once we switch everything over to Cobra commands, we can go back to calling
	// utilflag.InitFlags() (by removing its pflag.Parse() call). For now, we have to set the
	// normalize func and add the go flag set by hand.
	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	// utilflag.InitFlags()
	logs.InitLogs()
	defer logs.FlushLogs()

	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func checkReadConfigPermissions(configFile string) error {
	file, err := os.OpenFile(configFile, os.O_RDONLY, 0600)
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("Unable to read %s, please start server "+
				"with --write-kubeconfig-mode to modify kube config permissions", configFile)
		}
	}
	file.Close()
	return nil
}
