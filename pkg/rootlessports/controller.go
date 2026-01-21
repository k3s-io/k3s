//go:build !windows

package rootlessports

import (
	"context"
	"strings"
	"time"

	"github.com/k3s-io/k3s/pkg/rootless"
	corev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rootless-containers/rootlesskit/v2/pkg/api/client"
	"github.com/rootless-containers/rootlesskit/v2/pkg/port"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	all = "_all_"
)

func Register(ctx context.Context, serviceController corev1.ServiceController, enabled bool, httpsPort int) error {
	var (
		err            error
		rootlessClient client.Client
	)

	if rootless.Sock == "" {
		return nil
	}

	for i := 0; i < 30; i++ {
		rootlessClient, err = client.New(rootless.Sock)
		if err == nil {
			break
		}
		logrus.Infof("Waiting for rootless API socket %s: %v", rootless.Sock, err)
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		return err
	}

	h := &handler{
		enabled:        enabled,
		rootlessClient: rootlessClient,
		serviceClient:  serviceController,
		serviceCache:   serviceController.Cache(),
		httpsPort:      httpsPort,
		ctx:            ctx,
	}
	serviceController.OnChange(ctx, "rootlessports", h.serviceChanged)
	serviceController.Enqueue("", all)

	return nil
}

type handler struct {
	enabled        bool
	rootlessClient client.Client
	serviceClient  corev1.ServiceController
	serviceCache   corev1.ServiceCache
	httpsPort      int
	ctx            context.Context
}

func (h *handler) serviceChanged(key string, svc *v1.Service) (*v1.Service, error) {
	if key != all {
		h.serviceClient.Enqueue("", all)
		return svc, nil
	}

	ports, err := h.rootlessClient.PortManager().ListPorts(h.ctx)
	if err != nil {
		return svc, err
	}

	boundPorts := map[string]map[int]int{
		"tcp": {},
		"udp": {},
	}
	for _, port := range ports {
		boundPorts[port.Spec.Proto][port.Spec.ParentPort] = port.ID
	}

	toBindPort, err := h.toBindPorts()
	if err != nil {
		return svc, err
	}

	for proto, ports := range toBindPort {
		for bindPort, childBindPort := range ports {
			if _, ok := boundPorts[proto][bindPort]; ok {
				logrus.Debugf("Parent port %d/%s to child already bound", bindPort, proto)
				delete(boundPorts[proto], bindPort)
				continue
			}

			status, err := h.rootlessClient.PortManager().AddPort(h.ctx, port.Spec{
				Proto:      proto,
				ParentPort: bindPort,
				ChildPort:  childBindPort,
			})
			if err != nil {
				return svc, err
			}

			logrus.Infof("Bound parent port %s:%d/%s to child namespace port %d", status.Spec.ParentIP,
				status.Spec.ParentPort, proto, status.Spec.ChildPort)
		}
	}

	for proto, ports := range boundPorts {
		for bindPort, id := range ports {
			if err := h.rootlessClient.PortManager().RemovePort(h.ctx, id); err != nil {
				return svc, err
			}

			logrus.Infof("Removed parent port %d/%s to child namespace", bindPort, proto)
		}
	}

	return svc, nil
}

func (h *handler) toBindPorts() (map[string]map[int]int, error) {
	svcs, err := h.serviceCache.List("", labels.Everything())
	if err != nil {
		return nil, err
	}

	toBindPorts := map[string]map[int]int{
		"tcp": {h.httpsPort: h.httpsPort},
		"udp": {},
	}

	if !h.enabled {
		return toBindPorts, nil
	}

	for _, svc := range svcs {
		for _, ingress := range svc.Status.LoadBalancer.Ingress {
			if ingress.IP == "" {
				continue
			}

			for _, port := range svc.Spec.Ports {
				proto := strings.ToLower(string(port.Protocol))
				if _, ok := toBindPorts[proto]; !ok {
					logrus.Debugf("Skipping bind for unsupported protocol: %d/%s", port.Port, proto)
					continue
				}

				for _, toBindPort := range []int32{port.Port, port.NodePort} {
					if toBindPort == 0 {
						continue
					}
					if toBindPort <= 1024 {
						toBindPorts[proto][10000+int(toBindPort)] = int(toBindPort)
					} else {
						toBindPorts[proto][int(toBindPort)] = int(toBindPort)
					}
				}
			}
		}
	}

	return toBindPorts, nil
}
