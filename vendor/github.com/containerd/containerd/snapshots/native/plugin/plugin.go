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

package plugin

import (
	"errors"

	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/containerd/snapshots/native"
)

// Config represents configuration for the native plugin.
type Config struct {
	// Root directory for the plugin
	RootPath string `toml:"root_path"`
}

func init() {
	plugin.Register(&plugin.Registration{
		Type:   plugin.SnapshotPlugin,
		ID:     "native",
		Config: &Config{},
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			ic.Meta.Platforms = append(ic.Meta.Platforms, platforms.DefaultSpec())

			config, ok := ic.Config.(*Config)
			if !ok {
				return nil, errors.New("invalid native configuration")
			}

			root := ic.Root
			if len(config.RootPath) != 0 {
				root = config.RootPath
			}

			return native.NewSnapshotter(root)
		},
	})
}
