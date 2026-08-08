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
	deletes int
	// onCall is invoked before each store operation with the operation name and the
	// 1-based call count, and may mutate records to simulate a concurrent writer.
	onCall func(f *fakeImageStore, op string, call int)
	calls  int
	// createErr is returned by the createErrOnCall'th Create.
	createErr       error
	createErrOnCall int
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
	if f.createErr != nil && f.creates == f.createErrOnCall {
		return images.Image{}, f.createErr
	}
	if _, ok := f.records[image.Name]; ok {
		return images.Image{}, errdefs.ErrAlreadyExists
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
		assert.Equal(t, 0, store.deletes)
	})

	t.Run("replaces an existing record", func(t *testing.T) {
		store := newFakeImageStore()
		store.records[targetRef] = newImage("stale")
		image := newImage("fresh")

		require.NoError(t, forceCreateTag(context.Background(), store, image, targetRef))
		assert.Equal(t, image.Target, store.records[targetRef].Target)
		assert.Equal(t, 2, store.creates)
		assert.Equal(t, 1, store.deletes)
	})

	t.Run("tagging the same name twice is idempotent", func(t *testing.T) {
		store := newFakeImageStore()
		image := newImage("a")

		require.NoError(t, forceCreateTag(context.Background(), store, image, targetRef))
		require.NoError(t, forceCreateTag(context.Background(), store, image, targetRef))
		assert.Equal(t, image.Target, store.records[targetRef].Target)
	})

	t.Run("tolerates the record being recreated after we delete it", func(t *testing.T) {
		store := newFakeImageStore()
		store.records[targetRef] = newImage("stale")
		// Another writer takes the name back between our delete and our second create.
		// The import must not fail over a tag that we no longer own.
		store.onCall = func(f *fakeImageStore, op string, call int) {
			if op == "create" && call == 3 {
				f.records[targetRef] = newImage("other")
			}
		}

		assert.NoError(t, forceCreateTag(context.Background(), store, newImage("ours"), targetRef))
		assert.Equal(t, newImage("other").Target, store.records[targetRef].Target, "the other writer's record must be left alone")
	})

	t.Run("returns errors from the first create", func(t *testing.T) {
		store := newFakeImageStore()
		store.createErr = errdefs.ErrInvalidArgument
		store.createErrOnCall = 1

		err := forceCreateTag(context.Background(), store, newImage("ours"), targetRef)
		assert.ErrorContains(t, err, "failed to tag image "+targetRef)
	})

	t.Run("returns errors from the delete", func(t *testing.T) {
		store := newFakeImageStore()
		store.records[targetRef] = newImage("stale")
		// The record is already gone by the time we try to delete it.
		store.onCall = func(f *fakeImageStore, op string, call int) {
			if op == "delete" {
				delete(f.records, targetRef)
			}
		}

		err := forceCreateTag(context.Background(), store, newImage("ours"), targetRef)
		assert.ErrorContains(t, err, "failed to delete existing image "+targetRef)
	})

	t.Run("returns non-AlreadyExists errors from the second create", func(t *testing.T) {
		store := newFakeImageStore()
		store.records[targetRef] = newImage("stale")
		store.createErr = errdefs.ErrInvalidArgument
		store.createErrOnCall = 2

		err := forceCreateTag(context.Background(), store, newImage("ours"), targetRef)
		assert.ErrorContains(t, err, "failed to tag after deleting existing image "+targetRef)
	})
}
