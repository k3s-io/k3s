package etcd

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/k3s-io/k3s/pkg/agent/loadbalancer"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
)

type Proxy interface {
	Update(addresses []string)
}

var httpClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	},
}

// NewETCDProxy initializes a new proxy structure that contain a load balancer
// which listens on port 2379 and proxy between etcd cluster members
func NewETCDProxy(ctx context.Context, supervisorPort int, dataDir, etcdURL string, isIPv6 bool) (Proxy, error) {
	lb, err := loadbalancer.New(ctx, dataDir, loadbalancer.ETCDServerServiceName, etcdURL, 2379, isIPv6)
	if err != nil {
		return nil, err
	}

	return &etcdproxy{
		supervisorPort: supervisorPort,
		etcdLB:         lb,
		disconnect:     map[string]context.CancelFunc{},
	}, nil
}

type etcdproxy struct {
	supervisorPort int
	etcdLB         *loadbalancer.LoadBalancer
	disconnect     map[string]context.CancelFunc
}

func (e *etcdproxy) Update(addresses []string) {
	if e.etcdLB == nil {
		return
	}

	e.etcdLB.Update(addresses)

	validEndpoint := map[string]bool{}
	for _, address := range e.etcdLB.ServerAddresses() {
		validEndpoint[address] = true
		if _, ok := e.disconnect[address]; !ok {
			ctx, cancel := context.WithCancel(context.Background())
			e.disconnect[address] = cancel
			e.etcdLB.SetHealthCheck(address, e.createHealthCheck(ctx, address))
		}
	}

	for address, cancel := range e.disconnect {
		if !validEndpoint[address] {
			cancel()
			delete(e.disconnect, address)
		}
	}
}

// start a polling routine that makes periodic requests to the etcd node's supervisor port.
// If the request fails, the node is marked unhealthy.
func (e etcdproxy) createHealthCheck(ctx context.Context, address string) loadbalancer.HealthCheckFunc {
	var status loadbalancer.HealthCheckResult

	host, _, _ := net.SplitHostPort(address)
	url := fmt.Sprintf("https://%s/ping", net.JoinHostPort(host, strconv.Itoa(e.supervisorPort)))

	go wait.JitterUntilWithContext(ctx, func(ctx context.Context) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := httpClient.Do(req)
		var statusCode int
		if resp != nil {
			statusCode = resp.StatusCode
		}
		if err != nil || statusCode != http.StatusOK {
			logrus.Debugf("Health check %s failed: %v (StatusCode: %d)", address, err, statusCode)
			status = loadbalancer.HealthCheckResultFailed
		} else {
			status = loadbalancer.HealthCheckResultOK
		}
	}, 5*time.Second, 1.0, true)

	return func() loadbalancer.HealthCheckResult {
		// Reset the status to unknown on reading, until next time it is checked.
		// This avoids having a health check result alter the server state between active checks.
		s := status
		status = loadbalancer.HealthCheckResultUnknown
		return s
	}
}
