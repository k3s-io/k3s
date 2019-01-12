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

package cni

import (
	"fmt"
	"strings"
	"sync"

	cnilibrary "github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/pkg/errors"
)

type CNI interface {
	// Setup setup the network for the namespace
	Setup(id string, path string, opts ...NamespaceOpts) (*CNIResult, error)
	// Remove tears down the network of the namespace.
	Remove(id string, path string, opts ...NamespaceOpts) error
	// Load loads the cni network config
	Load(opts ...CNIOpt) error
	// Status checks the status of the cni initialization
	Status() error
}

type libcni struct {
	config

	cniConfig    cnilibrary.CNI
	networkCount int // minimum network plugin configurations needed to initialize cni
	networks     []*Network
	sync.RWMutex
}

func defaultCNIConfig() *libcni {
	return &libcni{
		config: config{
			pluginDirs:    []string{DefaultCNIDir},
			pluginConfDir: DefaultNetDir,
			prefix:        DefaultPrefix,
		},
		cniConfig: &cnilibrary.CNIConfig{
			Path: []string{DefaultCNIDir},
		},
		networkCount: 1,
	}
}

func New(config ...CNIOpt) (CNI, error) {
	cni := defaultCNIConfig()
	var err error
	for _, c := range config {
		if err = c(cni); err != nil {
			return nil, err
		}
	}
	return cni, nil
}

func (c *libcni) Load(opts ...CNIOpt) error {
	var err error
	c.Lock()
	defer c.Unlock()
	// Reset the networks on a load operation to ensure
	// config happens on a clean slate
	c.reset()

	for _, o := range opts {
		if err = o(c); err != nil {
			return errors.Wrapf(ErrLoad, fmt.Sprintf("cni config load failed: %v", err))
		}
	}
	return nil
}

func (c *libcni) Status() error {
	c.RLock()
	defer c.RUnlock()
	return c.status()
}

// Setup setups the network in the namespace
func (c *libcni) Setup(id string, path string, opts ...NamespaceOpts) (*CNIResult, error) {
	c.RLock()
	defer c.RUnlock()
	if err := c.status(); err != nil {
		return nil, err
	}
	ns, err := newNamespace(id, path, opts...)
	if err != nil {
		return nil, err
	}
	var results []*current.Result
	for _, network := range c.networks {
		r, err := network.Attach(ns)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return c.GetCNIResultFromResults(results)
}

// Remove removes the network config from the namespace
func (c *libcni) Remove(id string, path string, opts ...NamespaceOpts) error {
	c.RLock()
	defer c.RUnlock()
	if err := c.status(); err != nil {
		return err
	}
	ns, err := newNamespace(id, path, opts...)
	if err != nil {
		return err
	}
	for _, network := range c.networks {
		if err := network.Remove(ns); err != nil {
			// Based on CNI spec v0.7.0, empty network namespace is allowed to
			// do best effort cleanup. However, it is not handled consistently
			// right now:
			// https://github.com/containernetworking/plugins/issues/210
			// TODO(random-liu): Remove the error handling when the issue is
			// fixed and the CNI spec v0.6.0 support is deprecated.
			if path == "" && strings.Contains(err.Error(), "no such file or directory") {
				continue
			}
			return err
		}
	}
	return nil
}

func (c *libcni) reset() {
	c.networks = nil
}

func (c *libcni) status() error {
	if len(c.networks) < c.networkCount {
		return ErrCNINotInitialized
	}
	return nil
}
