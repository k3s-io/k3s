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
	"sort"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"google.golang.org/grpc"

	internalapi "k8s.io/cri-api/pkg/apis"
	"k8s.io/kubernetes/pkg/kubelet/cri/remote"
	"k8s.io/kubernetes/pkg/kubelet/util"

	"github.com/kubernetes-sigs/cri-tools/pkg/common"
	"github.com/kubernetes-sigs/cri-tools/pkg/version"
)

const (
	defaultTimeout = 2 * time.Second
)

var (
	// RuntimeEndpoint is CRI server runtime endpoint
	RuntimeEndpoint string
	// RuntimeEndpointIsSet is true when RuntimeEndpoint is configured
	RuntimeEndpointIsSet bool
	// ImageEndpoint is CRI server image endpoint, default same as runtime endpoint
	ImageEndpoint string
	// ImageEndpointIsSet is true when ImageEndpoint is configured
	ImageEndpointIsSet bool
	// Timeout  of connecting to server (default: 10s)
	Timeout time.Duration
	// Debug enable debug output
	Debug bool
	// PullImageOnCreate enables pulling image on create requests
	PullImageOnCreate bool
	// DisablePullOnRun disable pulling image on run requests
	DisablePullOnRun bool
)

func getRuntimeClientConnection(context *cli.Context) (*grpc.ClientConn, error) {
	if RuntimeEndpointIsSet && RuntimeEndpoint == "" {
		return nil, fmt.Errorf("--runtime-endpoint is not set")
	}
	logrus.Debug("get runtime connection")
	// If no EP set then use the default endpoint types
	if !RuntimeEndpointIsSet {
		logrus.Warningf("runtime connect using default endpoints: %v. "+
			"As the default settings are now deprecated, you should set the "+
			"endpoint instead.", defaultRuntimeEndpoints)
		logrus.Debug("Note that performance maybe affected as each default " +
			"connection attempt takes n-seconds to complete before timing out " +
			"and going to the next in sequence.")
		return getConnection(defaultRuntimeEndpoints)
	}
	return getConnection([]string{RuntimeEndpoint})
}

func getImageClientConnection(context *cli.Context) (*grpc.ClientConn, error) {
	if ImageEndpoint == "" {
		if RuntimeEndpointIsSet && RuntimeEndpoint == "" {
			return nil, fmt.Errorf("--image-endpoint is not set")
		}
		ImageEndpoint = RuntimeEndpoint
		ImageEndpointIsSet = RuntimeEndpointIsSet
	}
	logrus.Debugf("get image connection")
	// If no EP set then use the default endpoint types
	if !ImageEndpointIsSet {
		logrus.Warningf("image connect using default endpoints: %v. "+
			"As the default settings are now deprecated, you should set the "+
			"endpoint instead.", defaultRuntimeEndpoints)
		logrus.Debug("Note that performance maybe affected as each default " +
			"connection attempt takes n-seconds to complete before timing out " +
			"and going to the next in sequence.")
		return getConnection(defaultRuntimeEndpoints)
	}
	return getConnection([]string{ImageEndpoint})
}

func getConnection(endPoints []string) (*grpc.ClientConn, error) {
	if endPoints == nil || len(endPoints) == 0 {
		return nil, fmt.Errorf("endpoint is not set")
	}
	endPointsLen := len(endPoints)
	var conn *grpc.ClientConn
	for indx, endPoint := range endPoints {
		logrus.Debugf("connect using endpoint '%s' with '%s' timeout", endPoint, Timeout)
		addr, dialer, err := util.GetAddressAndDialer(endPoint)
		if err != nil {
			if indx == endPointsLen-1 {
				return nil, err
			}
			logrus.Error(err)
			continue
		}
		conn, err = grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(Timeout), grpc.WithContextDialer(dialer))
		if err != nil {
			errMsg := errors.Wrapf(err, "connect endpoint '%s', make sure you are running as root and the endpoint has been started", endPoint)
			if indx == endPointsLen-1 {
				return nil, errMsg
			}
			logrus.Error(errMsg)
		} else {
			logrus.Debugf("connected successfully using endpoint: %s", endPoint)
			break
		}
	}
	return conn, nil
}

