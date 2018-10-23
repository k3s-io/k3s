// Copyright 2015 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clientv3

import (
	"bytes"
	"database/sql"
	"fmt"
	"strings"
	"sync"

	"github.com/ibuildthecloud/kvsql/clientv3/driver"
	"github.com/ibuildthecloud/kvsql/clientv3/driver/sqlite"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/docker/docker/pkg/locker"
	"golang.org/x/net/context"
)

type (
	CompactResponse pb.CompactionResponse
	PutResponse     pb.PutResponse
	GetResponse     pb.RangeResponse
	DeleteResponse  pb.DeleteRangeResponse
	TxnResponse     pb.TxnResponse
)

var (
	connections map[string]*kv
	connectionsCtx context.Context
	CloseDB func()
	connectionsLock sync.Mutex
)

type KV interface {
	// Put puts a key-value pair into etcd.
	// Note that key,value can be plain bytes array and string is
	// an immutable representation of that bytes array.
	// To get a string of bytes, do string([]byte{0x10, 0x20}).
	Put(ctx context.Context, key, val string, opts ...OpOption) (*PutResponse, error)

	// Get retrieves keys.
	// By default, Get will return the value for "key", if any.
	// When passed WithRange(end), Get will return the keys in the range [key, end).
	// When passed WithFromKey(), Get returns keys greater than or equal to key.
	// When passed WithRev(rev) with rev > 0, Get retrieves keys at the given revision;
	// if the required revision is compacted, the request will fail with ErrCompacted .
	// When passed WithLimit(limit), the number of returned keys is bounded by limit.
	// When passed WithSort(), the keys will be sorted.
	Get(ctx context.Context, key string, opts ...OpOption) (*GetResponse, error)

	// Delete deletes a key, or optionally using WithRange(end), [key, end).
	Delete(ctx context.Context, key string, opts ...OpOption) (*DeleteResponse, error)

	// Compact compacts etcd KV history before the given rev.
	Compact(ctx context.Context, rev int64, opts ...CompactOption) (*CompactResponse, error)

	// Txn creates a transaction.
	Txn(ctx context.Context) Txn
}

type kv struct {
	l locker.Locker
	d driver.Driver
}

func newKV(cfg Config) (*kv, error) {
	connectionsLock.Lock()
	defer connectionsLock.Unlock()

	if len(cfg.Endpoints) != 1 {
		return nil, fmt.Errorf("exactly one endpoint required for DB setting, got %v", cfg.Endpoints)
	}

	key := cfg.Endpoints[0]

	if kv, ok := connections[key]; ok {
		return kv, nil
	}

	if connections == nil {
		connections = map[string]*kv{}
		connectionsCtx, CloseDB = context.WithCancel(context.Background())
	}

	parts := strings.SplitN(key, "://", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid kvsql string")
	}

	var (
		db *sql.DB
		driver *driver.Generic
		err error
	)

	switch parts[0] {
	case "sqlite":
		if db, err = sqlite.Open(parts[1]); err != nil {
			return nil, err
		}
		driver = sqlite.NewSQLite()
	}

	if err := driver.Start(context.TODO(), db); err != nil {
		db.Close()
		return nil, err
	}

	kv := &kv{
		d:driver,
	}
	connections[key] = kv

	return  kv, nil
}

func (k *kv) Put(ctx context.Context, key, val string, opts ...OpOption) (*PutResponse, error) {
	//trace := utiltrace.New(fmt.Sprintf("SQL Put key: %s", key))
	//defer trace.LogIfLong(500 * time.Millisecond)
	k.l.Lock(key)
	defer k.l.Unlock(key)

	op := OpPut(key, val, opts...)
	return k.opPut(ctx, op)
}

func (k *kv) opPut(ctx context.Context, op Op) (*PutResponse, error) {
	oldR, r, err := k.d.Update(ctx, op.key, op.val, op.rev, int64(op.leaseID))
	if err != nil {
		return nil, err
	}
	return getPutResponse(oldR, r), nil
}

func (k *kv) Get(ctx context.Context, key string, opts ...OpOption) (*GetResponse, error) {
	//trace := utiltrace.New(fmt.Sprintf("SQL Get key: %s", key))
	//defer trace.LogIfLong(500 * time.Millisecond)
	op := OpGet(key, opts...)
	return k.opGet(ctx, op)
}

func (k *kv) opGet(ctx context.Context, op Op) (*GetResponse, error) {
	var (
		rangeKey string
		startKey string
	)

	if op.boundingKey == "" {
		rangeKey = op.key
		startKey = ""
	} else {
		rangeKey = op.boundingKey
		startKey = string(bytes.SplitN([]byte(op.key), []byte{'\x00'}, -1)[0])
	}

	kvs, rev, err := k.d.List(ctx, op.rev, op.limit, rangeKey, startKey)
	if err != nil {
		return nil, err
	}

	return getResponse(kvs, rev, op.limit, op.countOnly), nil
}

func getPutResponse(oldValue *driver.KeyValue, value *driver.KeyValue) *PutResponse {
	return &PutResponse{
		Header: &pb.ResponseHeader{
			Revision: value.Revision,
		},
		PrevKv: toKeyValue(oldValue),
	}
}

func toKeyValue(v *driver.KeyValue) *mvccpb.KeyValue {
	if v == nil {
		return nil
	}

	return &mvccpb.KeyValue{
		Key:            []byte(v.Key),
		CreateRevision: v.CreateRevision,
		ModRevision:    v.Revision,
		Version:        v.Version,
		Value:          v.Value,
		Lease:          v.TTL,
	}
}

func getDeleteResponse(values []*driver.KeyValue) *DeleteResponse {
	gr := getResponse(values, 0, 0, false)
	return &DeleteResponse{
		Header: &pb.ResponseHeader{
			Revision: gr.Header.Revision,
		},
		PrevKvs: gr.Kvs,
	}
}

func getResponse(values []*driver.KeyValue, revision, limit int64, count bool) *GetResponse {
	gr := &GetResponse{
		Header: &pb.ResponseHeader{
			Revision: revision,
		},
	}

	for _, v := range values {
		kv := toKeyValue(v)
		if kv.ModRevision > gr.Header.Revision {
			gr.Header.Revision = kv.ModRevision
		}

		gr.Kvs = append(gr.Kvs, kv)
	}

	gr.Count = int64(len(gr.Kvs))
	if limit > 0 && gr.Count > limit {
		gr.Kvs = gr.Kvs[:limit]
		gr.More = true
	}

	if count {
		gr.Kvs = nil
	}

	return gr
}

func (k *kv) Delete(ctx context.Context, key string, opts ...OpOption) (*DeleteResponse, error) {
	//trace := utiltrace.New(fmt.Sprintf("SQL Delete key: %s", key))
	//defer trace.LogIfLong(500 * time.Millisecond)
	k.l.Lock(key)
	defer k.l.Unlock(key)

	op := OpDelete(key, opts...)
	return k.opDelete(ctx, op)
}

func (k *kv) opDelete(ctx context.Context, op Op) (*DeleteResponse, error) {
	r, err := k.d.Delete(ctx, op.key, op.rev)
	if err != nil {
		return nil, err
	}
	return getDeleteResponse(r), nil
}

func (k *kv) Compact(ctx context.Context, rev int64, opts ...CompactOption) (*CompactResponse, error) {
	return &CompactResponse{
		Header: &pb.ResponseHeader{},
	}, nil
}

func (k *kv) Txn(ctx context.Context) Txn {
	return &txn{
		kv:  k,
		ctx: ctx,
	}
}
