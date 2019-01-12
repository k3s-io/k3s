// +build linux

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

package apparmor

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

// WithProfile sets the provided apparmor profile to the spec
func WithProfile(profile string) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
		s.Process.ApparmorProfile = profile
		return nil
	}
}

// WithDefaultProfile will generate a default apparmor profile under the provided name
// for the container.  It is only generated if a profile under that name does not exist.
func WithDefaultProfile(name string) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
		yes, err := isLoaded(name)
		if err != nil {
			return err
		}
		if yes {
			s.Process.ApparmorProfile = name
			return nil
		}
		p, err := loadData(name)
		if err != nil {
			return err
		}
		f, err := ioutil.TempFile(os.Getenv("XDG_RUNTIME_DIR"), p.Name)
		if err != nil {
			return err
		}
		defer f.Close()
		path := f.Name()
		defer os.Remove(path)

		if err := generate(p, f); err != nil {
			return err
		}
		if err := load(path); err != nil {
			return errors.Wrapf(err, "load apparmor profile %s", path)
		}
		s.Process.ApparmorProfile = name
		return nil
	}
}
