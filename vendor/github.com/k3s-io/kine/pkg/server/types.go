package server

import (
	"context"
	"database/sql"

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
	DbSize(ctx context.Context) (int64, error)
}

type Dialect interface {
	ListCurrent(ctx context.Context, prefix string, limit int64, includeDeleted bool) (*sql.Rows, error)
	List(ctx context.Context, prefix, startKey string, limit, revision int64, includeDeleted bool) (*sql.Rows, error)
	Count(ctx context.Context, prefix string) (int64, int64, error)
	CurrentRevision(ctx context.Context) (int64, error)
	After(ctx context.Context, prefix string, rev, limit int64) (*sql.Rows, error)
	Insert(ctx context.Context, key string, create, delete bool, createRevision, previousRevision int64, ttl int64, value, prevValue []byte) (int64, error)
	GetRevision(ctx context.Context, revision int64) (*sql.Rows, error)
	DeleteRevision(ctx context.Context, revision int64) error
	GetCompactRevision(ctx context.Context) (int64, error)
	SetCompactRevision(ctx context.Context, revision int64) error
	Compact(ctx context.Context, revision int64) (int64, error)
	Fill(ctx context.Context, revision int64) error
	IsFill(key string) bool
	BeginTx(ctx context.Context, opts *sql.TxOptions) (Transaction, error)
	GetSize(ctx context.Context) (int64, error)
}

type Transaction interface {
	Commit() error
	MustCommit()
	Rollback() error
	MustRollback()
	GetCompactRevision(ctx context.Context) (int64, error)
	SetCompactRevision(ctx context.Context, revision int64) error
	Compact(ctx context.Context, revision int64) (int64, error)
	GetRevision(ctx context.Context, revision int64) (*sql.Rows, error)
	DeleteRevision(ctx context.Context, revision int64) error
	CurrentRevision(ctx context.Context) (int64, error)
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
