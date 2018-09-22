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

import pb "github.com/coreos/etcd/etcdserver/etcdserverpb"

type opType int

const (
	// A default Op has opType 0, which is invalid.
	tRange opType = iota + 1
	tPut
	tDeleteRange
	tTxn
)

var (
	noPrefixEnd = []byte{0}
)

// Op represents an Operation that kv can execute.
type Op struct {
	t           opType
	key         string
	boundingKey string

	// for range
	limit     int64
	countOnly bool

	// for range, watch
	rev int64

	// for watch, put, delete
	prevKV bool

	// for put
	val     []byte
	leaseID LeaseID

	// txn
	cmps    []Cmp
	thenOps []Op
	elseOps []Op
}

// accessors / mutators

func (op Op) IsTxn() bool              { return op.t == tTxn }
func (op Op) Txn() ([]Cmp, []Op, []Op) { return op.cmps, op.thenOps, op.elseOps }

// Rev returns the requested revision, if any.
func (op Op) Rev() int64 { return op.rev }

// IsPut returns true iff the operation is a Put.
func (op Op) IsPut() bool { return op.t == tPut }

// IsGet returns true iff the operation is a Get.
func (op Op) IsGet() bool { return op.t == tRange }

// IsDelete returns true iff the operation is a Delete.
func (op Op) IsDelete() bool { return op.t == tDeleteRange }

// IsCountOnly returns whether countOnly is set.
func (op Op) IsCountOnly() bool { return op.countOnly == true }

// ValueBytes returns the byte slice holding the Op's value, if any.
func (op Op) ValueBytes() []byte { return op.val }

// WithValueBytes sets the byte slice for the Op's value.
func (op *Op) WithValueBytes(v []byte) { op.val = v }

func (op Op) toRangeRequest() *pb.RangeRequest {
	if op.t != tRange {
		panic("op.t != tRange")
	}
	r := &pb.RangeRequest{
		Key:       []byte(op.key),
		RangeEnd:  []byte(op.boundingKey),
		Limit:     op.limit,
		Revision:  op.rev,
		CountOnly: op.countOnly,
	}
	return r
}

func OpGet(key string, opts ...OpOption) Op {
	ret := Op{t: tRange, key: key}
	ret.applyOpts(opts)
	return ret
}

func OpDelete(key string, opts ...OpOption) Op {
	ret := Op{t: tDeleteRange, key: key}
	ret.applyOpts(opts)
	switch {
	case ret.leaseID != 0:
		panic("unexpected lease in delete")
	case ret.limit != 0:
		panic("unexpected limit in delete")
	case ret.rev != 0:
		panic("unexpected revision in delete")
	case ret.countOnly:
		panic("unexpected countOnly in delete")
	}
	return ret
}

func OpPut(key, val string, opts ...OpOption) Op {
	ret := Op{t: tPut, key: key, val: []byte(val)}
	ret.applyOpts(opts)
	switch {
	case len(ret.key) > 0 && ret.key[len(ret.key)-1] == '%':
		panic("unexpected range in put")
	case ret.limit != 0:
		panic("unexpected limit in put")
	case ret.rev != 0:
		panic("unexpected revision in put")
	case ret.countOnly:
		panic("unexpected countOnly in put")
	}
	return ret
}

func (op *Op) applyOpts(opts []OpOption) {
	for _, opt := range opts {
		opt(op)
	}
}

// OpOption configures Operations like Get, Put, Delete.
type OpOption func(*Op)

// WithLease attaches a lease ID to a key in 'Put' request.
func WithLease(leaseID LeaseID) OpOption {
	return func(op *Op) { op.leaseID = leaseID }
}

// WithLimit limits the number of results to return from 'Get' request.
// If WithLimit is given a 0 limit, it is treated as no limit.
func WithLimit(n int64) OpOption { return func(op *Op) { op.limit = n } }

// WithRev specifies the store revision for 'Get' request.
// Or the start revision of 'Watch' request.
func WithRev(rev int64) OpOption { return func(op *Op) { op.rev = rev } }

// GetPrefixRangeEnd gets the range end of the prefix.
// 'Get(foo, WithPrefix())' is equal to 'Get(foo, WithRange(GetPrefixRangeEnd(foo))'.
func GetPrefixRangeEnd(prefix string) string {
	return prefix + "%"
}

// WithPrefix enables 'Get', 'Delete', or 'Watch' requests to operate
// on the keys with matching prefix. For example, 'Get(foo, WithPrefix())'
// can return 'foo1', 'foo2', and so on.
func WithPrefix() OpOption {
	return func(op *Op) {
		op.key += "%"
	}
}

// WithRange specifies the range of 'Get', 'Delete', 'Watch' requests.
// For example, 'Get' requests with 'WithRange(end)' returns
// the keys in the range [key, end).
// endKey must be lexicographically greater than start key.
func WithRange(endKey string) OpOption {
	return func(op *Op) { op.boundingKey = endKey }
}

// WithSerializable makes 'Get' request serializable. By default,
// it's linearizable. Serializable requests are better for lower latency
// requirement.
func WithSerializable() OpOption {
	return func(op *Op) {}
}

// WithCountOnly makes the 'Get' request return only the count of keys.
func WithCountOnly() OpOption {
	return func(op *Op) { op.countOnly = true }
}

// WithPrevKV gets the previous key-value pair before the event happens. If the previous KV is already compacted,
// nothing will be returned.
func WithPrevKV() OpOption {
	return func(op *Op) {
		op.prevKV = true
	}
}
