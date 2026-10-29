package spegel

import (
	"context"
	"errors"
	"io"

	"github.com/opencontainers/go-digest"
	"github.com/spegel-org/spegel/pkg/oci/containerd"
	"github.com/spegel-org/spegel/pkg/store"
)

var errStoreNotStarted = errors.New("deferred OCI Store not started")

// DeferredStore allows for delayed connection
// to the backend. Functions are expected to return errors until the store
// has been started.
type DeferredStore interface {
	store.Provider
	store.Watcher
	io.Closer
	Start() error
}

// explicit interface check
var _ DeferredStore = &deferredStore{}

type deferredStore struct {
	store  *containerd.Containerd
	create func() (*containerd.Containerd, error)
}

func (ds *deferredStore) Name() string {
	if ds.store == nil {
		return "deferred"
	}
	return ds.store.Name()
}

func (ds *deferredStore) Resolve(ctx context.Context, ref string) (digest.Digest, error) {
	if ds.store == nil {
		return "", errStoreNotStarted
	}
	return ds.store.Resolve(ctx, ref)
}

func (ds *deferredStore) Descriptor(ctx context.Context, dgst digest.Digest) (store.Descriptor, error) {
	if ds.store == nil {
		return store.Descriptor{}, errStoreNotStarted
	}
	return ds.store.Descriptor(ctx, dgst)
}

func (ds *deferredStore) Open(ctx context.Context, dgst digest.Digest) (io.ReadSeekCloser, error) {
	if ds.store == nil {
		return nil, errStoreNotStarted
	}
	return ds.store.Open(ctx, dgst)
}

func (ds *deferredStore) Watch(ctx context.Context) ([]store.Event, <-chan store.Event, error) {
	if ds.store == nil {
		return nil, nil, errStoreNotStarted
	}
	return ds.store.Watch(ctx)
}

func (ds *deferredStore) Close() error {
	if ds.store != nil {
		store := ds.store
		ds.store = nil
		return store.Close()
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
func NewDeferredContainerd(ctx context.Context, sock, namespace string, opts ...containerd.ContainerdOption) (DeferredStore, error) {
	return &deferredStore{
		create: func() (*containerd.Containerd, error) {
			return containerd.NewContainerd(ctx, sock, namespace, opts...)
		},
	}, nil
}
