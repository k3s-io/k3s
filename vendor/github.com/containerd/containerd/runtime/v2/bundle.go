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

package v2

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/identifiers"
	"github.com/containerd/containerd/namespaces"
	"github.com/pkg/errors"
)

const configFilename = "config.json"

// LoadBundle loads an existing bundle from disk
func LoadBundle(ctx context.Context, root, id string) (*Bundle, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}
	return &Bundle{
		ID:        id,
		Path:      filepath.Join(root, ns, id),
		Namespace: ns,
	}, nil
}

// NewBundle returns a new bundle on disk
func NewBundle(ctx context.Context, root, state, id string, spec []byte) (b *Bundle, err error) {
	if err := identifiers.Validate(id); err != nil {
		return nil, errors.Wrapf(err, "invalid task id %s", id)
	}

	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}
	work := filepath.Join(root, ns, id)
	b = &Bundle{
		ID:        id,
		Path:      filepath.Join(state, ns, id),
		Namespace: ns,
	}
	var paths []string
	defer func() {
		if err != nil {
			for _, d := range paths {
				os.RemoveAll(d)
			}
		}
	}()
	// create state directory for the bundle
	if err := os.MkdirAll(filepath.Dir(b.Path), 0711); err != nil {
		return nil, err
	}
	if err := os.Mkdir(b.Path, 0711); err != nil {
		return nil, err
	}
	paths = append(paths, b.Path)
	// create working directory for the bundle
	if err := os.MkdirAll(filepath.Dir(work), 0711); err != nil {
		return nil, err
	}
	if err := os.Mkdir(work, 0711); err != nil {
		if !os.IsExist(err) {
			return nil, err
		}
		os.RemoveAll(work)
		if err := os.Mkdir(work, 0711); err != nil {
			return nil, err
		}
	}
	paths = append(paths, work)
	// create rootfs dir
	if err := os.Mkdir(filepath.Join(b.Path, "rootfs"), 0711); err != nil {
		return nil, err
	}
	// symlink workdir
	if err := os.Symlink(work, filepath.Join(b.Path, "work")); err != nil {
		return nil, err
	}
	// write the spec to the bundle
	err = ioutil.WriteFile(filepath.Join(b.Path, configFilename), spec, 0666)
	return b, err
}

// Bundle represents an OCI bundle
type Bundle struct {
	// ID of the bundle
	ID string
	// Path to the bundle
	Path string
	// Namespace of the bundle
	Namespace string
}

// Delete a bundle atomically
func (b *Bundle) Delete() error {
	work, werr := os.Readlink(filepath.Join(b.Path, "work"))
	err := os.RemoveAll(b.Path)
	if err == nil {
		if werr == nil {
			return os.RemoveAll(work)
		}
		return nil
	}
	// error removing the bundle path; still attempt removing work dir
	var err2 error
	if werr == nil {
		err2 = os.RemoveAll(work)
		if err2 == nil {
			return err
		}
	}
	return errors.Wrapf(err, "failed to remove both bundle and workdir locations: %v", err2)
}
