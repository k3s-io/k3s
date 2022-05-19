package etcd

import (
	"context"
	"net/url"

	"github.com/k3s-io/k3s/pkg/agent/loadbalancer"
	"github.com/pkg/errors"
)

type Proxy interface {
	Update(addresses []string)
	ETCDURL() string
	ETCDAddresses() []string
	ETCDServerURL() string
}

// NewETCDProxy initializes a new proxy structure that contain a load balancer
// which listens on port 2379 and proxy between etcd cluster members
func NewETCDProxy(ctx context.Context, enabled bool, dataDir, etcdURL string, isIPv6 bool) (Proxy, error) {
	u, err := url.Parse(etcdURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse etcd client URL")
	}

	e := &etcdproxy{
		dataDir:        dataDir,
		initialETCDURL: etcdURL,
		etcdURL:        etcdURL,
	}

	if enabled {
		lb, err := loadbalancer.New(ctx, dataDir, loadbalancer.ETCDServerServiceName, etcdURL, 2379, isIPv6)
		if err != nil {
			return nil, err
		}
		e.etcdLB = lb
		e.etcdLBURL = lb.LoadBalancerServerURL()
	}

	e.fallbackETCDAddress = u.Host
	e.etcdPort = u.Port()

	return e, nil
}

type etcdproxy struct {
	dataDir   string
	etcdLBURL string

	initialETCDURL      string
	etcdURL             string
	etcdPort            string
	fallbackETCDAddress string
	etcdAddresses       []string
	etcdLB              *loadbalancer.LoadBalancer
}

func (e *etcdproxy) Update(addresses []string) {
	if e.etcdLB != nil {
		e.etcdLB.Update(addresses)
	}
}

func (e *etcdproxy) ETCDURL() string {
	return e.etcdURL
}

func (e *etcdproxy) ETCDAddresses() []string {
	if len(e.etcdAddresses) > 0 {
		return e.etcdAddresses
	}
	return []string{e.fallbackETCDAddress}
}

func (e *etcdproxy) ETCDServerURL() string {
	return e.etcdURL
}
