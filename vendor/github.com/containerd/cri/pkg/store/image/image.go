/*
Copyright 2017 The Kubernetes Authors.

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

package image

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/docker/distribution/digestset"
	imagedigest "github.com/opencontainers/go-digest"
	imageidentity "github.com/opencontainers/image-spec/identity"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"

	storeutil "github.com/containerd/cri/pkg/store"
	"github.com/containerd/cri/pkg/util"
)

// Image contains all resources associated with the image. All fields
// MUST not be mutated directly after created.
type Image struct {
	// Id of the image. Normally the digest of image config.
	ID string
	// References are references to the image, e.g. RepoTag and RepoDigest.
	References []string
	// ChainID is the chainID of the image.
	ChainID string
	// Size is the compressed size of the image.
	Size int64
	// ImageSpec is the oci image structure which describes basic information about the image.
	ImageSpec imagespec.Image
	// Containerd image reference
	Image containerd.Image
}

// Store stores all images.
type Store struct {
	lock sync.RWMutex
	// refCache is a containerd image reference to image id cache.
	refCache map[string]string
	// client is the containerd client.
	client *containerd.Client
	// store is the internal image store indexed by image id.
	store *store
}

// NewStore creates an image store.
func NewStore(client *containerd.Client) *Store {
	return &Store{
		refCache: make(map[string]string),
		client:   client,
		store: &store{
			images:    make(map[string]Image),
			digestSet: digestset.NewSet(),
		},
	}
}

// Update updates cache for a reference.
func (s *Store) Update(ctx context.Context, ref string) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	i, err := s.client.GetImage(ctx, ref)
	if err != nil && !errdefs.IsNotFound(err) {
		return errors.Wrap(err, "get image from containerd")
	}
	var img *Image
	if err == nil {
		img, err = getImage(ctx, i)
		if err != nil {
			return errors.Wrap(err, "get image info from containerd")
		}
	}
	return s.update(ref, img)
}

// update updates the internal cache. img == nil means that
// the image does not exist in containerd.
func (s *Store) update(ref string, img *Image) error {
	oldID, oldExist := s.refCache[ref]
	if img == nil {
		// The image reference doesn't exist in containerd.
		if oldExist {
			// Remove the reference from the store.
			s.store.delete(oldID, ref)
			delete(s.refCache, ref)
		}
		return nil
	}
	if oldExist {
		if oldID == img.ID {
			return nil
		}
		// Updated. Remove tag from old image.
		s.store.delete(oldID, ref)
	}
	// New image. Add new image.
	s.refCache[ref] = img.ID
	return s.store.add(*img)
}

// getImage gets image information from containerd.
func getImage(ctx context.Context, i containerd.Image) (*Image, error) {
	// Get image information.
	diffIDs, err := i.RootFS(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get image diffIDs")
	}
	chainID := imageidentity.ChainID(diffIDs)

	size, err := i.Size(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get image compressed resource size")
	}

	desc, err := i.Config(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get image config descriptor")
	}
	id := desc.Digest.String()

	rb, err := content.ReadBlob(ctx, i.ContentStore(), desc)
	if err != nil {
		return nil, errors.Wrap(err, "read image config from content store")
	}
	var ociimage imagespec.Image
	if err := json.Unmarshal(rb, &ociimage); err != nil {
		return nil, errors.Wrapf(err, "unmarshal image config %s", rb)
	}

	return &Image{
		ID:         id,
		References: []string{i.Name()},
		ChainID:    chainID.String(),
		Size:       size,
		ImageSpec:  ociimage,
		Image:      i,
	}, nil
}

// Resolve resolves a image reference to image id.
func (s *Store) Resolve(ref string) (string, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	id, ok := s.refCache[ref]
	if !ok {
		return "", storeutil.ErrNotExist
	}
	return id, nil
}

// Get gets image metadata by image id. The id can be truncated.
// Returns various validation errors if the image id is invalid.
// Returns storeutil.ErrNotExist if the image doesn't exist.
func (s *Store) Get(id string) (Image, error) {
	return s.store.get(id)
}

// List lists all images.
func (s *Store) List() []Image {
	return s.store.list()
}

type store struct {
	lock      sync.RWMutex
	images    map[string]Image
	digestSet *digestset.Set
}

func (s *store) list() []Image {
	s.lock.RLock()
	defer s.lock.RUnlock()
	var images []Image
	for _, i := range s.images {
		images = append(images, i)
	}
	return images
}

func (s *store) add(img Image) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, err := s.digestSet.Lookup(img.ID); err != nil {
		if err != digestset.ErrDigestNotFound {
			return err
		}
		if err := s.digestSet.Add(imagedigest.Digest(img.ID)); err != nil {
			return err
		}
	}

	i, ok := s.images[img.ID]
	if !ok {
		// If the image doesn't exist, add it.
		s.images[img.ID] = img
		return nil
	}
	// Or else, merge the references.
	i.References = util.MergeStringSlices(i.References, img.References)
	s.images[img.ID] = i
	return nil
}

func (s *store) get(id string) (Image, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	digest, err := s.digestSet.Lookup(id)
	if err != nil {
		if err == digestset.ErrDigestNotFound {
			err = storeutil.ErrNotExist
		}
		return Image{}, err
	}
	if i, ok := s.images[digest.String()]; ok {
		return i, nil
	}
	return Image{}, storeutil.ErrNotExist
}

func (s *store) delete(id, ref string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	digest, err := s.digestSet.Lookup(id)
	if err != nil {
		// Note: The idIndex.Delete and delete doesn't handle truncated index.
		// So we need to return if there are error.
		return
	}
	i, ok := s.images[digest.String()]
	if !ok {
		return
	}
	i.References = util.SubtractStringSlice(i.References, ref)
	if len(i.References) != 0 {
		s.images[digest.String()] = i
		return
	}
	// Remove the image if it is not referenced any more.
	s.digestSet.Remove(digest) // nolint: errcheck
	delete(s.images, digest.String())
}
