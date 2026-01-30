package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/k3s-io/kine/pkg/endpoint"
	"github.com/otiai10/copy"
	pkgerrors "github.com/pkg/errors"
	"github.com/rancher/wrangler/v3/pkg/merr"
	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/client/pkg/v3/logutil"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/config"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/server/v3/etcdserver/cindex"
	etcderrors "go.etcd.io/etcd/server/v3/etcdserver/errors"
	"go.etcd.io/etcd/server/v3/lease"
	"go.etcd.io/etcd/server/v3/storage"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
	"go.etcd.io/etcd/server/v3/storage/schema"
	"go.etcd.io/etcd/server/v3/storage/wal"
	"go.uber.org/zap/zapcore"
)

// ReadCloser is a generic wrapper around a MVCC store that provides only read/close functions
type ReadCloser interface {
	List(ctx context.Context, key string, rev int64) ([]mvccpb.KeyValue, error)
	Get(ctx context.Context, key string) (mvccpb.KeyValue, error)
	Close() error
}

type ReadWriteCloser interface {
	ReadCloser
	Create(ctx context.Context, key string, value []byte) error
	Update(ctx context.Context, key string, revision int64, value []byte) error
	Delete(ctx context.Context, key string, revision int64) error
}

// explicit interface check
var _ ReadWriteCloser = &RemoteStore{}

// RemoteStore is a wrapper around a remote etcd datastore.
// This is much like kine/pkg/client.Client but it uses the native
// mvccpb types for interop with the raw MVCC stores
type RemoteStore struct {
	client *clientv3.Client
}

func NewRemoteStore(config endpoint.ETCDConfig) (*RemoteStore, error) {
	tlsConfig, err := config.TLSConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	logger, err := logutil.CreateDefaultZapLogger(zapcore.InfoLevel)
	if err != nil {
		return nil, err
	}
	c, err := clientv3.New(clientv3.Config{
		Endpoints:   config.Endpoints,
		DialTimeout: 5 * time.Second,
		Logger:      logger,
		TLS:         tlsConfig,
	})
	if err != nil {
		return nil, err
	}

	return &RemoteStore{client: c}, nil
}

func (r *RemoteStore) List(ctx context.Context, key string, rev int64) ([]mvccpb.KeyValue, error) {
	resp, err := r.client.Get(ctx, key, clientv3.WithPrefix(), clientv3.WithRev(rev))
	if err != nil {
		return nil, err
	}
	vals := make([]mvccpb.KeyValue, len(resp.Kvs))
	for i := range resp.Kvs {
		vals[i] = *resp.Kvs[i]
	}
	return vals, nil
}

func (r *RemoteStore) Get(ctx context.Context, key string) (mvccpb.KeyValue, error) {
	resp, err := r.client.Get(ctx, key)
	if err != nil {
		return mvccpb.KeyValue{}, err
	}
	if len(resp.Kvs) == 1 {
		return *resp.Kvs[0], nil
	}
	return mvccpb.KeyValue{}, etcderrors.ErrKeyNotFound
}

func (r *RemoteStore) Close() error {
	return r.client.Close()
}

func (r *RemoteStore) Create(ctx context.Context, key string, value []byte) error {
	resp, err := r.client.Txn(ctx).
		If(clientv3.Compare(clientv3.ModRevision(key), "=", 0)).
		Then(clientv3.OpPut(key, string(value))).
		Commit()
	if err != nil {
		return err
	}
	if !resp.Succeeded {
		return errors.New("key exists")
	}
	return nil
}

func (r *RemoteStore) Update(ctx context.Context, key string, revision int64, value []byte) error {
	resp, err := r.client.Txn(ctx).
		If(clientv3.Compare(clientv3.ModRevision(key), "=", revision)).
		Then(clientv3.OpPut(key, string(value))).
		Else(clientv3.OpGet(key)).
		Commit()
	if err != nil {
		return err
	}
	if !resp.Succeeded {
		return fmt.Errorf("revision %d doesnt match", revision)
	}
	return nil
}

func (r *RemoteStore) Delete(ctx context.Context, key string, revision int64) error {
	resp, err := r.client.Txn(ctx).
		If(clientv3.Compare(clientv3.ModRevision(key), "=", revision)).
		Then(clientv3.OpDelete(key)).
		Else(clientv3.OpGet(key)).
		Commit()
	if err != nil {
		return err
	}
	if !resp.Succeeded {
		return fmt.Errorf("revision %d doesnt match", revision)
	}
	return nil
}

// explicit interface check
var _ ReadCloser = &TemporaryStore{}

// TemporaryStore is a wrapper around Store. A temporary copy of the specified
// etcd database files is created when the store is opened, and the files
// are deleted when it is closed.
type TemporaryStore struct {
	store   *Store
	dataDir string
}

