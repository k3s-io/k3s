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
	"fmt"
	"sync"
	"time"

	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"golang.org/x/net/context"
	utiltrace "k8s.io/apiserver/pkg/util/trace"
)

type Txn interface {
	// If takes a list of comparison. If all comparisons passed in succeed,
	// the operations passed into Then() will be executed. Or the operations
	// passed into Else() will be executed.
	If(cs ...Cmp) Txn

	// Then takes a list of operations. The Ops list will be executed, if the
	// comparisons passed in If() succeed.
	Then(ops ...Op) Txn

	// Else takes a list of operations. The Ops list will be executed, if the
	// comparisons passed in If() fail.
	Else(ops ...Op) Txn

	// Commit tries to commit the transaction.
	Commit() (*TxnResponse, error)
}

type txn struct {
	kv  *kv
	ctx context.Context

	mu    sync.Mutex
	cif   bool
	cthen bool
	celse bool

	cmps []*pb.Compare

	sus []Op
	fas []Op
}

func (txn *txn) If(cs ...Cmp) Txn {
	txn.mu.Lock()
	defer txn.mu.Unlock()

	if txn.cif {
		panic("cannot call If twice!")
	}

	if txn.cthen {
		panic("cannot call If after Then!")
	}

	if txn.celse {
		panic("cannot call If after Else!")
	}

	txn.cif = true

	for i := range cs {
		txn.cmps = append(txn.cmps, (*pb.Compare)(&cs[i]))
	}

	return txn
}

func (txn *txn) Then(ops ...Op) Txn {
	txn.mu.Lock()
	defer txn.mu.Unlock()

	if txn.cthen {
		panic("cannot call Then twice!")
	}
	if txn.celse {
		panic("cannot call Then after Else!")
	}

	txn.cthen = true

	for _, op := range ops {
		txn.sus = append(txn.sus, op)
	}

	return txn
}

func (txn *txn) Else(ops ...Op) Txn {
	txn.mu.Lock()
	defer txn.mu.Unlock()

	if txn.celse {
		panic("cannot call Else twice!")
	}

	txn.celse = true

	for _, op := range ops {
		txn.fas = append(txn.fas, op)
	}

	return txn
}

func (txn *txn) do(op Op) (*pb.ResponseOp, error) {
	switch op.t {
	case tRange:
		r, err := txn.kv.opGet(txn.ctx, op)
		if err != nil {
			return nil, err
		}
		return &pb.ResponseOp{
			Response: &pb.ResponseOp_ResponseRange{
				ResponseRange: (*pb.RangeResponse)(r),
			},
		}, nil
	case tPut:
		r, err := txn.kv.opPut(txn.ctx, op)
		if err != nil {
			return nil, err
		}
		return &pb.ResponseOp{
			Response: &pb.ResponseOp_ResponsePut{
				ResponsePut: (*pb.PutResponse)(r),
			},
		}, nil
	case tDeleteRange:
		r, err := txn.kv.opDelete(txn.ctx, op)
		if err != nil {
			return nil, err
		}
		return &pb.ResponseOp{
			Response: &pb.ResponseOp_ResponseDeleteRange{
				ResponseDeleteRange: (*pb.DeleteRangeResponse)(r),
			},
		}, nil
	default:
		return nil, fmt.Errorf("unknown op in txn: %#v", op)
	}
}

func (txn *txn) Commit() (*TxnResponse, error) {
	trace := utiltrace.New("SQL Commit")
	defer trace.LogIfLong(500 * time.Millisecond)

	locks := map[string]bool{}
	resp := &TxnResponse{
		Header: &pb.ResponseHeader{},
	}

	good := true
	for _, c := range txn.cmps {
		k := string(c.Key)
		if !locks[k] {
			txn.kv.l.Lock(k)
			trace.Step(fmt.Sprintf("lock acquired: %s", k))
			locks[k] = true
			defer txn.kv.l.Unlock(k)
		}
		gr, err := txn.kv.Get(txn.ctx, k)
		if err != nil {
			return nil, err
		}

		switch c.Target {
		case pb.Compare_VERSION:
			ver := int64(0)
			if len(gr.Kvs) > 0 {
				ver = gr.Kvs[0].Version
			}
			cv, _ := c.TargetUnion.(*pb.Compare_Version)
			if ver != cv.Version {
				good = false
			}
		case pb.Compare_MOD:
			mod := int64(0)
			if len(gr.Kvs) > 0 {
				mod = gr.Kvs[0].ModRevision
			}
			cv, _ := c.TargetUnion.(*pb.Compare_ModRevision)
			if mod != cv.ModRevision {
				good = false
			}
		default:
			return nil, fmt.Errorf("unknown txn target %v", c.Target)
		}

		trace.Step(fmt.Sprintf("condition key %s good %v", k, good))
	}

	resp.Succeeded = good
	ops := txn.sus
	if !good {
		ops = txn.fas
	}

	for _, op := range ops {
		r, err := txn.do(op)
		if err != nil {
			return nil, err
		}
		resp.Responses = append(resp.Responses, r)
		trace.Step(fmt.Sprintf("op key %s op %v", op.key, op.t))
	}

	return resp, nil
}
