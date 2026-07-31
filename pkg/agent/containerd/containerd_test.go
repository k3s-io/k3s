package containerd

import (
	"context"
	"testing"

	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/errdefs"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testDigest = "sha256:72fe734697c5d327a7549e65930898da999a5938f194a3e50340768b7de10604"

func Test_UnitImageTagNames(t *testing.T) {
	tests := []struct {
		name       string
		imageName  string
		registries []string
		want       []string
		wantErr    bool
	}{
		{
			name:      "no registries",
			imageName: "docker.io/library/nginx:1.29.4",
			want: []string{
				"docker.io/library/nginx@" + testDigest,
			},
		},
		{
			name:       "distinct registry",
			imageName:  "docker.io/library/nginx:1.29.4",
			registries: []string{"registry.example.com"},
			want: []string{
				"docker.io/library/nginx@" + testDigest,
				"registry.example.com/library/nginx:1.29.4",
				"registry.example.com/library/nginx@" + testDigest,
			},
		},
		{
			// Not hosted by the registry yet, so the path is prepended exactly once.
			name:       "registry with path component",
			imageName:  "docker.io/library/nginx:1.29.4",
			registries: []string{"registry.example.com/mirror"},
			want: []string{
				"docker.io/library/nginx@" + testDigest,
				"registry.example.com/mirror/library/nginx:1.29.4",
				"registry.example.com/mirror/library/nginx@" + testDigest,
			},
		},
		{
			// system-default-registry set to the registry the image is already tagged
			// for: every name we would generate is one the image already has.
			name:       "registry matching image domain",
			imageName:  "registry.example.com/library/nginx:1.29.4",
			registries: []string{"registry.example.com"},
			want: []string{
				"registry.example.com/library/nginx@" + testDigest,
			},
		},
		{
			// As above, but with a path component the image name already carries;
			// prepending it would repeat on every import. See rancher/rke2#10822.
			name:       "registry with path component matching image",
			imageName:  "registry.example.com/mirror/library/nginx:1.29.4",
			registries: []string{"registry.example.com/mirror"},
			want: []string{
				"registry.example.com/mirror/library/nginx@" + testDigest,
			},
		},
		{
			// An image that already accumulated repeated components must not gain more.
			name:       "registry with path component already repeated",
			imageName:  "registry.example.com/mirror/mirror/library/nginx:1.29.4",
			registries: []string{"registry.example.com/mirror"},
			want: []string{
				"registry.example.com/mirror/mirror/library/nginx@" + testDigest,
			},
		},
		{
			// A prefix of the image path, but not a path-segment prefix, so the image
			// is not hosted by it and must still be retagged.
			name:       "registry is a partial path segment",
			imageName:  "registry.example.com/mirrored/library/nginx:1.29.4",
			registries: []string{"registry.example.com/mirror"},
			want: []string{
				"registry.example.com/mirrored/library/nginx@" + testDigest,
				"registry.example.com/mirror/mirrored/library/nginx:1.29.4",
				"registry.example.com/mirror/mirrored/library/nginx@" + testDigest,
			},
		},
		{
			name:       "duplicate registries",
			imageName:  "docker.io/library/nginx:1.29.4",
			registries: []string{"registry.example.com", "registry.example.com"},
			want: []string{
				"docker.io/library/nginx@" + testDigest,
				"registry.example.com/library/nginx:1.29.4",
				"registry.example.com/library/nginx@" + testDigest,
			},
		},
		{
			// A trailing slash is trimmed, so it neither doubles the separator nor
			// counts as distinct from the same registry written without one.
			name:       "registry with trailing slash",
			imageName:  "docker.io/library/nginx:1.29.4",
			registries: []string{"registry.example.com/", "registry.example.com"},
			want: []string{
				"docker.io/library/nginx@" + testDigest,
				"registry.example.com/library/nginx:1.29.4",
				"registry.example.com/library/nginx@" + testDigest,
			},
		},
		{
			name:      "untagged image",
			imageName: "docker.io/library/nginx@" + testDigest,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			image := images.Image{
				Name:   tt.imageName,
				Target: ocispec.Descriptor{Digest: testDigest},
			}

			got, err := imageTagNames(image, tt.registries)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.NotContains(t, got, tt.imageName, "must not retag an image with its own name")
		})
	}
}

// fakeImageStore implements the subset of images.Store used by forceCreateTag, and
// optionally simulates another writer mutating a record between calls.
type fakeImageStore struct {
	images.Store
	records map[string]images.Image
	creates int
	updates int
	deletes int
	// onCall is invoked before each store operation with the operation name and the
	// 1-based call count, and may mutate records to simulate a concurrent writer.
	onCall func(f *fakeImageStore, op string, call int)
	calls  int
}

