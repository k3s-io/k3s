package server

import (
	"context"

	"go.etcd.io/etcd/etcdserver/etcdserverpb"
)

func isCompact(txn *etcdserverpb.TxnRequest) bool {
	return len(txn.Compare) == 1 &&
		txn.Compare[0].Target == etcdserverpb.Compare_VERSION &&
		txn.Compare[0].Result == etcdserverpb.Compare_EQUAL &&
		len(txn.Success) == 1 &&
		txn.Success[0].GetRequestPut() != nil &&
		len(txn.Failure) == 1 &&
		txn.Failure[0].GetRequestRange() != nil &&
		string(txn.Compare[0].Key) == "compact_rev_key"
}

func (l *LimitedServer) compact(ctx context.Context) (*etcdserverpb.TxnResponse, error) {
	return &etcdserverpb.TxnResponse{
		Header:    &etcdserverpb.ResponseHeader{},
		Succeeded: true,
		Responses: []*etcdserverpb.ResponseOp{
			{
				Response: &etcdserverpb.ResponseOp_ResponsePut{
					ResponsePut: &etcdserverpb.PutResponse{
						Header: &etcdserverpb.ResponseHeader{},
					},
				},
			},
		},
	}, nil
}
