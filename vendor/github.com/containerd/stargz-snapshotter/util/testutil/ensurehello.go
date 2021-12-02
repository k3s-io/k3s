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
	"compress/gzip"
	"context"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/content/local"
	"github.com/containerd/containerd/images/archive"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

const (
	// HelloArchiveURL points to an OCI archive of `hello-world`.
	// Exported from `docker.io/library/hello-world@sha256:1a523af650137b8accdaed439c17d684df61ee4d74feac151b5b337bd29e7eec` .
	// See https://github.com/AkihiroSuda/test-oci-archives/releases/tag/v20210101
	HelloArchiveURL = "https://github.com/AkihiroSuda/test-oci-archives/releases/download/v20210101/hello-world.tar.gz"
	// HelloArchiveDigest is the digest of the archive.
	HelloArchiveDigest = "sha256:5aa022621c4de0e941ab2a30d4569c403e156b4ba2de2ec32e382ae8679f40e1"
)

// EnsureHello creates a temp content store and ensures `hello-world` image from HelloArchiveURL into the store.
func EnsureHello(ctx context.Context) (*ocispec.Descriptor, content.Store, error) {
	// Pulling an image without the daemon is a mess, so we use OCI archive here.
	resp, err := http.Get(HelloArchiveURL)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	sha256Digester := digest.SHA256.Digester()
	sha256Hasher := sha256Digester.Hash()
	tr := io.TeeReader(resp.Body, sha256Hasher)
	gzReader, err := gzip.NewReader(tr)
	if err != nil {
		return nil, nil, err
	}

	tempDir, err := ioutil.TempDir("", "test-estargz")
	if err != nil {
		return nil, nil, err
	}

	cs, err := local.NewStore(tempDir)
	if err != nil {
		return nil, nil, err
	}

	desc, err := archive.ImportIndex(ctx, cs, gzReader)
	if err != nil {
		return nil, nil, err
	}
	resp.Body.Close()
	if d := sha256Digester.Digest().String(); d != HelloArchiveDigest {
		err = errors.Errorf("expected digest of %q to be %q, got %q", HelloArchiveURL, HelloArchiveDigest, d)
		return nil, nil, err
	}
	return &desc, cs, nil
}
