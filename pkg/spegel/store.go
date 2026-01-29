package spegel

import (
	"context"
	"errors"
	"io"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spegel-org/spegel/pkg/oci"
)

var errStoreNotStarted = errors.New("deferred OCI Store not started")

// DeferredStore extends `oci.Store` by allowing for delayed connection
// to the backend. Functions are expected to return errors until the store
// has been started.
type DeferredStore interface {
	oci.Store
	io.Closer
	Start() error
}

// explicit interface check
var _ DeferredStore = &deferredStore{}

type deferredStore struct {
	store  oci.Store
	create func() (oci.Store, error)
}

func (ds *deferredStore) Name() string {
	if ds.store == nil {
		return "deferred"
	}
	return ds.store.Name()
}

func (ds *deferredStore) ListImages(ctx context.Context) ([]oci.Image, error) {
	if ds.store == nil {
		return nil, errStoreNotStarted
	}
	return ds.store.ListImages(ctx)
}

func (ds *deferredStore) ListContent(ctx context.Context) ([][]oci.Reference, error) {
	if ds.store == nil {
		return nil, errStoreNotStarted
	}
	return ds.store.ListContent(ctx)
}

func (ds *deferredStore) Resolve(ctx context.Context, ref string) (digest.Digest, error) {
	if ds.store == nil {
		return "", errStoreNotStarted
	}
	return ds.store.Resolve(ctx, ref)
}

func (ds *deferredStore) Descriptor(ctx context.Context, dgst digest.Digest) (ocispec.Descriptor, error) {
	if ds.store == nil {
		return ocispec.Descriptor{}, errStoreNotStarted
	}
	return ds.store.Descriptor(ctx, dgst)
}

func (ds *deferredStore) Open(ctx context.Context, dgst digest.Digest) (io.ReadSeekCloser, error) {
	if ds.store == nil {
		return nil, errStoreNotStarted
	}
	return ds.store.Open(ctx, dgst)
}

func (ds *deferredStore) Subscribe(ctx context.Context) (<-chan oci.OCIEvent, error) {
	if ds.store == nil {
		return nil, errStoreNotStarted
	}
	return ds.store.Subscribe(ctx)
}

// Close is not part of the Store interface, but probably should be. Containerd impliments it.
func (ds *deferredStore) Close() error {
	if ds.store != nil {
		store := ds.store
		ds.store = nil
		if closer, ok := store.(io.Closer); ok {
			return closer.Close()
		}
	}
	return nil
}

// Start is called to actuall make a connection to the store backend
func (ds *deferredStore) Start() error {
	var err error
	if ds.store == nil && ds.create != nil {
		ds.store, err = ds.create()
	}
	return err
}

// NewDeferredContainerd creates a deferred store that creates a new Containerd store when Start is called.
func NewDeferredContainerd(ctx context.Context, sock, namespace string, opts ...oci.ContainerdOption) (DeferredStore, error) {
	return &deferredStore{
		create: func() (oci.Store, error) {
			return oci.NewContainerd(ctx, sock, namespace, opts...)
		},
	}, nil
}