func NewTemporaryStore(dataDir string) (*TemporaryStore, error) {
	tempDir := dataDir + "-tmp"
	if err := os.RemoveAll(tempDir); err != nil {
		return nil, err
	}

	if err := copy.Copy(dataDir, tempDir, copy.Options{PreserveOwner: true}); err != nil {
		return nil, err
	}

	s, err := NewStore(tempDir)
	if err != nil {
		return nil, err
	}

	return &TemporaryStore{store: s, dataDir: tempDir}, nil
}

func (t *TemporaryStore) List(ctx context.Context, key string, rev int64) ([]mvccpb.KeyValue, error) {
	return t.store.List(ctx, key, rev)
}

func (t *TemporaryStore) Get(ctx context.Context, key string) (mvccpb.KeyValue, error) {
	return t.store.Get(ctx, key)
}

func (t *TemporaryStore) Close() error {
	return merr.NewErrors(t.store.Close(), os.RemoveAll(t.dataDir))
}

// explicit interface check
var _ ReadCloser = &Store{}

// Store is a wrapper around an etcd MVCC store. It provides many of the same interfaces as
// a running etcd server, but without any of the raft clustering bits, by directly opening
// the bbolt database.
type Store struct {
	kv mvcc.KV
}

func NewStore(dataDir string) (*Store, error) {
	var currentIndex, latestIndex uint64
	logger, err := logutil.CreateDefaultZapLogger(zapcore.InfoLevel)
	if err != nil {
		return nil, err
	}

	cfg := config.ServerConfig{Logger: logger, DataDir: dataDir}
	path := cfg.BackendPath()

	// need to check for backend path ourselves, as backend.New just logs a panic
	// via zap if it doesn't exist, which isn't fatal.
	if _, err := os.Stat(path); err != nil {
		return nil, pkgerrors.WithMessage(err, "failed to stat MVCC KV store backend path")
	}

	logrus.Infof("Opening etcd MVCC KV store at %s", path)

	// open backend database
	bcfg := backend.DefaultBackendConfig(logger)
	bcfg.Path = path
	bcfg.UnsafeNoFsync = true
	bcfg.BatchInterval = 0
	bcfg.BatchLimit = 0
	be := backend.New(bcfg)

	// get current index from backend
	currentIndex, _ = schema.ReadConsistentIndex(be.ReadTx())

	// list snapshots from WAL dir
	walSnaps, err := wal.ValidSnapshotEntries(cfg.Logger, cfg.WALDir())
	if err != nil {
		return nil, err
	}

	// find latest available snapshot index
	ss := snap.New(logger, cfg.SnapDir())
	snapshot, err := ss.LoadNewestAvailable(walSnaps)
	if err != nil && !errors.Is(err, snap.ErrNoSnapshot) {
		return nil, err
	}
	if snapshot != nil {
		latestIndex = snapshot.Metadata.Index
	}

	// restore from snapshot if available
	if latestIndex > currentIndex {
		logrus.Warnf("MVCC database index %d is less than latest snapshot index %d", currentIndex, latestIndex)
		path, err := ss.DBFilePath(latestIndex)
		if err != nil {
			logrus.Warnf("MVCC database for snapshot index %d not available; data may be stale", latestIndex)
		} else {
			logrus.Infof("MVCC database restoring snapshot index %d from %s", latestIndex, path)
			be, err = storage.RecoverSnapshotBackend(cfg, be, *snapshot, true, storage.NewBackendHooks(cfg.Logger, cindex.NewConsistentIndex(nil)))
			if err != nil {
				be.Close()
				return nil, err
			}
		}
	}

	return &Store{kv: mvcc.NewStore(cfg.Logger, be, &lease.FakeLessor{}, mvcc.StoreConfig{})}, nil
}

func (s *Store) Close() error {
	logrus.Info("Closing etcd MVCC KV store")
	return s.kv.Close()
}

func (s *Store) List(ctx context.Context, key string, rev int64) ([]mvccpb.KeyValue, error) {
	resp, err := s.kv.Range(ctx, []byte(key), []byte(key+"\xff"), mvcc.RangeOptions{Rev: rev})
	if err != nil {
		return nil, err
	}
	return resp.KVs, nil
}

func (s *Store) Get(ctx context.Context, key string) (mvccpb.KeyValue, error) {
	resp, err := s.kv.Range(ctx, []byte(key), nil, mvcc.RangeOptions{})
	if err != nil {
		return mvccpb.KeyValue{}, err
	}

	if len(resp.KVs) == 1 {
		return resp.KVs[0], nil
	}

	return mvccpb.KeyValue{}, etcderrors.ErrKeyNotFound
}
