// +build !no_etcd

package cluster

import (
	"github.com/rancher/k3s/pkg/cluster/managed"
	"github.com/rancher/k3s/pkg/etcd"
)

func init() {
	managed.RegisterDriver(&etcd.ETCD{})
}
