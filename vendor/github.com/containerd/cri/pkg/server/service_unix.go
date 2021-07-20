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

package server

import (
	"github.com/containerd/containerd/sys"
	"github.com/containerd/cri/pkg/cap"
	cni "github.com/containerd/go-cni"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// networkAttachCount is the minimum number of networks the PodSandbox
// attaches to
const networkAttachCount = 2

// initPlatform handles linux specific initialization for the CRI service.
func (c *criService) initPlatform() error {
	var err error

	if sys.RunningInUserNS() {
		if !(c.config.DisableCgroup && !c.apparmorEnabled() && c.config.RestrictOOMScoreAdj) {
			logrus.Warn("Running containerd in a user namespace typically requires disable_cgroup, disable_apparmor, restrict_oom_score_adj set to be true")
		}
	}

	if c.config.EnableSelinux {
		if !selinux.GetEnabled() {
			logrus.Warn("Selinux is not supported")
		}
		if r := c.config.SelinuxCategoryRange; r > 0 {
			selinux.CategoryRange = uint32(r)
		}
	} else {
		selinux.SetDisabled()
	}

	// Pod needs to attach to at least loopback network and a non host network,
	// hence networkAttachCount is 2. If there are more network configs the
	// pod will be attached to all the networks but we will only use the ip
	// of the default network interface as the pod IP.
	c.netPlugin, err = cni.New(cni.WithMinNetworkCount(networkAttachCount),
		cni.WithPluginConfDir(c.config.NetworkPluginConfDir),
		cni.WithPluginMaxConfNum(c.config.NetworkPluginMaxConfNum),
		cni.WithPluginDir([]string{c.config.NetworkPluginBinDir}))
	if err != nil {
		return errors.Wrap(err, "failed to initialize cni")
	}

	if c.allCaps == nil {
		c.allCaps, err = cap.Current()
		if err != nil {
			return errors.Wrap(err, "failed to get caps")
		}
	}

	return nil
}

// cniLoadOptions returns cni load options for the linux.
func (c *criService) cniLoadOptions() []cni.CNIOpt {
	return []cni.CNIOpt{cni.WithLoNetwork, cni.WithDefaultConf}
}
