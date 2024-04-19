package etcd

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/k3s-io/k3s/pkg/agent/loadbalancer"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
)

type Proxy interface {
	Update(addresses []string)
	ETCDURL() string
	ETCDAddresses() []string
	ETCDServerURL() string
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
	u, err := url.Parse(etcdURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse etcd client URL")
	}

	e := &etcdproxy{
		dataDir:        dataDir,
		initialETCDURL: etcdURL,
		etcdURL:        etcdURL,
		supervisorPort: supervisorPort,
		disconnect:     map[string]context.CancelFunc{},
	}

	lb, err := loadbalancer.New(ctx, dataDir, loadbalancer.ETCDServerServiceName, etcdURL, 2379, isIPv6)
	if err != nil {
		return nil, err
	}
	e.etcdLB = lb
	e.etcdLBURL = lb.LoadBalancerServerURL()

	e.fallbackETCDAddress = u.Host
	e.etcdPort = u.Port()

	return e, nil
}

type etcdproxy struct {
	dataDir   string
	etcdLBURL string

	supervisorPort      int
	initialETCDURL      string
	etcdURL             string
	etcdPort            string
	fallbackETCDAddress string
	etcdAddresses       []string
	etcdLB              *loadbalancer.LoadBalancer
	disconnect          map[string]context.CancelFunc
}

func (e *etcdproxy) Update(addresses []string) {
	e.etcdLB.Update(addresses)

	validEndpoint := map[string]bool{}
	for _, address := range e.etcdLB.ServerAddresses {
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

// start a polling routine that makes periodic requests to the etcd node's supervisor port.
// If the request fails, the node is marked unhealthy.
func (e etcdproxy) createHealthCheck(ctx context.Context, address string) func() bool {
	// Assume that the connection to the server will succeed, to avoid failing health checks while attempting to connect.
	// If we cannot connect, connected will be set to false when the initial connection attempt fails.
	connected := true

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
			logrus.Debugf("Health check %s failed: %v (StatusCode: %d)", url, err, statusCode)
			connected = false
		} else {
			connected = true
		}
	}, 5*time.Second, 1.0, true)

	return func() bool {
		return connected
	}
}
