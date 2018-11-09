package driver

import (
	"context"
)

type Driver interface {
	List(ctx context.Context, revision, limit int64, rangeKey, startKey string) (kvs []*KeyValue, listRevision int64, err error)

	Delete(ctx context.Context, key string, revision int64) ([]*KeyValue, error)

	// Update should return ErrNotExist when the key does not exist and ErrRevisionMatch when revision doesn't match
	Update(ctx context.Context, key string, value []byte, revision, ttl int64) (oldKv *KeyValue, newKv *KeyValue, err error)

	Watch(ctx context.Context, key string, revision int64) <-chan Event

	Close() error
}
