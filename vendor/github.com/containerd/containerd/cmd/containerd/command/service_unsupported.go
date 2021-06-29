// +build !windows

/*
   Copyright The containerd Authors.

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

package command

import (
	"github.com/containerd/containerd/services/server"
	"github.com/urfave/cli"
)

// serviceFlags returns an array of flags for configuring containerd to run
// as a service. Only relevant on Windows.
func serviceFlags() []cli.Flag {
	return nil
}

// applyPlatformFlags applies platform-specific flags.
func applyPlatformFlags(context *cli.Context) {
}

// registerUnregisterService is only relevant on Windows.
func registerUnregisterService(root string) (bool, error) {
	return false, nil
}

// launchService is only relevant on Windows.
func launchService(s *server.Server, done chan struct{}) error {
	return nil
}
