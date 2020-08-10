/*
Copyright 2020 The Kubernetes Authors.

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

package common

import (
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
)

// ServerConfiguration is the config for connecting to and using a CRI server
type ServerConfiguration struct {
	// RuntimeEndpoint is CRI server runtime endpoint
	RuntimeEndpoint string
	// ImageEndpoint is CRI server image endpoint
	ImageEndpoint string
	// Timeout  of connecting to server
	Timeout time.Duration
	// Debug enable debug output
	Debug bool
	// PullImageOnCreate enables also pulling an image for create requests
	PullImageOnCreate bool
	// DisablePullOnRun disables pulling an image for run requests
	DisablePullOnRun bool
}

// GetServerConfigFromFile returns the CRI server configuration from file
func GetServerConfigFromFile(configFileName, currentDir string) (*ServerConfiguration, error) {
	serverConfig := ServerConfiguration{}
	if _, err := os.Stat(configFileName); err != nil {
		if !os.IsNotExist(err) {
			return nil, errors.Wrap(err, "load config file")
		}
		// If the config file was not found, try looking in the program's
		// directory as a fallback. This is to accommodate where the config file
		// is placed with the cri tools binary.
		configFileName = filepath.Join(filepath.Dir(currentDir), "crictl.yaml")
		if _, err := os.Stat(configFileName); err != nil {
			return nil, errors.Wrap(err, "load config file")
		}
	}

	// Get config from file.
	config, err := ReadConfig(configFileName)
	if err != nil {
		return nil, errors.Wrap(err, "load config file")
	}

	// Set the config from file to the server config struct for return
	serverConfig.RuntimeEndpoint = config.RuntimeEndpoint
	serverConfig.ImageEndpoint = config.ImageEndpoint
	serverConfig.Timeout = time.Duration(config.Timeout) * time.Second
	serverConfig.Debug = config.Debug
	serverConfig.PullImageOnCreate = config.PullImageOnCreate
	serverConfig.DisablePullOnRun = config.DisablePullOnRun
	return &serverConfig, nil
}
