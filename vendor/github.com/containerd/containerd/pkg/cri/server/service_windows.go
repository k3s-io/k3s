// +build windows

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

package server

import (
	cni "github.com/containerd/go-cni"
	"github.com/pkg/errors"
)

// windowsNetworkAttachCount is the minimum number of networks the PodSandbox
// attaches to
const windowsNetworkAttachCount = 1

// initPlatform handles linux specific initialization for the CRI service.
func (c *criService) initPlatform() error {
	var err error
	// For windows, the loopback network is added as default.
	// There is no need to explicitly add one hence networkAttachCount is 1.
	// If there are more network configs the pod will be attached to all the
	// networks but we will only use the ip of the default network interface
	// as the pod IP.
	c.netPlugin, err = cni.New(cni.WithMinNetworkCount(windowsNetworkAttachCount),
		cni.WithPluginConfDir(c.config.NetworkPluginConfDir),
		cni.WithPluginMaxConfNum(c.config.NetworkPluginMaxConfNum),
		cni.WithPluginDir([]string{c.config.NetworkPluginBinDir}))
	if err != nil {
		return errors.Wrap(err, "failed to initialize cni")
	}

	return nil
}

// cniLoadOptions returns cni load options for the windows.
func (c *criService) cniLoadOptions() []cni.Opt {
	return []cni.Opt{cni.WithDefaultConf}
}
