/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package crictl

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
	internalapi "k8s.io/cri-api/pkg/apis"
	"k8s.io/kubernetes/pkg/kubelet/remote"
	"k8s.io/kubernetes/pkg/kubelet/util"

	"github.com/kubernetes-sigs/cri-tools/pkg/version"
)

const (
	defaultTimeout = 2 * time.Second
)

var (
	// RuntimeEndpoint is CRI server runtime endpoint
	RuntimeEndpoint string
	// ImageEndpoint is CRI server image endpoint, default same as runtime endpoint
	ImageEndpoint string
	// Timeout  of connecting to server (default: 10s)
	Timeout time.Duration
	// Debug enable debug output
	Debug bool
)

func getRuntimeClientConnection(context *cli.Context) (*grpc.ClientConn, error) {
	if RuntimeEndpoint == "" {
		return nil, fmt.Errorf("--runtime-endpoint is not set")
	}

	addr, dialer, err := util.GetAddressAndDialer(RuntimeEndpoint)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(Timeout), grpc.WithDialer(dialer))
	if err != nil {
		return nil, fmt.Errorf("failed to connect, make sure you are running as root and the runtime has been started: %v", err)
	}
	return conn, nil
}

func getImageClientConnection(context *cli.Context) (*grpc.ClientConn, error) {
	if ImageEndpoint == "" {
		if RuntimeEndpoint == "" {
			return nil, fmt.Errorf("--image-endpoint is not set")
		}
		ImageEndpoint = RuntimeEndpoint
	}

	addr, dialer, err := util.GetAddressAndDialer(ImageEndpoint)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(Timeout), grpc.WithDialer(dialer))
	if err != nil {
		return nil, fmt.Errorf("failed to connect, make sure you are running as root and the runtime has been started: %v", err)
	}
	return conn, nil
}

func getRuntimeService(context *cli.Context) (internalapi.RuntimeService, error) {
	return remote.NewRemoteRuntimeService(RuntimeEndpoint, Timeout)
}

func Main() {
	app := cli.NewApp()
	app.Name = "crictl"
	app.Usage = "client for CRI"
	app.Version = version.Version

	app.Commands = []cli.Command{
		runtimeAttachCommand,
		createContainerCommand,
		runtimeExecCommand,
		runtimeVersionCommand,
		listImageCommand,
		containerStatusCommand,
		imageStatusCommand,
		imageFsInfoCommand,
		podStatusCommand,
		logsCommand,
		runtimePortForwardCommand,
		listContainersCommand,
		pullImageCommand,
		runPodCommand,
		removeContainerCommand,
		removeImageCommand,
		removePodCommand,
		listPodCommand,
		startContainerCommand,
		runtimeStatusCommand,
		stopContainerCommand,
		stopPodCommand,
		updateContainerCommand,
		configCommand,
		statsCommand,
		completionCommand,
	}

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "config, c",
			EnvVar: "CRI_CONFIG_FILE",
			Value:  defaultConfigPath,
			Usage:  "Location of the client config file. If not specified and the default does not exist, the program's directory is searched as well",
		},
		cli.StringFlag{
			Name:   "runtime-endpoint, r",
			EnvVar: "CONTAINER_RUNTIME_ENDPOINT",
			Value:  defaultRuntimeEndpoint,
			Usage:  "Endpoint of CRI container runtime service",
		},
		cli.StringFlag{
			Name:   "image-endpoint, i",
			EnvVar: "IMAGE_SERVICE_ENDPOINT",
			Usage:  "Endpoint of CRI image manager service",
		},
		cli.DurationFlag{
			Name:  "timeout, t",
			Value: defaultTimeout,
			Usage: "Timeout of connecting to the server",
		},
		cli.BoolFlag{
			Name:  "debug, D",
			Usage: "Enable debug mode",
		},
	}

	app.Before = func(context *cli.Context) error {
		isUseConfig := false
		configFile := context.GlobalString("config")
		if _, err := os.Stat(configFile); err == nil {
			isUseConfig = true
		} else {
			if context.IsSet("config") || !os.IsNotExist(err) {
				// note: the absence of default config file is normal case
				// when user have not set it in cli
				logrus.Fatalf("Failed to load config file: %v", err)
			} else {
				// If the default config was not found, and the user didn't
				// explicitly specify a config, try looking in the program's
				// directory as a fallback. This is a convenience for
				// deployments of crictl so they don't have to place a file in a
				// global location.
				configFile = filepath.Join(filepath.Dir(os.Args[0]), "crictl.yaml")
				if _, err := os.Stat(configFile); err == nil {
					isUseConfig = true
				} else if !os.IsNotExist(err) {
					logrus.Fatalf("Failed to load config file: %v", err)
				}
			}
		}

		if !isUseConfig {
			RuntimeEndpoint = context.GlobalString("runtime-endpoint")
			ImageEndpoint = context.GlobalString("image-endpoint")
			Timeout = context.GlobalDuration("timeout")
			Debug = context.GlobalBool("debug")
		} else {
			// Get config from file.
			config, err := ReadConfig(configFile)
			if err != nil {
				logrus.Fatalf("Falied to load config file: %v", err)
			}

			// Command line flags overrides config file.
			if context.IsSet("runtime-endpoint") {
				RuntimeEndpoint = context.String("runtime-endpoint")
			} else if config.RuntimeEndpoint != "" {
				RuntimeEndpoint = config.RuntimeEndpoint
			} else {
				RuntimeEndpoint = context.GlobalString("runtime-endpoint")
			}
			if context.IsSet("image-endpoint") {
				ImageEndpoint = context.String("image-endpoint")
			} else if config.ImageEndpoint != "" {
				ImageEndpoint = config.ImageEndpoint
			} else {
				ImageEndpoint = context.GlobalString("image-endpoint")
			}
			if context.IsSet("timeout") {
				Timeout = context.Duration("timeout")
			} else if config.Timeout != 0 {
				Timeout = time.Duration(config.Timeout) * time.Second
			} else {
				Timeout = context.GlobalDuration("timeout")
			}
			if context.IsSet("debug") {
				Debug = context.GlobalBool("debug")
			} else {
				Debug = config.Debug
			}
		}

		if Debug {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}
	// sort all flags
	for _, cmd := range app.Commands {
		sort.Sort(cli.FlagsByName(cmd.Flags))
	}
	sort.Sort(cli.FlagsByName(app.Flags))

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
