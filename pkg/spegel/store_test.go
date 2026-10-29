package spegel

import (
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/spegel-org/spegel/pkg/oci"
	"github.com/spegel-org/spegel/pkg/oci/containerd"
	"github.com/spegel-org/spegel/pkg/store"
	"github.com/stretchr/testify/require"
)

func Test_UnitDeferredStoreNotStarted(t *testing.T) {
	ds := &deferredStore{}
	ctx := t.Context()
	dgst := digest.FromString("missing")

	require.Equal(t, "deferred", ds.Name())
	_, err := ds.Resolve(ctx, "docker.io/library/busybox:1.37.0")
	require.ErrorIs(t, err, errStoreNotStarted)
	_, err = ds.Descriptor(ctx, dgst)
	require.ErrorIs(t, err, errStoreNotStarted)
	_, err = ds.Open(ctx, dgst)
	require.ErrorIs(t, err, errStoreNotStarted)
	initial, events, err := ds.Watch(ctx)
	require.ErrorIs(t, err, errStoreNotStarted)
	require.Nil(t, initial)
	require.Nil(t, events)
	require.NoError(t, ds.Close())
}

func Test_UnitContainerdContentAdvertisement(t *testing.T) {
	img, err := oci.ParseImage("docker.io/library/busybox:1.37.0", oci.WithDigest(digest.FromString("manifest")))
	require.NoError(t, err)
	idx := containerd.NewIndex()
	layer := digest.FromString("layer")

	// Image metadata must not advertise digests before their content is committed.
	require.Equal(t, []store.Event{{Type: store.CreateEvent, Reference: "docker.io/library/busybox:1.37.0"}}, idx.AddImage(img))
	require.Empty(t, idx.DiffContent(nil))

	require.Equal(t, []store.Event{{Type: store.CreateEvent, Digest: layer}}, idx.AddContent(layer))
	require.Empty(t, idx.AddContent(layer))
	require.Equal(t, []store.Event{{Type: store.DeleteEvent, Digest: layer}}, idx.DiffContent(nil))
}
