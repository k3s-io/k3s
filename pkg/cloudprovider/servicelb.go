package cloudprovider

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/k3s-io/k3s/pkg/version"
	"github.com/rancher/wrangler/pkg/condition"
	coreclient "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/merr"
	"github.com/rancher/wrangler/pkg/objectset"
	"github.com/sirupsen/logrus"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	ccmapp "k8s.io/cloud-provider/app"
	servicehelper "k8s.io/cloud-provider/service/helpers"
	utilsnet "k8s.io/utils/net"
	utilpointer "k8s.io/utils/pointer"
)

var (
	finalizerName          = "svccontroller." + version.Program + ".cattle.io/daemonset"
	svcNameLabel           = "svccontroller." + version.Program + ".cattle.io/svcname"
	svcNamespaceLabel      = "svccontroller." + version.Program + ".cattle.io/svcnamespace"
	daemonsetNodeLabel     = "svccontroller." + version.Program + ".cattle.io/enablelb"
	daemonsetNodePoolLabel = "svccontroller." + version.Program + ".cattle.io/lbpool"
	nodeSelectorLabel      = "svccontroller." + version.Program + ".cattle.io/nodeselector"
	controllerName         = ccmapp.DefaultInitFuncConstructors["service"].InitContext.ClientName
)

const (
	Ready          = condition.Cond("Ready")
	DefaultLBNS    = meta.NamespaceSystem
	DefaultLBImage = "rancher/klipper-lb:v0.4.0"
)

func (k *k3s) Register(ctx context.Context,
	nodes coreclient.NodeController,
	pods coreclient.PodController,
) error {
	nodes.OnChange(ctx, controllerName, k.onChangeNode)
	pods.OnChange(ctx, controllerName, k.onChangePod)

	if err := k.createServiceLBNamespace(ctx); err != nil {
		return err
	}

	if err := k.createServiceLBServiceAccount(ctx); err != nil {
		return err
	}

	go wait.Until(k.runWorker, time.Second, ctx.Done())

	return k.removeServiceFinalizers(ctx)
}

