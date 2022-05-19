package cluster

import (
	"github.com/k3s-io/k3s/pkg/cluster/managed"
	"github.com/k3s-io/k3s/pkg/etcd"
)

func init() {
	managed.RegisterDriver(etcd.NewETCD())
}
