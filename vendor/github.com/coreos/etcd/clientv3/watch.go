// Copyright 2016 The etcd Authors
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
	v3rpc "github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/mvcc/mvccpb"

	"golang.org/x/net/context"
)

const (
	EventTypeDelete = mvccpb.DELETE
	EventTypePut    = mvccpb.PUT
)

type Event mvccpb.Event

type WatchChan <-chan WatchResponse

type Watcher interface {
	// Watch watches on a key or prefix. The watched events will be returned
	// through the returned channel. If revisions waiting to be sent over the
	// watch are compacted, then the watch will be canceled by the server, the
	// client will post a compacted error watch response, and the channel will close.
	Watch(ctx context.Context, key string, opts ...OpOption) WatchChan

	// Close closes the watcher and cancels all watch requests.
	Close() error
}

func (k *kv) Watch(ctx context.Context, key string, opts ...OpOption) WatchChan {
	op := OpGet(key, opts...)
	c := k.d.Watch(ctx, op.key, op.rev)

	result := make(chan WatchResponse)
	go func() {
		defer close(result)
		for e := range c {
			if e.Err != nil {
				result <- NewWatchResponseErr(e.Err)
				continue
			} else if e.Start {
				result <- WatchResponse{
					Created: true,
				}
				continue
			}

			k := e.KV

			event := &Event{}
			if k.Del == 0 {
				event.Type = mvccpb.PUT
			} else {
				event.Type = mvccpb.DELETE
			}

			event.Kv = toKeyValue(k)
			if event.Kv.Version > 1 && k.OldRevision > 0 {
				oldKV := *event.Kv
				oldKV.ModRevision = k.OldRevision
				oldKV.Value = k.OldValue
				event.PrevKv = &oldKV
			}

			wr := WatchResponse{
				Header: pb.ResponseHeader{
					Revision: event.Kv.ModRevision,
				},
				Events: []*Event{
					event,
				},
			}
			result <- wr
		}
	}()

	return result
}

func (k *kv) Close() error {
	return k.d.Close()
}

type WatchResponse struct {
	Header pb.ResponseHeader
	Events []*Event

	// CompactRevision is the minimum revision the watcher may receive.
	CompactRevision int64

	// Canceled is used to indicate watch failure.
	// If the watch failed and the stream was about to close, before the channel is closed,
	// the channel sends a final response that has Canceled set to true with a non-nil Err().
	Canceled bool

	// Created is used to indicate the creation of the watcher.
	Created bool

	closeErr error
}

func NewWatchResponseErr(err error) WatchResponse {
	return WatchResponse{
		Canceled: true,
		closeErr: err,
	}
}

// IsCreate returns true if the event tells that the key is newly created.
func (e *Event) IsCreate() bool {
	return e.Type == EventTypePut && e.Kv.CreateRevision == e.Kv.ModRevision
}

// IsModify returns true if the event tells that a new value is put on existing key.
func (e *Event) IsModify() bool {
	return e.Type == EventTypePut && e.Kv.CreateRevision != e.Kv.ModRevision
}

// Err is the error value if this WatchResponse holds an error.
func (wr *WatchResponse) Err() error {
	switch {
	case wr.closeErr != nil:
		return v3rpc.Error(wr.closeErr)
	case wr.CompactRevision != 0:
		return v3rpc.ErrCompacted
	}
	return nil
}

// IsProgressNotify returns true if the WatchResponse is progress notification.
func (wr *WatchResponse) IsProgressNotify() bool {
	return len(wr.Events) == 0 && !wr.Canceled && !wr.Created && wr.CompactRevision == 0 && wr.Header.Revision != 0
}
