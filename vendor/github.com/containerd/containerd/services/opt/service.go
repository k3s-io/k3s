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

package opt

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/containerd/containerd/plugin"
	"github.com/pkg/errors"
)

// Config for the opt manager
type Config struct {
	// Path for the opt directory
	Path string `toml:"path"`
}

func init() {
	plugin.Register(&plugin.Registration{
		Type: plugin.InternalPlugin,
		ID:   "opt",
		Config: &Config{
			Path: defaultPath,
		},
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			path := ic.Config.(*Config).Path
			ic.Meta.Exports["path"] = path

			bin := filepath.Join(path, "bin")
			if err := os.MkdirAll(bin, 0711); err != nil {
				return nil, err
			}
			if err := os.Setenv("PATH", fmt.Sprintf("%s:%s", bin, os.Getenv("PATH"))); err != nil {
				return nil, errors.Wrapf(err, "set binary image directory in path %s", bin)
			}
			if runtime.GOOS != "windows" {
				lib := filepath.Join(path, "lib")
				if err := os.MkdirAll(lib, 0711); err != nil {
					return nil, err
				}
				if err := os.Setenv("LD_LIBRARY_PATH", fmt.Sprintf("%s:%s", os.Getenv("LD_LIBRARY_PATH"), lib)); err != nil {
					return nil, errors.Wrapf(err, "set binary lib directory in path %s", lib)
				}
			}
			return &manager{}, nil
		},
	})
}

type manager struct {
}