func getRuntimeService(context *cli.Context) (internalapi.RuntimeService, error) {
	return remote.NewRemoteRuntimeService(RuntimeEndpoint, Timeout)
}

func getTimeout(timeDuration time.Duration) time.Duration {
	if timeDuration.Seconds() > 0 {
		return timeDuration
	}
	return defaultTimeout // use default
}

func Main() {
	app := cli.NewApp()
	app.Name = "crictl"
	app.Usage = "client for CRI"
	app.Version = version.Version

	app.Commands = []*cli.Command{
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
		runContainerCommand,
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

	runtimeEndpointUsage := fmt.Sprintf("Endpoint of CRI container runtime "+
		"service (default: uses in order the first successful one of %v). "+
		"Default is now deprecated and the endpoint should be set instead.",
		defaultRuntimeEndpoints)

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			EnvVars: []string{"CRI_CONFIG_FILE"},
			Value:   defaultConfigPath,
			Usage:   "Location of the client config file. If not specified and the default does not exist, the program's directory is searched as well",
		},
		&cli.StringFlag{
			Name:    "runtime-endpoint",
			Aliases: []string{"r"},
			EnvVars: []string{"CONTAINER_RUNTIME_ENDPOINT"},
			Usage:   runtimeEndpointUsage,
		},
		&cli.StringFlag{
			Name:    "image-endpoint",
			Aliases: []string{"i"},
			EnvVars: []string{"IMAGE_SERVICE_ENDPOINT"},
			Usage: "Endpoint of CRI image manager service (default: uses " +
				"'runtime-endpoint' setting)",
		},
		&cli.DurationFlag{
			Name:    "timeout",
			Aliases: []string{"t"},
			Value:   defaultTimeout,
			Usage: "Timeout of connecting to the server in seconds (e.g. 2s, 20s.). " +
				"0 or less is set to default",
		},
		&cli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"D"},
			Usage:   "Enable debug mode",
		},
	}

	app.Before = func(context *cli.Context) (err error) {
		var config *common.ServerConfiguration
		var exePath string

		if exePath, err = os.Executable(); err != nil {
			logrus.Fatal(err)
		}
		if config, err = common.GetServerConfigFromFile(context.String("config"), exePath); err != nil {
			if context.IsSet("config") {
				logrus.Fatal(err)
			}
		}

		if config == nil {
			RuntimeEndpoint = context.String("runtime-endpoint")
			if context.IsSet("runtime-endpoint") {
				RuntimeEndpointIsSet = true
			}
			ImageEndpoint = context.String("image-endpoint")
			if context.IsSet("image-endpoint") {
				ImageEndpointIsSet = true
			}
			if context.IsSet("timeout") {
				Timeout = getTimeout(context.Duration("timeout"))
			} else {
				Timeout = context.Duration("timeout")
			}
			Debug = context.Bool("debug")
			DisablePullOnRun = false
		} else {
			// Command line flags overrides config file.
			if context.IsSet("runtime-endpoint") {
				RuntimeEndpoint = context.String("runtime-endpoint")
				RuntimeEndpointIsSet = true
			} else if config.RuntimeEndpoint != "" {
				RuntimeEndpoint = config.RuntimeEndpoint
				RuntimeEndpointIsSet = true
			} else {
				RuntimeEndpoint = context.String("runtime-endpoint")
			}
			if context.IsSet("image-endpoint") {
				ImageEndpoint = context.String("image-endpoint")
				ImageEndpointIsSet = true
			} else if config.ImageEndpoint != "" {
				ImageEndpoint = config.ImageEndpoint
				ImageEndpointIsSet = true
			} else {
				ImageEndpoint = context.String("image-endpoint")
			}
			if context.IsSet("timeout") {
				Timeout = getTimeout(context.Duration("timeout"))
			} else if config.Timeout > 0 { // 0/neg value set to default timeout
				Timeout = config.Timeout
			} else {
				Timeout = context.Duration("timeout")
			}
			if context.IsSet("debug") {
				Debug = context.Bool("debug")
			} else {
				Debug = config.Debug
			}
			PullImageOnCreate = config.PullImageOnCreate
			DisablePullOnRun = config.DisablePullOnRun
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
