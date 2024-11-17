package cluster

import (
	"context"
	"sync"

	"github.com/k3s-io/k3s/pkg/util"
	controllerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func registerAddressHandlers(ctx context.Context, c *Cluster) {
	nodes := c.config.Runtime.Core.Core().V1().Node()
	a := &addressesHandler{
		nodeController: nodes,
		allowed:        sets.New(c.config.SANs...),
	}

	logrus.Infof("Starting dynamiclistener CN filter node controller with SANs: %v", c.config.SANs)
	nodes.OnChange(ctx, "server-cn-filter", a.sync)
	c.cnFilterFunc = a.filterCN
}

type addressesHandler struct {
	sync.RWMutex

	nodeController controllerv1.NodeController
	allowed        sets.Set[string]
}

// filterCN filters a list of potential server CNs (hostnames or IPs), removing any which do not correspond to
// valid cluster servers (control-plane or etcd), or an address explicitly added via the tls-san option.
func (a *addressesHandler) filterCN(cns ...string) []string {
	if len(cns) == 0 || !a.nodeController.Informer().HasSynced() {
		return cns
	}

	a.RLock()
	defer a.RUnlock()

	return a.allowed.Intersection(sets.New(cns...)).UnsortedList()
}

// sync updates the allowed address list to include addresses for the node
func (a *addressesHandler) sync(key string, node *v1.Node) (*v1.Node, error) {
	if node != nil && (node.Labels[util.ControlPlaneRoleLabelKey] != "" || node.Labels[util.ETCDRoleLabelKey] != "") {
		a.Lock()
		defer a.Unlock()

		for _, address := range node.Status.Addresses {
			a.allowed.Insert(address.String())
		}
	}
	return node, nil
}
