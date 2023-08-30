package cluster

import (
	"context"
	"sync"

	"github.com/k3s-io/k3s/pkg/util"
	controllerv1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

func registerAddressHandlers(ctx context.Context, c *Cluster) {
	nodes := c.config.Runtime.Core.Core().V1().Node()
	a := &addressesHandler{
		nodeController: nodes,
		allowed:        map[string]bool{},
	}

	for _, cn := range c.config.SANs {
		a.allowed[cn] = true
	}

	logrus.Infof("Starting dynamiclistener CN filter node controller")
	nodes.OnChange(ctx, "server-cn-filter", a.sync)
	c.cnFilterFunc = a.filterCN
}

type addressesHandler struct {
	sync.RWMutex

	nodeController controllerv1.NodeController
	allowed        map[string]bool
}

// filterCN filters a list of potential server CNs (hostnames or IPs), removing any which do not correspond to
// valid cluster servers (control-plane or etcd), or an address explicitly added via the tls-san option.
func (a *addressesHandler) filterCN(cns ...string) []string {
	if !a.nodeController.Informer().HasSynced() {
		return cns
	}

	a.RLock()
	defer a.RUnlock()

	filteredCNs := make([]string, 0, len(cns))
	for _, cn := range cns {
		if a.allowed[cn] {
			filteredCNs = append(filteredCNs, cn)
		} else {
			logrus.Debugf("CN filter controller rejecting certificate CN: %s", cn)
		}
	}
	return filteredCNs
}

// sync updates the allowed address list to include addresses for the node
func (a *addressesHandler) sync(key string, node *v1.Node) (*v1.Node, error) {
	if node != nil {
		if node.Labels[util.ControlPlaneRoleLabelKey] != "" || node.Labels[util.ETCDRoleLabelKey] != "" {
			a.Lock()
			defer a.Unlock()

			for _, address := range node.Status.Addresses {
				a.allowed[address.String()] = true
			}
		}
	}
	return node, nil
}
