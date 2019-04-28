package servicelb

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	appclient "github.com/rancher/k3s/types/apis/apps/v1"
	coreclient "github.com/rancher/k3s/types/apis/core/v1"
	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/pkg/changeset"
	"github.com/rancher/norman/pkg/objectset"
	"github.com/rancher/norman/types/slice"
	"github.com/sirupsen/logrus"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	v1getter "k8s.io/client-go/kubernetes/typed/apps/v1"
	coregetter "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	image              = "rancher/klipper-lb:v0.1.1"
	svcNameLabel       = "svccontroller.k3s.cattle.io/svcname"
	daemonsetNodeLabel = "svccontroller.k3s.cattle.io/enablelb"
	nodeSelectorLabel  = "svccontroller.k3s.cattle.io/nodeselector"
	Ready              = condition.Cond("Ready")
)

var (
	trueVal = true
)

func Register(ctx context.Context, kubernetes kubernetes.Interface, enabled, rootless bool) error {
	clients := coreclient.ClientsFrom(ctx)
	appClients := appclient.ClientsFrom(ctx)

	h := &handler{
		rootless:  rootless,
		enabled:   enabled,
		nodeCache: clients.Node.Cache(),
		podCache:  clients.Pod.Cache(),
		processor: objectset.NewProcessor("svccontroller").
			Client(appClients.DaemonSet),
		serviceCache: clients.Service.Cache(),
		services:     kubernetes.CoreV1(),
		daemonsets:   kubernetes.AppsV1(),
	}

	clients.Service.OnChange(ctx, "svccontroller", h.onChangeService)
	clients.Node.OnChange(ctx, "svccontroller", h.onChangeNode)
	changeset.Watch(ctx, "svccontroller-watcher",
		h.onResourceChange,
		clients.Service,
		clients.Pod,
		clients.Endpoints)

	return nil
}

type handler struct {
	rootless     bool
	enabled      bool
	nodeCache    coreclient.NodeClientCache
	podCache     coreclient.PodClientCache
	processor    *objectset.Processor
	serviceCache coreclient.ServiceClientCache
	services     coregetter.ServicesGetter
	daemonsets   v1getter.DaemonSetsGetter
}

func (h *handler) onResourceChange(name, namespace string, obj runtime.Object) ([]changeset.Key, error) {
	if ep, ok := obj.(*core.Endpoints); ok {
		return []changeset.Key{
			{
				Name:      ep.Name,
				Namespace: ep.Namespace,
			},
		}, nil
	}

	pod, ok := obj.(*core.Pod)
	if !ok {
		return nil, nil
	}

	serviceName := pod.Labels[svcNameLabel]
	if serviceName == "" {
		return nil, nil
	}

	if pod.Status.PodIP == "" {
		return nil, nil
	}

	return []changeset.Key{
		{
			Name:      serviceName,
			Namespace: pod.Namespace,
		},
	}, nil
}

func (h *handler) onChangeService(svc *core.Service) (runtime.Object, error) {
	if svc.Spec.Type != core.ServiceTypeLoadBalancer || svc.Spec.ClusterIP == "" ||
		svc.Spec.ClusterIP == "None" {
		return svc, nil
	}

	if err := h.deployPod(svc); err != nil {
		return svc, err
	}

	// Don't return service because we don't want another update
	_, err := h.updateService(svc)
	return nil, err
}

func (h *handler) onChangeNode(node *core.Node) (runtime.Object, error) {
	if _, ok := node.Labels[daemonsetNodeLabel]; !ok {
		return node, nil
	}

	if err := h.updateDaemonSets(); err != nil {
		return node, err
	}

	return node, nil
}

func (h *handler) updateService(svc *core.Service) (runtime.Object, error) {
	pods, err := h.podCache.List(svc.Namespace, labels.SelectorFromSet(map[string]string{
		svcNameLabel: svc.Name,
	}))

	if err != nil {
		return svc, err
	}

	existingIPs := serviceIPs(svc)
	expectedIPs, err := h.podIPs(pods)
	if err != nil {
		return svc, err
	}

	sort.Strings(expectedIPs)
	sort.Strings(existingIPs)

	if slice.StringsEqual(expectedIPs, existingIPs) {
		return svc, nil
	}

	svc = svc.DeepCopy()
	svc.Status.LoadBalancer.Ingress = nil
	for _, ip := range expectedIPs {
		svc.Status.LoadBalancer.Ingress = append(svc.Status.LoadBalancer.Ingress, core.LoadBalancerIngress{
			IP: ip,
		})
	}

	logrus.Debugf("Setting service loadbalancer %s/%s to IPs %v", svc.Namespace, svc.Name, expectedIPs)
	return h.services.Services(svc.Namespace).UpdateStatus(svc)
}

func serviceIPs(svc *core.Service) []string {
	var ips []string

	for _, ingress := range svc.Status.LoadBalancer.Ingress {
		if ingress.IP != "" {
			ips = append(ips, ingress.IP)
		}
	}

	return ips
}

