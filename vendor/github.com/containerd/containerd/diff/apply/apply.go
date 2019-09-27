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

package apply

import (
	"context"
	"io"
	"io/ioutil"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/diff"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// NewFileSystemApplier returns an applier which simply mounts
// and applies diff onto the mounted filesystem.
func NewFileSystemApplier(cs content.Provider) diff.Applier {
	return &fsApplier{
		store: cs,
	}
}

type fsApplier struct {
	store content.Provider
}

var emptyDesc = ocispec.Descriptor{}

// Apply applies the content associated with the provided digests onto the
// provided mounts. Archive content will be extracted and decompressed if
// necessary.
func (s *fsApplier) Apply(ctx context.Context, desc ocispec.Descriptor, mounts []mount.Mount, opts ...diff.ApplyOpt) (d ocispec.Descriptor, err error) {
	t1 := time.Now()
	defer func() {
		if err == nil {
			log.G(ctx).WithFields(logrus.Fields{
				"d":     time.Since(t1),
				"dgst":  desc.Digest,
				"size":  desc.Size,
				"media": desc.MediaType,
			}).Debugf("diff applied")
		}
	}()

	var config diff.ApplyConfig
	for _, o := range opts {
		if err := o(ctx, desc, &config); err != nil {
			return emptyDesc, errors.Wrap(err, "failed to apply config opt")
		}
	}

	ra, err := s.store.ReaderAt(ctx, desc)
	if err != nil {
		return emptyDesc, errors.Wrap(err, "failed to get reader from content store")
	}
	defer ra.Close()

	var processors []diff.StreamProcessor
	processor := diff.NewProcessorChain(desc.MediaType, content.NewReader(ra))
	processors = append(processors, processor)
	for {
		if processor, err = diff.GetProcessor(ctx, processor, config.ProcessorPayloads); err != nil {
			return emptyDesc, errors.Wrapf(err, "failed to get stream processor for %s", desc.MediaType)
		}
		processors = append(processors, processor)
		if processor.MediaType() == ocispec.MediaTypeImageLayer {
			break
		}
	}
	defer processor.Close()

	digester := digest.Canonical.Digester()
	rc := &readCounter{
		r: io.TeeReader(processor, digester.Hash()),
	}

	if err := apply(ctx, mounts, rc); err != nil {
		return emptyDesc, err
	}

	// Read any trailing data
	if _, err := io.Copy(ioutil.Discard, rc); err != nil {
		return emptyDesc, err
	}

	for _, p := range processors {
		if ep, ok := p.(interface {
			Err() error
		}); ok {
			if err := ep.Err(); err != nil {
				return emptyDesc, err
			}
		}
	}
	return ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageLayer,
		Size:      rc.c,
		Digest:    digester.Digest(),
	}, nil
}

type readCounter struct {
	r io.Reader
	c int64
}

func (rc *readCounter) Read(p []byte) (n int, err error) {
	n, err = rc.r.Read(p)
	rc.c += int64(n)
	return
}
