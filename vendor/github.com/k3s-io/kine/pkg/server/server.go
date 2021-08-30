package server

import (
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type KVServerBridge struct {
	limited *LimitedServer
}

func New(backend Backend, scheme string) *KVServerBridge {
	return &KVServerBridge{
		limited: &LimitedServer{
			backend: backend,
			scheme:  scheme,
		},
	}
}

func (k *KVServerBridge) Register(server *grpc.Server) {
	etcdserverpb.RegisterLeaseServer(server, k)
	etcdserverpb.RegisterWatchServer(server, k)
	etcdserverpb.RegisterKVServer(server, k)
	etcdserverpb.RegisterClusterServer(server, k)
	etcdserverpb.RegisterMaintenanceServer(server, k)

	hsrv := health.NewServer()
	hsrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(server, hsrv)
}
