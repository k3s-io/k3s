package spegel

import (
	"context"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/k3s-io/k3s/pkg/clientaccess"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/rancher/wrangler/v3/pkg/merr"
	"github.com/sirupsen/logrus"
	"github.com/spegel-org/spegel/pkg/routing"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	nodeutil "k8s.io/kubernetes/pkg/controller/util/node"
)

// explicit interface checks
var _ routing.Bootstrapper = &selfBootstrapper{}
var _ routing.Bootstrapper = &agentBootstrapper{}
var _ routing.Bootstrapper = &serverBootstrapper{}
var _ routing.Bootstrapper = &chainingBootstrapper{}

type selfBootstrapper struct {
	id string
}

// NewSelfBootstrapper returns a stub p2p bootstrapper that just returns its own ID
func NewSelfBootstrapper() routing.Bootstrapper {
	return &selfBootstrapper{}
}

func (s *selfBootstrapper) Run(_ context.Context, id string) error {
	s.id = id
	return nil
}

func (s *selfBootstrapper) Get() (*peer.AddrInfo, error) {
	return peer.AddrInfoFromString(s.id)
}

type agentBootstrapper struct {
	server     string
	token      string
	clientCert string
	clientKey  string
}

// NewAgentBootstrapper returns a p2p bootstrapper that retrieves a peer address from its server
func NewAgentBootstrapper(server, token, dataDir string) routing.Bootstrapper {
	return &agentBootstrapper{
		clientCert: filepath.Join(dataDir, "agent", "client-kubelet.crt"),
		clientKey:  filepath.Join(dataDir, "agent", "client-kubelet.key"),
		server:     server,
		token:      token,
	}
}

func (c *agentBootstrapper) Run(_ context.Context, _ string) error {
	return nil
}

func (c *agentBootstrapper) Get() (*peer.AddrInfo, error) {
	if c.server == "" || c.token == "" {
		return nil, errors.New("cannot get addresses without server and token")
	}

	withCert := clientaccess.WithClientCertificate(c.clientCert, c.clientKey)
	info, err := clientaccess.ParseAndValidateToken(c.server, c.token, withCert)
	if err != nil {
		return nil, err
	}

	addr, err := info.Get("/v1-" + version.Program + "/p2p")
	if err != nil {
		return nil, err
	}

	addrInfo, err := peer.AddrInfoFromString(string(addr))
	return addrInfo, err
}

type serverBootstrapper struct {
	controlConfig *config.Control
}

// NewServerBootstrapper returns a p2p bootstrapper that returns an address from a random other cluster member.
func NewServerBootstrapper(controlConfig *config.Control) routing.Bootstrapper {
	return &serverBootstrapper{
		controlConfig: controlConfig,
	}
}

func (s *serverBootstrapper) Run(_ context.Context, id string) error {
	s.controlConfig.Runtime.ClusterControllerStarts["spegel-p2p"] = func(ctx context.Context) {
		nodes := s.controlConfig.Runtime.Core.Core().V1().Node()
		_ = wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (bool, error) {
			nodeName := os.Getenv("NODE_NAME")
			if nodeName == "" {
				return false, nil
			}
			node, err := nodes.Get(nodeName, metav1.GetOptions{})
			if err != nil {
				return false, nil
			}

			if node.Annotations == nil {
				node.Annotations = map[string]string{}
			}
			node.Annotations[P2pAddressAnnotation] = id
			if node.Labels == nil {
				node.Labels = map[string]string{}
			}
			node.Labels[P2pEnabledLabel] = "true"

			if _, err = nodes.Update(node); err != nil {
				return false, nil
			}
			logrus.Infof("Node P2P address annotations and labels added: %s", id)
			return true, nil
		})
	}
	return nil
}

func (s *serverBootstrapper) Get() (addrInfo *peer.AddrInfo, err error) {
	if s.controlConfig.Runtime.Core == nil {
		return nil, util.ErrCoreNotReady
	}
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		return nil, errors.New("node name not set")
	}
	nodes := s.controlConfig.Runtime.Core.Core().V1().Node()
	labelSelector := labels.Set{P2pEnabledLabel: "true"}.String()
	nodeList, err := nodes.List(metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, err
	}
	for _, i := range rand.Perm(len(nodeList.Items)) {
		node := nodeList.Items[i]
		if node.Name == nodeName {
			// don't return our own address
			continue
		}
		if find, condition := nodeutil.GetNodeCondition(&node.Status, v1.NodeReady); find == -1 || condition.Status != v1.ConditionTrue {
			// don't return the address of a not-ready node
			continue
		}
		if val, ok := node.Annotations[P2pAddressAnnotation]; ok {
			for _, addr := range strings.Split(val, ",") {
				if info, err := peer.AddrInfoFromString(addr); err == nil {
					return info, nil
				}
			}
		}
	}
	return nil, errors.New("no ready p2p peers found")
}

type chainingBootstrapper struct {
	bootstrappers []routing.Bootstrapper
}

// NewChainingBootstrapper returns a p2p bootstrapper that passes through to a list of bootstrappers.
func NewChainingBootstrapper(bootstrappers ...routing.Bootstrapper) routing.Bootstrapper {
	return &chainingBootstrapper{
		bootstrappers: bootstrappers,
	}
}

func (c *chainingBootstrapper) Run(ctx context.Context, id string) error {
	errs := merr.Errors{}
	for _, b := range c.bootstrappers {
		if err := b.Run(ctx, id); err != nil {
			errs = append(errs, err)
		}
	}
	return merr.NewErrors(errs...)
}

func (c *chainingBootstrapper) Get() (*peer.AddrInfo, error) {
	errs := merr.Errors{}
	for _, b := range c.bootstrappers {
		addr, err := b.Get()
		if err == nil {
			return addr, nil
		}
		errs = append(errs, err)
	}
	return nil, merr.NewErrors(errs...)
}
