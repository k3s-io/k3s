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

package testutil

import (
	"bytes"
	"io"

	"github.com/containerd/stargz-snapshotter/estargz"
	digest "github.com/opencontainers/go-digest"
)

type buildEStargzOptions struct {
	estargzOptions  []estargz.Option
	buildTarOptions []BuildTarOption
}

type BuildEStargzOption func(o *buildEStargzOptions) error

// WithEStargzOptions specifies options for estargz lib
func WithEStargzOptions(eo ...estargz.Option) BuildEStargzOption {
	return func(o *buildEStargzOptions) error {
		o.estargzOptions = eo
		return nil
	}
}

// WithBuildTarOptions option specifies the options for tar creation
func WithBuildTarOptions(to ...BuildTarOption) BuildEStargzOption {
	return func(o *buildEStargzOptions) error {
		o.buildTarOptions = to
		return nil
	}
}

func BuildEStargz(ents []TarEntry, opts ...BuildEStargzOption) (*io.SectionReader, digest.Digest, error) {
	var beOpts buildEStargzOptions
	for _, o := range opts {
		o(&beOpts)
	}
	tarBuf := new(bytes.Buffer)
	if _, err := io.Copy(tarBuf, BuildTar(ents, beOpts.buildTarOptions...)); err != nil {
		return nil, "", err
	}
	tarData := tarBuf.Bytes()
	rc, err := estargz.Build(
		io.NewSectionReader(bytes.NewReader(tarData), 0, int64(len(tarData))),
		beOpts.estargzOptions...)
	if err != nil {
		return nil, "", err
	}
	defer rc.Close()
	vsb := new(bytes.Buffer)
	if _, err := io.Copy(vsb, rc); err != nil {
		return nil, "", err
	}
	vsbb := vsb.Bytes()

	return io.NewSectionReader(bytes.NewReader(vsbb), 0, int64(len(vsbb))), rc.TOCDigest(), nil
}
