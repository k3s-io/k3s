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
	"net/http"
	"net/url"
	"os"
	"os/signal"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"golang.org/x/net/context"
	restclient "k8s.io/client-go/rest"
	portforward "k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

var runtimePortForwardCommand = &cli.Command{
	Name:      "port-forward",
	Usage:     "Forward local port to a pod",
	ArgsUsage: "POD-ID [LOCAL_PORT:]REMOTE_PORT",
	Action: func(context *cli.Context) error {
		args := context.Args().Slice()
		if len(args) < 2 {
			return cli.ShowSubcommandHelp(context)
		}

		runtimeClient, runtimeConn, err := getRuntimeClient(context)
		if err != nil {
			return err
		}
		defer closeConnection(context, runtimeConn)

		var opts = portforwardOptions{
			id:    args[0],
			ports: args[1:],
		}
		err = PortForward(runtimeClient, opts)
		if err != nil {
			return errors.Wrap(err, "port forward")

		}
		return nil

	},
}

// PortForward sends an PortForwardRequest to server, and parses the returned PortForwardResponse
func PortForward(client pb.RuntimeServiceClient, opts portforwardOptions) error {
	if opts.id == "" {
		return fmt.Errorf("ID cannot be empty")

	}
	request := &pb.PortForwardRequest{
		PodSandboxId: opts.id,
	}
	logrus.Debugf("PortForwardRequest: %v", request)
	r, err := client.PortForward(context.Background(), request)
	logrus.Debugf("PortForwardResponse; %v", r)
	if err != nil {
		return err
	}
	portforwardURL := r.Url

	URL, err := url.Parse(portforwardURL)
	if err != nil {
		return err
	}

	if URL.Host == "" {
		URL.Host = kubeletURLHost
	}

	if URL.Scheme == "" {
		URL.Scheme = kubeletURLSchema
	}

	logrus.Debugf("PortForward URL: %v", URL)
	transport, upgrader, err := spdy.RoundTripperFor(&restclient.Config{})
	if err != nil {
		return err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", URL)

	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{})

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)

	go func() {
		<-signals
		if stopChan != nil {
			close(stopChan)
		}
	}()
	logrus.Debugf("Ports to forword: %v", opts.ports)
	pf, err := portforward.New(dialer, opts.ports, stopChan, readyChan, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}
	return pf.ForwardPorts()
}