func (h *handler) podIPs(pods []*core.Pod) ([]string, error) {
	ips := map[string]bool{}

	for _, pod := range pods {
		if pod.Spec.NodeName == "" || pod.Status.PodIP == "" {
			continue
		}
		if !Ready.IsTrue(pod) {
			continue
		}

		node, err := h.nodeCache.Get("", pod.Spec.NodeName)
		if errors.IsNotFound(err) {
			continue
		} else if err != nil {
			return nil, err
		}

		for _, addr := range node.Status.Addresses {
			if addr.Type == core.NodeInternalIP {
				ips[addr.Address] = true
			}
		}
	}

	var ipList []string
	for k := range ips {
		ipList = append(ipList, k)
	}

	if len(ipList) > 0 && h.rootless {
		return []string{"127.0.0.1"}, nil
	}

	return ipList, nil
}

func (h *handler) deployPod(svc *core.Service) error {

	objs := objectset.NewObjectSet()
	if !h.enabled {
		return h.processor.NewDesiredSet(svc, objs).Apply()
	}

	ds, err := h.newDaemonSet(svc)
	if err != nil {
		return err
	}
	if ds != nil {
		objs.Add(ds)
	}
	return h.processor.NewDesiredSet(svc, objs).Apply()
}

func (h *handler) newDaemonSet(svc *core.Service) (*apps.DaemonSet, error) {
	name := fmt.Sprintf("svclb-%s", svc.Name)
	oneInt := intstr.FromInt(1)

	ds := &apps.DaemonSet{
		ObjectMeta: meta.ObjectMeta{
			Name:      name,
			Namespace: svc.Namespace,
			OwnerReferences: []meta.OwnerReference{
				{
					Name:       svc.Name,
					APIVersion: "v1",
					Kind:       "Service",
					UID:        svc.UID,
					Controller: &trueVal,
				},
			},
			Labels: map[string]string{
				nodeSelectorLabel: "false",
			},
		},
		TypeMeta: meta.TypeMeta{
			Kind:       "DaemonSet",
			APIVersion: "apps/v1",
		},
		Spec: apps.DaemonSetSpec{
			Selector: &meta.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: core.PodTemplateSpec{
				ObjectMeta: meta.ObjectMeta{
					Labels: map[string]string{
						"app":        name,
						svcNameLabel: svc.Name,
					},
				},
			},
			UpdateStrategy: apps.DaemonSetUpdateStrategy{
				Type: apps.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &apps.RollingUpdateDaemonSet{
					MaxUnavailable: &oneInt,
				},
			},
		},
	}

	for _, port := range svc.Spec.Ports {
		portName := fmt.Sprintf("lb-port-%d", port.Port)
		container := core.Container{
			Name:            portName,
			Image:           image,
			ImagePullPolicy: core.PullIfNotPresent,
			Ports: []core.ContainerPort{
				{
					Name:          portName,
					ContainerPort: port.Port,
					HostPort:      port.Port,
				},
			},
			Env: []core.EnvVar{
				{
					Name:  "SRC_PORT",
					Value: strconv.Itoa(int(port.Port)),
				},
				{
					Name:  "DEST_PROTO",
					Value: string(port.Protocol),
				},
				{
					Name:  "DEST_PORT",
					Value: strconv.Itoa(int(port.Port)),
				},
				{
					Name:  "DEST_IP",
					Value: svc.Spec.ClusterIP,
				},
			},
			SecurityContext: &core.SecurityContext{
				Capabilities: &core.Capabilities{
					Add: []core.Capability{
						"NET_ADMIN",
					},
				},
			},
		}

		ds.Spec.Template.Spec.Containers = append(ds.Spec.Template.Spec.Containers, container)
	}
	// Add node selector only if label "svccontroller.k3s.cattle.io/enablelb" exists on the nodes
	selector, err := labels.Parse(daemonsetNodeLabel)
	if err != nil {
		return nil, err
	}
	nodesWithLabel, err := h.nodeCache.List("", selector)
	if err != nil {
		return nil, err
	}
	if len(nodesWithLabel) > 0 {
		ds.Spec.Template.Spec.NodeSelector = map[string]string{
			daemonsetNodeLabel: "true",
		}
		ds.Labels[nodeSelectorLabel] = "true"
	}
	return ds, nil
}

func (h *handler) updateDaemonSets() error {
	daemonsets, err := h.daemonsets.DaemonSets("").List(meta.ListOptions{
		LabelSelector: nodeSelectorLabel + "=false",
	})
	if err != nil {
		return err
	}

	for _, ds := range daemonsets.Items {
		ds.Spec.Template.Spec.NodeSelector = map[string]string{
			daemonsetNodeLabel: "true",
		}
		ds.Labels[nodeSelectorLabel] = "true"
		if _, err := h.daemonsets.DaemonSets(ds.Namespace).Update(&ds); err != nil {
			return err
		}

	}

	return nil
}
