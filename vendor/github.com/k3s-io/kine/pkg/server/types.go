package server

import (
	"context"

	"go.etcd.io/etcd/etcdserver/api/v3rpc/rpctypes"
)

var (
	ErrKeyExists = rpctypes.ErrGRPCDuplicateKey
	ErrCompacted = rpctypes.ErrGRPCCompacted
)

type Backend interface {
	Start(ctx context.Context) error
	Get(ctx context.Context, key string, revision int64) (int64, *KeyValue, error)
	Create(ctx context.Context, key string, value []byte, lease int64) (int64, error)
	Delete(ctx context.Context, key string, revision int64) (int64, *KeyValue, bool, error)
	List(ctx context.Context, prefix, startKey string, limit, revision int64) (int64, []*KeyValue, error)
	Count(ctx context.Context, prefix string) (int64, int64, error)
	Update(ctx context.Context, key string, value []byte, revision, lease int64) (int64, *KeyValue, bool, error)
	Watch(ctx context.Context, key string, revision int64) <-chan []*Event
}

type KeyValue struct {
	Key            string
	CreateRevision int64
	ModRevision    int64
	Value          []byte
	Lease          int64
}

type Event struct {
	Delete bool
	Create bool
	KV     *KeyValue
	PrevKV *KeyValue
}
