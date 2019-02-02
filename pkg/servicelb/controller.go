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
	coregetter "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	image        = "rancher/klipper-lb:v0.1.1"
	svcNameLabel = "svccontroller.k3s.cattle.io/svcname"
	Ready        = condition.Cond("Ready")
)

var (
	trueVal = true
)

func Register(ctx context.Context, kubernetes kubernetes.Interface, enabled bool) error {
	clients := coreclient.ClientsFrom(ctx)
	appClients := appclient.ClientsFrom(ctx)

	h := &handler{
		enabled:   enabled,
		nodeCache: clients.Node.Cache(),
		podCache:  clients.Pod.Cache(),
		processor: objectset.NewProcessor("svccontroller").
			Client(appClients.Deployment),
		serviceCache: clients.Service.Cache(),
		services:     kubernetes.CoreV1(),
	}

	clients.Service.OnChange(ctx, "svccontroller", h.onChange)
	changeset.Watch(ctx, "svccontroller-watcher",
		h.onPodChange,
		clients.Service,
		clients.Pod)

	return nil
}

type handler struct {
	enabled      bool
	nodeCache    coreclient.NodeClientCache
	podCache     coreclient.PodClientCache
	processor    *objectset.Processor
	serviceCache coreclient.ServiceClientCache
	services     coregetter.ServicesGetter
}

func (h *handler) onPodChange(name, namespace string, obj runtime.Object) ([]changeset.Key, error) {
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

func (h *handler) onChange(svc *core.Service) (runtime.Object, error) {
	if svc.Spec.Type != core.ServiceTypeLoadBalancer || svc.Spec.ClusterIP == "" ||
		svc.Spec.ClusterIP == "None" {
		return svc, nil
	}

	if err := h.deployPod(svc); err != nil {
		return svc, err
	}

	return h.updateService(svc)
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
	return ipList, nil
}

func (h *handler) deployPod(svc *core.Service) error {
	objs := objectset.NewObjectSet()
	if !h.enabled {
		return h.processor.NewDesiredSet(svc, objs).Apply()
	}

	dep, err := h.newDeployment(svc)
	if err != nil {
		return err
	}
	if dep != nil {
		objs.Add(dep)
	}

	return h.processor.NewDesiredSet(svc, objs).Apply()
}

func (h *handler) resolvePort(svc *core.Service, targetPort core.ServicePort) (int32, error) {
	if len(svc.Spec.Selector) == 0 {
		return 0, nil
	}

	if targetPort.TargetPort.IntVal != 0 {
		return targetPort.TargetPort.IntVal, nil
	}

	pods, err := h.podCache.List(svc.Namespace, labels.SelectorFromSet(svc.Spec.Selector))
	if err != nil {
		return 0, err
	}

	for _, pod := range pods {
		if !Ready.IsTrue(pod) {
			continue
		}
		for _, container := range pod.Spec.Containers {
			for _, port := range container.Ports {
				if port.Name == targetPort.TargetPort.StrVal {
					return port.ContainerPort, nil
				}
			}
		}
	}

	return 0, nil
}

func (h *handler) newDeployment(svc *core.Service) (*apps.Deployment, error) {
	name := fmt.Sprintf("svclb-%s", svc.Name)
	zero := intstr.FromInt(0)
	one := intstr.FromInt(1)
	two := int32(2)

	dep := &apps.Deployment{
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
		},
		TypeMeta: meta.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		Spec: apps.DeploymentSpec{
			Replicas: &two,
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
			Strategy: apps.DeploymentStrategy{
				Type: apps.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &apps.RollingUpdateDeployment{
					MaxSurge:       &zero,
					MaxUnavailable: &one,
				},
			},
		},
	}

	for _, port := range svc.Spec.Ports {
		targetPort, err := h.resolvePort(svc, port)
		if err != nil {
			return nil, err
		}

		container := core.Container{
			Name:            fmt.Sprintf("port-%s", port.Name),
			Image:           image,
			ImagePullPolicy: core.PullIfNotPresent,
			Ports: []core.ContainerPort{
				{
					Name:          port.Name,
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
					Value: strconv.Itoa(int(targetPort)),
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

		dep.Spec.Template.Spec.Containers = append(dep.Spec.Template.Spec.Containers, container)
	}

	return dep, nil
}
