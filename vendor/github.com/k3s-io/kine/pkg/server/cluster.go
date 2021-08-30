package server

import (
	"context"
	"fmt"
	"strings"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"google.golang.org/grpc/metadata"
)

// explicit interface check
var _ etcdserverpb.ClusterServer = (*KVServerBridge)(nil)

func (s *KVServerBridge) MemberAdd(context.Context, *etcdserverpb.MemberAddRequest) (*etcdserverpb.MemberAddResponse, error) {
	return nil, fmt.Errorf("member add is not supported")
}

func (s *KVServerBridge) MemberRemove(context.Context, *etcdserverpb.MemberRemoveRequest) (*etcdserverpb.MemberRemoveResponse, error) {
	return nil, fmt.Errorf("member remove is not supported")
}

func (s *KVServerBridge) MemberUpdate(context.Context, *etcdserverpb.MemberUpdateRequest) (*etcdserverpb.MemberUpdateResponse, error) {
	return nil, fmt.Errorf("member update is not supported")
}

func (s *KVServerBridge) MemberList(ctx context.Context, r *etcdserverpb.MemberListRequest) (*etcdserverpb.MemberListResponse, error) {
	listenURL := authorityURL(ctx, s.limited.scheme)
	return &etcdserverpb.MemberListResponse{
		Header: &etcdserverpb.ResponseHeader{},
		Members: []*etcdserverpb.Member{
			{
				Name:       "kine",
				ClientURLs: []string{listenURL},
				PeerURLs:   []string{listenURL},
			},
		},
	}, nil
}

func (s *KVServerBridge) MemberPromote(context.Context, *etcdserverpb.MemberPromoteRequest) (*etcdserverpb.MemberPromoteResponse, error) {
	return nil, fmt.Errorf("member promote is not supported")
}

// authorityURL returns the URL of the authority (host) that the client connected to.
// If no scheme is included in the authority data, the provided scheme is used. If no
// authority data is provided, the default etcd endpoint is used.
func authorityURL(ctx context.Context, scheme string) string {
	authority := "127.0.0.1:2379"
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		authList := md.Get(":authority")
		if len(authList) > 0 {
			authority = authList[0]
			// etcd v3.5 encodes the endpoint address list as "#initially=[ADDRESS1;ADDRESS2]"
			if strings.HasPrefix(authority, "#initially=[") {
				authority = strings.TrimPrefix(authority, "#initially=[")
				authority = strings.TrimSuffix(authority, "]")
				authority = strings.ReplaceAll(authority, ";", ",")
				return authority
			}
		}
	}
	return scheme + "://" + authority
}
