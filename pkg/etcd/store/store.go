package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
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
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/server/v3/etcdserver/cindex"
	"go.etcd.io/etcd/server/v3/lease"
	"go.etcd.io/etcd/server/v3/mvcc"
	"go.etcd.io/etcd/server/v3/mvcc/backend"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
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
	logger = logger.Named("k3s.remotestore")

	logrus.Infof("Opening etcd client connection with endpoints %v", config.Endpoints)

	c, err := clientv3.New(clientv3.Config{
		Endpoints:   config.Endpoints,
		DialTimeout: 5 * time.Second,
		DialOptions: []grpc.DialOption{grpc.WithBlock(), grpc.FailOnNonTempDialError(true)},
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
	return mvccpb.KeyValue{}, etcdserver.ErrKeyNotFound
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

	// only copy the bbolt backend database; we don't need the WAL, legacy v2
	// store snapshots, config file, or anything else.
	// ref: https://etcd.io/docs/v3.6/learning/persistent-storage-files/#long-leaving-files
	copyOpts := copy.Options{
		PreserveOwner: true,
		PreserveTimes: true,
		NumOfWorkers:  0,
		Sync:          true,
		Skip: func(srcinfo os.FileInfo, src, dest string) (bool, error) {
			switch srcinfo.Name() {
			case "member", "snap", "db":
				return false, nil
			default:
				return true, nil
			}
		},
	}
	if err := copy.Copy(dataDir, tempDir, copyOpts); err != nil {
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
	be backend.Backend
}

func NewStore(dataDir string) (store *Store, rerr error) {
	s := &Store{}

	logger, err := logutil.CreateDefaultZapLogger(zapcore.InfoLevel)
	if err != nil {
		return nil, err
	}

	// etcd relies on panic/fatal errors to trigger process exit; we need to
	// handle it properly by recovering and returning an error.
	logger = logger.Named("k3s.store").WithOptions(
		zap.WithPanicHook(zapcore.WriteThenPanic),
		zap.WithFatalHook(zapcore.WriteThenPanic),
	)

	// recover from zap panics and ensure kv and backened are closed on error
	defer func() {
		if err := recover(); err != nil {
			msg := fmt.Sprintf("panic: %v", err)
			if logrus.IsLevelEnabled(logrus.DebugLevel) {
				msg += " at " + string(debug.Stack())
			}
			rerr = errors.New(msg)
		}
		if rerr != nil && s != nil {
			go s.Close()
		}
	}()

	cfg := config.ServerConfig{Logger: logger, DataDir: dataDir}
	path := cfg.BackendPath()

	// need to check for backend path ourselves, as backend.New just creates
	// a new empty database if the file does not exist or is empty.
	if _, err := os.Stat(path); err != nil {
		return nil, pkgerrors.WithMessage(err, "failed to stat MVCC KV store backend path")
	}

	logrus.Infof("Opening etcd MVCC KV backend database at %s", path)

	// open backend database
	bcfg := backend.DefaultBackendConfig()
	bcfg.Logger = logger
	bcfg.Path = path
	bcfg.UnsafeNoFsync = true
	bcfg.BatchInterval = time.Hour
	bcfg.BatchLimit = 100000

	// try to open the bbolt database; this may unrecoverably panic from inside
	// the bbolt freelist goroutine if the database is in an inconsistent state.
	s.be = backend.New(bcfg)
	if s.be == nil {
		return nil, errors.New("failed to open database")
	}

	// try to get current index from backend; this may fail if the bbolt database
	// was opened successfully but is in an inconsistent state.
	if currentIndex, _ := cindex.ReadConsistentIndex(s.be.ReadTx()); currentIndex == 0 {
		return nil, errors.New("failed to read consistent index")
	}

	// We do not bother checking the latest snapshot index from the WAL or attempting to
	// restore from a snapshot, as v3 store snapshots are only created when replicas are
	// lagging and the leader sends them a fresh copy of the bbolt database - and are
	// therefore highly unlikely to exist. The .snap files in the snap dir are for the
	// legacy v2 store, and are of no use.
	//
	// ref: https://etcd.io/docs/v3.6/learning/persistent-storage-files/#long-leaving-files
	// > Note: Periodic snapshots generated on each replica are only emitted in the form of
	// > *.snap file (not snap.db file). So there is no guarantee the most recent snapshot (in
	// > WAL log) has the *.snap.db file. But in such a case the backend (snap/db) is expected
	// > to be newer than the snapshot.

	s.kv = mvcc.NewStore(logger, s.be, &lease.FakeLessor{}, mvcc.StoreConfig{})
	logrus.Info("Opened etcd MVCC KV store")

	// nb: closing the kv store does not implicitly close its backend; the backend must be closed separately
	return s, nil
}

func (s *Store) Close() error {
	logrus.Info("Closing etcd MVCC KV store")
	errs := []error{}
	if s.kv != nil {
		errs = append(errs, s.kv.Close())
	}
	if s.be != nil {
		errs = append(errs, s.be.Close())
	}
	return merr.NewErrors(errs...)
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

	return mvccpb.KeyValue{}, etcdserver.ErrKeyNotFound
}