// createServiceLBNamespace ensures that the configured namespace exists.
func (k *k3s) createServiceLBNamespace(ctx context.Context) error {
	_, err := k.client.CoreV1().Namespaces().Create(ctx, &core.Namespace{
		ObjectMeta: meta.ObjectMeta{
			Name: k.LBNamespace,
		},
	}, meta.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

// createServiceLBServiceAccount ensures that the ServiceAccount used by pods exists
func (k *k3s) createServiceLBServiceAccount(ctx context.Context) error {
	_, err := k.client.CoreV1().ServiceAccounts(k.LBNamespace).Create(ctx, &core.ServiceAccount{
		ObjectMeta: meta.ObjectMeta{
			Name:      "svclb",
			Namespace: k.LBNamespace,
		},
	}, meta.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

// onChangePod handles changes to Pods.
// If the pod has labels that tie it to a service, and the pod has an IP assigned,
// enqueue an update to the service's status.
func (k *k3s) onChangePod(key string, pod *core.Pod) (*core.Pod, error) {
	if pod == nil {
		return nil, nil
	}

	serviceName := pod.Labels[svcNameLabel]
	if serviceName == "" {
		return pod, nil
	}

	serviceNamespace := pod.Labels[svcNamespaceLabel]
	if serviceNamespace == "" {
		return pod, nil
	}

	if pod.Status.PodIP == "" {
		return pod, nil
	}

	k.workqueue.Add(serviceNamespace + "/" + serviceName)
	return pod, nil
}

// onChangeNode handles changes to Nodes. We need to handle this as we may need to kick the DaemonSet
// to add or remove pods from nodes if labels have changed.
func (k *k3s) onChangeNode(key string, node *core.Node) (*core.Node, error) {
	if node == nil {
		return nil, nil
	}
	if _, ok := node.Labels[daemonsetNodeLabel]; !ok {
		return node, nil
	}

	if err := k.updateDaemonSets(); err != nil {
		return node, err
	}

	return node, nil
}

// runWorker dequeues Service changes from the work queue
// We run a lightweight work queue to handle service updates. We don't need the full overhead
// of a wrangler service controller and shared informer cache, but we do want to run changes
// through a keyed queue to reduce thrashing when pods are updated. Much of this is cribbed from
// https://github.com/rancher/lasso/blob/release/v2.5/pkg/controller/controller.go#L173-L215
func (k *k3s) runWorker() {
	for k.processNextWorkItem() {
	}
}

// processNextWorkItem does work for a single item in the queue,
// returning a boolean that indicates if the queue should continue
// to be serviced.
func (k *k3s) processNextWorkItem() bool {
	obj, shutdown := k.workqueue.Get()

	if shutdown {
		return false
	}

	if err := k.processSingleItem(obj); err != nil && !apierrors.IsConflict(err) {
		logrus.Errorf("%s: %v", controllerName, err)
	}
	return true
}

// processSingleItem processes a single item from the work queue,
// requeueing it if the handler fails.
func (k *k3s) processSingleItem(obj interface{}) error {
	var (
		key string
		ok  bool
	)

	defer k.workqueue.Done(obj)

	if key, ok = obj.(string); !ok {
		logrus.Errorf("expected string in workqueue but got %#v", obj)
		k.workqueue.Forget(obj)
		return nil
	}
	keyParts := strings.SplitN(key, "/", 2)
	if err := k.updateStatus(keyParts[0], keyParts[1]); err != nil {
		k.workqueue.AddRateLimited(key)
		return fmt.Errorf("error updating LoadBalancer Status for %s: %v, requeueing", key, err)
	}

	k.workqueue.Forget(obj)
	return nil

}

// updateServiceStatus updates the load balancer status for the matching service, if it exists and is a
// LoadBalancer service.  The patchStatus function handles checking to see if status needs updating.
func (k *k3s) updateStatus(namespace, name string) error {
	svc, err := k.client.CoreV1().Services(namespace).Get(context.TODO(), name, meta.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if svc.Spec.Type != core.ServiceTypeLoadBalancer {
		return nil
	}

	previousStatus := svc.Status.LoadBalancer.DeepCopy()
	newStatus, err := k.getStatus(svc)
	if err != nil {
		return err
	}

	return k.patchStatus(svc, previousStatus, newStatus)
}

// getDaemonSet returns the DaemonSet that should exist for the Service.
func (k *k3s) getDaemonSet(svc *core.Service) (*apps.DaemonSet, error) {
	return k.daemonsetCache.Get(k.LBNamespace, generateName(svc))
}

// getStatus returns a LoadBalancerStatus listing ingress IPs for all ready pods
// matching the selected service.
func (k *k3s) getStatus(svc *core.Service) (*core.LoadBalancerStatus, error) {
	pods, err := k.podCache.List(k.LBNamespace, labels.SelectorFromSet(map[string]string{
		svcNameLabel:      svc.Name,
		svcNamespaceLabel: svc.Namespace,
	}))

	if err != nil {
		return nil, err
	}

	expectedIPs, err := k.podIPs(pods, svc)
	if err != nil {
		return nil, err
	}

	sort.Strings(expectedIPs)

	loadbalancer := &core.LoadBalancerStatus{}
	for _, ip := range expectedIPs {
		loadbalancer.Ingress = append(loadbalancer.Ingress, core.LoadBalancerIngress{
			IP: ip,
		})
	}

	return loadbalancer, nil
}

// patchStatus patches the service status. If the status has not changed, this function is a no-op.
func (k *k3s) patchStatus(svc *core.Service, previousStatus, newStatus *core.LoadBalancerStatus) error {
	if servicehelper.LoadBalancerStatusEqual(previousStatus, newStatus) {
		return nil
	}

	updated := svc.DeepCopy()
	updated.Status.LoadBalancer = *newStatus
	_, err := servicehelper.PatchService(k.client.CoreV1(), svc, updated)
	if err == nil {
		if len(newStatus.Ingress) == 0 {
			k.recorder.Event(svc, core.EventTypeWarning, "UnAvailableLoadBalancer", "There are no available nodes for LoadBalancer")
		} else {
			k.recorder.Eventf(svc, core.EventTypeNormal, "UpdatedLoadBalancer", "Updated LoadBalancer with new IPs: %v -> %v", ingressToString(previousStatus.Ingress), ingressToString(newStatus.Ingress))
		}
	}
	return err
}

// podIPs returns a list of IPs for Nodes hosting ServiceLB Pods.
// If at least one node has External IPs available, only external IPs are returned.
// If no nodes have External IPs set, the Internal IPs of all nodes running pods are returned.
func (k *k3s) podIPs(pods []*core.Pod, svc *core.Service) ([]string, error) {
	// Go doesn't have sets so we stuff things into a map of bools and then get lists of keys
	// to determine the unique set of IPs in use by pods.
	extIPs := map[string]bool{}
	intIPs := map[string]bool{}

	for _, pod := range pods {
		if pod.Spec.NodeName == "" || pod.Status.PodIP == "" {
			continue
		}
		if !Ready.IsTrue(pod) {
			continue
		}

		node, err := k.nodeCache.Get(pod.Spec.NodeName)
		if apierrors.IsNotFound(err) {
			continue
		} else if err != nil {
			return nil, err
		}

		for _, addr := range node.Status.Addresses {
			if addr.Type == core.NodeExternalIP {
				extIPs[addr.Address] = true
			} else if addr.Type == core.NodeInternalIP {
				intIPs[addr.Address] = true
			}
		}
	}

	keys := func(addrs map[string]bool) (ips []string) {
		for k := range addrs {
			ips = append(ips, k)
		}
		return ips
	}

	var ips []string
	if len(extIPs) > 0 {
		ips = keys(extIPs)
	} else {
		ips = keys(intIPs)
	}

	ips, err := filterByIPFamily(ips, svc)
	if err != nil {
		return nil, err
	}

	if len(ips) > 0 && k.Rootless {
		return []string{"127.0.0.1"}, nil
	}

	return ips, nil
}

// filterByIPFamily filters ips based on dual-stack parameters of the service
func filterByIPFamily(ips []string, svc *core.Service) ([]string, error) {
	var ipFamilyPolicy core.IPFamilyPolicyType
	var ipv4Addresses []string
	var ipv6Addresses []string

	for _, ip := range ips {
		if utilsnet.IsIPv4String(ip) {
			ipv4Addresses = append(ipv4Addresses, ip)
		}
		if utilsnet.IsIPv6String(ip) {
			ipv6Addresses = append(ipv6Addresses, ip)
		}
	}

	if svc.Spec.IPFamilyPolicy != nil {
		ipFamilyPolicy = *svc.Spec.IPFamilyPolicy
	}

	switch ipFamilyPolicy {
	case core.IPFamilyPolicySingleStack:
		if svc.Spec.IPFamilies[0] == core.IPv4Protocol {
			return ipv4Addresses, nil
		}
		if svc.Spec.IPFamilies[0] == core.IPv6Protocol {
			return ipv6Addresses, nil
		}
	case core.IPFamilyPolicyPreferDualStack:
		if svc.Spec.IPFamilies[0] == core.IPv4Protocol {
			ipAddresses := append(ipv4Addresses, ipv6Addresses...)
			return ipAddresses, nil
		}
		if svc.Spec.IPFamilies[0] == core.IPv6Protocol {
			ipAddresses := append(ipv6Addresses, ipv4Addresses...)
			return ipAddresses, nil
		}
	case core.IPFamilyPolicyRequireDualStack:
		if (len(ipv4Addresses) == 0) || (len(ipv6Addresses) == 0) {
			return nil, errors.New("one or more IP families did not have addresses available for service with ipFamilyPolicy=RequireDualStack")
		}
		if svc.Spec.IPFamilies[0] == core.IPv4Protocol {
			ipAddresses := append(ipv4Addresses, ipv6Addresses...)
			return ipAddresses, nil
		}
		if svc.Spec.IPFamilies[0] == core.IPv6Protocol {
			ipAddresses := append(ipv6Addresses, ipv4Addresses...)
			return ipAddresses, nil
		}
	}

	return nil, errors.New("unhandled ipFamilyPolicy")
}

// deployDaemonSet ensures that there is a DaemonSet for the service.
func (k *k3s) deployDaemonSet(ctx context.Context, svc *core.Service) error {
	ds, err := k.newDaemonSet(svc)
	if err != nil {
		return err
	}

	defer k.recorder.Eventf(svc, core.EventTypeNormal, "AppliedDaemonSet", "Applied LoadBalancer DaemonSet %s/%s", ds.Namespace, ds.Name)
	return k.processor.WithContext(ctx).WithOwner(svc).Apply(objectset.NewObjectSet(ds))
}

// deleteDaemonSet ensures that there are no DaemonSets for the given service.
func (k *k3s) deleteDaemonSet(ctx context.Context, svc *core.Service) error {
	name := generateName(svc)
	if err := k.client.AppsV1().DaemonSets(k.LBNamespace).Delete(ctx, name, meta.DeleteOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	defer k.recorder.Eventf(svc, core.EventTypeNormal, "DeletedDaemonSet", "Deleted LoadBalancer DaemonSet %s/%s", k.LBNamespace, name)
	return nil
}

// newDaemonSet creates a DaemonSet to ensure that ServiceLB pods are run on
// each eligible node.
func (k *k3s) newDaemonSet(svc *core.Service) (*apps.DaemonSet, error) {
	name := generateName(svc)
	oneInt := intstr.FromInt(1)

	sourceRanges, err := servicehelper.GetLoadBalancerSourceRanges(svc)
	if err != nil {
		return nil, err
	}

	ds := &apps.DaemonSet{
		ObjectMeta: meta.ObjectMeta{
			Name:      name,
			Namespace: k.LBNamespace,
			Labels: map[string]string{
				nodeSelectorLabel: "false",
				svcNameLabel:      svc.Name,
				svcNamespaceLabel: svc.Namespace,
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
						"app":             name,
						svcNameLabel:      svc.Name,
						svcNamespaceLabel: svc.Namespace,
					},
				},
				Spec: core.PodSpec{
					ServiceAccountName:           "svclb",
					AutomountServiceAccountToken: utilpointer.Bool(false),
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

	var sysctls []core.Sysctl
	for _, ipFamily := range svc.Spec.IPFamilies {
		switch ipFamily {
		case core.IPv4Protocol:
			sysctls = append(sysctls, core.Sysctl{Name: "net.ipv4.ip_forward", Value: "1"})
		case core.IPv6Protocol:
			sysctls = append(sysctls, core.Sysctl{Name: "net.ipv6.conf.all.forwarding", Value: "1"})
		}
	}

	ds.Spec.Template.Spec.SecurityContext = &core.PodSecurityContext{Sysctls: sysctls}

	for _, port := range svc.Spec.Ports {
		portName := fmt.Sprintf("lb-%s-%d", strings.ToLower(string(port.Protocol)), port.Port)
		container := core.Container{
			Name:            portName,
			Image:           k.LBImage,
			ImagePullPolicy: core.PullIfNotPresent,
			Ports: []core.ContainerPort{
				{
					Name:          portName,
					ContainerPort: port.Port,
					HostPort:      port.Port,
					Protocol:      port.Protocol,
				},
			},
			Env: []core.EnvVar{
				{
					Name:  "SRC_PORT",
					Value: strconv.Itoa(int(port.Port)),
				},
				{
					Name:  "SRC_RANGES",
					Value: strings.Join(sourceRanges.StringSlice(), " "),
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
					Name:  "DEST_IPS",
					Value: strings.Join(svc.Spec.ClusterIPs, " "),
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

	// Add toleration to noderole.kubernetes.io/master=*:NoSchedule
	masterToleration := core.Toleration{
		Key:      "node-role.kubernetes.io/master",
		Operator: "Exists",
		Effect:   "NoSchedule",
	}
	ds.Spec.Template.Spec.Tolerations = append(ds.Spec.Template.Spec.Tolerations, masterToleration)

	// Add toleration to noderole.kubernetes.io/control-plane=*:NoSchedule
	controlPlaneToleration := core.Toleration{
		Key:      "node-role.kubernetes.io/control-plane",
		Operator: "Exists",
		Effect:   "NoSchedule",
	}
	ds.Spec.Template.Spec.Tolerations = append(ds.Spec.Template.Spec.Tolerations, controlPlaneToleration)

	// Add toleration to CriticalAddonsOnly
	criticalAddonsOnlyToleration := core.Toleration{
		Key:      "CriticalAddonsOnly",
		Operator: "Exists",
	}
	ds.Spec.Template.Spec.Tolerations = append(ds.Spec.Template.Spec.Tolerations, criticalAddonsOnlyToleration)

	// Add node selector only if label "svccontroller.k3s.cattle.io/enablelb" exists on the nodes
	enableNodeSelector, err := k.nodeHasDaemonSetLabel()
	if err != nil {
		return nil, err
	}
	if enableNodeSelector {
		ds.Spec.Template.Spec.NodeSelector = map[string]string{
			daemonsetNodeLabel: "true",
		}
		// Add node selector for "svccontroller.k3s.cattle.io/lbpool=<pool>" if service has lbpool label
		if svc.Labels[daemonsetNodePoolLabel] != "" {
			ds.Spec.Template.Spec.NodeSelector[daemonsetNodePoolLabel] = svc.Labels[daemonsetNodePoolLabel]
		}
		ds.Labels[nodeSelectorLabel] = "true"
	}
	return ds, nil
}

// updateDaemonSets ensures that our DaemonSets have a NodeSelector present if one is enabled,
// and do not have one if it is not. Nodes are checked for this label when the DaemonSet is generated,
// but node labels may change between Service updates and the NodeSelector needs to be updated appropriately.
func (k *k3s) updateDaemonSets() error {
	enableNodeSelector, err := k.nodeHasDaemonSetLabel()
	if err != nil {
		return err
	}

	nodeSelector := labels.SelectorFromSet(map[string]string{nodeSelectorLabel: fmt.Sprintf("%t", !enableNodeSelector)})
	daemonsets, err := k.daemonsetCache.List(k.LBNamespace, nodeSelector)
	if err != nil {
		return err
	}

	for _, ds := range daemonsets {
		ds.Labels[nodeSelectorLabel] = fmt.Sprintf("%t", enableNodeSelector)
		ds.Spec.Template.Spec.NodeSelector = map[string]string{}
		if enableNodeSelector {
			ds.Spec.Template.Spec.NodeSelector[daemonsetNodeLabel] = "true"
		}
		if _, err := k.client.AppsV1().DaemonSets(ds.Namespace).Update(context.TODO(), ds, meta.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// nodeHasDaemonSetLabel returns true if any node is labeled for inclusion or exclusion
// from use by ServiceLB. If any node is labeled, only nodes with a label value of "true"
// will be used.
func (k *k3s) nodeHasDaemonSetLabel() (bool, error) {
	selector, err := labels.Parse(daemonsetNodeLabel)
	if err != nil {
		return false, err
	}
	nodesWithLabel, err := k.nodeCache.List(selector)
	return len(nodesWithLabel) > 0, err
}

// deleteAllDaemonsets deletes all daemonsets created by this controller
func (k *k3s) deleteAllDaemonsets(ctx context.Context) error {
	return k.client.AppsV1().DaemonSets(k.LBNamespace).DeleteCollection(ctx, meta.DeleteOptions{}, meta.ListOptions{LabelSelector: nodeSelectorLabel})
}

// removeServiceFinalizers ensures that there are no finalizers left on any services.
// Previous implementations of the servicelb controller manually added finalizers to services it managed;
// these need to be removed in order to release ownership to the cloud provider implementation.
func (k *k3s) removeServiceFinalizers(ctx context.Context) error {
	services, err := k.client.CoreV1().Services(meta.NamespaceAll).List(ctx, meta.ListOptions{})
	if err != nil {
		return err
	}

	var errs merr.Errors
	for _, svc := range services.Items {
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			s, err := k.removeFinalizer(ctx, &svc)
			svc = *s
			return err
		}); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

// removeFinalizer ensures that there is not a finalizer for this controller on the Service
func (k *k3s) removeFinalizer(ctx context.Context, svc *core.Service) (*core.Service, error) {
	var found bool
	for k, v := range svc.Finalizers {
		if v != finalizerName {
			continue
		}
		found = true
		svc.Finalizers = append(svc.Finalizers[:k], svc.Finalizers[k+1:]...)
	}

	if found {
		return k.client.CoreV1().Services(svc.Namespace).Update(ctx, svc, meta.UpdateOptions{})
	}
	return svc, nil
}

// generateName generates a distinct name for the DaemonSet based on the service name and UID
func generateName(svc *core.Service) string {
	return fmt.Sprintf("svclb-%s-%s", svc.Name, svc.UID[:8])
}

// ingressToString converts a list of LoadBalancerIngress entries to strings
func ingressToString(ingresses []core.LoadBalancerIngress) []string {
	parts := make([]string, len(ingresses))
	for i, ingress := range ingresses {
		if ingress.IP != "" {
			parts[i] = ingress.IP
		} else {
			parts[i] = ingress.Hostname
		}
	}
	return parts
}