func newFakeImageStore() *fakeImageStore {
	return &fakeImageStore{records: map[string]images.Image{}}
}

func (f *fakeImageStore) hook(op string) {
	f.calls++
	if f.onCall != nil {
		f.onCall(f, op, f.calls)
	}
}

func (f *fakeImageStore) Create(ctx context.Context, image images.Image) (images.Image, error) {
	f.hook("create")
	f.creates++
	if _, ok := f.records[image.Name]; ok {
		return images.Image{}, errdefs.ErrAlreadyExists
	}
	f.records[image.Name] = image
	return image, nil
}

func (f *fakeImageStore) Update(ctx context.Context, image images.Image, fieldpaths ...string) (images.Image, error) {
	f.hook("update")
	f.updates++
	if _, ok := f.records[image.Name]; !ok {
		return images.Image{}, errdefs.ErrNotFound
	}
	f.records[image.Name] = image
	return image, nil
}

func (f *fakeImageStore) Delete(ctx context.Context, name string, opts ...images.DeleteOpt) error {
	f.hook("delete")
	f.deletes++
	if _, ok := f.records[name]; !ok {
		return errdefs.ErrNotFound
	}
	delete(f.records, name)
	return nil
}

func Test_UnitForceCreateTag(t *testing.T) {
	const targetRef = "registry.example.com/library/nginx@" + testDigest

	newImage := func(content string) images.Image {
		return images.Image{
			Name:   "docker.io/library/nginx:1.29.4",
			Target: ocispec.Descriptor{Digest: digest.FromString(content)},
		}
	}

	t.Run("creates a new record", func(t *testing.T) {
		store := newFakeImageStore()
		image := newImage("a")

		require.NoError(t, forceCreateTag(context.Background(), store, image, targetRef))
		assert.Equal(t, image.Target, store.records[targetRef].Target)
		assert.Equal(t, 1, store.creates)
		assert.Equal(t, 0, store.updates)
	})

	t.Run("overwrites an existing record without deleting it", func(t *testing.T) {
		store := newFakeImageStore()
		store.records[targetRef] = newImage("stale")
		image := newImage("fresh")

		require.NoError(t, forceCreateTag(context.Background(), store, image, targetRef))
		assert.Equal(t, image.Target, store.records[targetRef].Target)
		assert.Equal(t, 1, store.updates)
		assert.Equal(t, 0, store.deletes, "the existing record must be replaced, not deleted and recreated")
	})

	t.Run("tagging the same name twice is idempotent", func(t *testing.T) {
		store := newFakeImageStore()
		image := newImage("a")

		require.NoError(t, forceCreateTag(context.Background(), store, image, targetRef))
		require.NoError(t, forceCreateTag(context.Background(), store, image, targetRef))
		assert.Equal(t, image.Target, store.records[targetRef].Target)
	})

	t.Run("recovers when another writer creates the record first", func(t *testing.T) {
		store := newFakeImageStore()
		// Another writer creates the record just before our Create.
		store.onCall = func(f *fakeImageStore, op string, call int) {
			if call == 1 {
				f.records[targetRef] = newImage("other")
			}
		}
		image := newImage("ours")

		require.NoError(t, forceCreateTag(context.Background(), store, image, targetRef))
		assert.Equal(t, image.Target, store.records[targetRef].Target)
	})

	t.Run("recovers when another writer deletes the record mid-flight", func(t *testing.T) {
		store := newFakeImageStore()
		store.records[targetRef] = newImage("stale")
		// Another writer deletes the record after our Create fails but before our Update.
		store.onCall = func(f *fakeImageStore, op string, call int) {
			if call == 2 {
				delete(f.records, targetRef)
			}
		}
		image := newImage("ours")

		require.NoError(t, forceCreateTag(context.Background(), store, image, targetRef))
		assert.Equal(t, image.Target, store.records[targetRef].Target)
	})

	t.Run("gives up after repeated interference", func(t *testing.T) {
		store := newFakeImageStore()
		store.records[targetRef] = newImage("stale")
		// Another writer defeats every attempt: the record is always present when we
		// try to create it, and always gone by the time we try to update it.
		store.onCall = func(f *fakeImageStore, op string, call int) {
			if op == "create" {
				f.records[targetRef] = newImage("other")
			} else {
				delete(f.records, targetRef)
			}
		}

		err := forceCreateTag(context.Background(), store, newImage("ours"), targetRef)
		assert.ErrorContains(t, err, "failed to tag image "+targetRef)
	})
}
