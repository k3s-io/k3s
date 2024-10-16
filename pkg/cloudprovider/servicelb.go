package cloudprovider

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"encoding/json"
	"sigs.k8s.io/yaml"

	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/rancher/wrangler/v3/pkg/condition"
	coreclient "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	discoveryclient "github.com/rancher/wrangler/v3/pkg/generated/controllers/discovery/v1"
	"github.com/rancher/wrangler/v3/pkg/merr"
	"github.com/rancher/wrangler/v3/pkg/objectset"
	"github.com/sirupsen/logrus"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/util/retry"
	"k8s.io/cloud-provider/names"
	servicehelper "k8s.io/cloud-provider/service/helpers"
	"k8s.io/kubernetes/pkg/features"
	utilsnet "k8s.io/utils/net"
	utilsptr "k8s.io/utils/ptr"
)

var (
	finalizerName          = "svccontroller." + version.Program + ".cattle.io/daemonset"
	svcNameLabel           = "svccontroller." + version.Program + ".cattle.io/svcname"
	svcNamespaceLabel      = "svccontroller." + version.Program + ".cattle.io/svcnamespace"
	daemonsetNodeLabel     = "svccontroller." + version.Program + ".cattle.io/enablelb"
	daemonsetNodePoolLabel = "svccontroller." + version.Program + ".cattle.io/lbpool"
	nodeSelectorLabel      = "svccontroller." + version.Program + ".cattle.io/nodeselector"
	priorityAnnotation     = "svccontroller." + version.Program + ".cattle.io/priorityclassname"
	tolerationsAnnotation  = "svccontroller." + version.Program + ".cattle.io/tolerations"
	controllerName         = names.ServiceLBController
)

const (
	Ready                      = condition.Cond("Ready")
	DefaultLBNS                = meta.NamespaceSystem
	DefaultLBPriorityClassName = "system-node-critical"
)

var (
	DefaultLBImage = "rancher/klipper-lb:v0.4.9"
)

func (k *k3s) Register(ctx context.Context,
	nodes coreclient.NodeController,
	pods coreclient.PodController,
	endpointslices discoveryclient.EndpointSliceController,
) error {
	nodes.OnChange(ctx, controllerName, k.onChangeNode)
	pods.OnChange(ctx, controllerName, k.onChangePod)
	endpointslices.OnChange(ctx, controllerName, k.onChangeEndpointSlice)

	if err := k.ensureServiceLBNamespace(ctx); err != nil {
		return err
	}

	if err := k.ensureServiceLBServiceAccount(ctx); err != nil {
		return err
	}

	go wait.Until(k.runWorker, time.Second, ctx.Done())

	return k.removeServiceFinalizers(ctx)
}

// ensureServiceLBNamespace ensures that the configured namespace exists.
func (k *k3s) ensureServiceLBNamespace(ctx context.Context) error {
	ns := k.client.CoreV1().Namespaces()
	if _, err := ns.Get(ctx, k.LBNamespace, meta.GetOptions{}); err == nil || !apierrors.IsNotFound(err) {
		return err
	}
	_, err := ns.Create(ctx, &core.Namespace{
		ObjectMeta: meta.ObjectMeta{
			Name: k.LBNamespace,
		},
	}, meta.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

// ensureServiceLBServiceAccount ensures that the ServiceAccount used by pods exists.
func (k *k3s) ensureServiceLBServiceAccount(ctx context.Context) error {
	sa := k.client.CoreV1().ServiceAccounts(k.LBNamespace)
	if _, err := sa.Get(ctx, "svclb", meta.GetOptions{}); err == nil || !apierrors.IsNotFound(err) {
		return err
	}
	_, err := sa.Create(ctx, &core.ServiceAccount{
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

// onChangeEndpointSlice handles changes to EndpointSlices. This is used to ensure that LoadBalancer
// addresses only list Nodes with ready Pods, when their ExternalTrafficPolicy is set to Local.
func (k *k3s) onChangeEndpointSlice(key string, eps *discovery.EndpointSlice) (*discovery.EndpointSlice, error) {
	if eps == nil {
		return nil, nil
	}

	serviceName, ok := eps.Labels[discovery.LabelServiceName]
	if !ok {
		return eps, nil
	}

	k.workqueue.Add(eps.Namespace + "/" + serviceName)
	return eps, nil
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
	var readyNodes map[string]bool

	if servicehelper.RequestsOnlyLocalTraffic(svc) {
		readyNodes = map[string]bool{}
		eps, err := k.endpointsCache.List(svc.Namespace, labels.SelectorFromSet(labels.Set{
			discovery.LabelServiceName: svc.Name,
		}))
		if err != nil {
			return nil, err
		}

		for _, ep := range eps {
			for _, endpoint := range ep.Endpoints {
				isPod := endpoint.TargetRef != nil && endpoint.TargetRef.Kind == "Pod"
				isReady := endpoint.Conditions.Ready != nil && *endpoint.Conditions.Ready
				if isPod && isReady && endpoint.NodeName != nil {
					readyNodes[*endpoint.NodeName] = true
				}
			}
		}
	}

	pods, err := k.podCache.List(k.LBNamespace, labels.SelectorFromSet(labels.Set{
		svcNameLabel:      svc.Name,
		svcNamespaceLabel: svc.Namespace,
	}))
	if err != nil {
		return nil, err
	}

	expectedIPs, err := k.podIPs(pods, svc, readyNodes)
	if err != nil {
		return nil, err
	}

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
func (k *k3s) podIPs(pods []*core.Pod, svc *core.Service, readyNodes map[string]bool) ([]string, error) {
	extIPs := sets.Set[string]{}
	intIPs := sets.Set[string]{}

	for _, pod := range pods {
		if pod.Spec.NodeName == "" || pod.Status.PodIP == "" {
			continue
		}
		if !Ready.IsTrue(pod) {
			continue
		}
		if readyNodes != nil && !readyNodes[pod.Spec.NodeName] {
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
				extIPs.Insert(addr.Address)
			} else if addr.Type == core.NodeInternalIP {
				intIPs.Insert(addr.Address)
			}
		}
	}

	var ips []string
	if extIPs.Len() > 0 {
		ips = extIPs.UnsortedList()
	} else {
		ips = intIPs.UnsortedList()
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

// filterByIPFamily filters node IPs based on dual-stack parameters of the service
func filterByIPFamily(ips []string, svc *core.Service) ([]string, error) {
	var ipv4Addresses []string
	var ipv6Addresses []string
	var allAddresses []string

	for _, ip := range ips {
		if utilsnet.IsIPv4String(ip) {
			ipv4Addresses = append(ipv4Addresses, ip)
		}
		if utilsnet.IsIPv6String(ip) {
			ipv6Addresses = append(ipv6Addresses, ip)
		}
	}

	sort.Strings(ipv4Addresses)
	sort.Strings(ipv6Addresses)

	for _, ipFamily := range svc.Spec.IPFamilies {
		switch ipFamily {
		case core.IPv4Protocol:
			allAddresses = append(allAddresses, ipv4Addresses...)
		case core.IPv6Protocol:
			allAddresses = append(allAddresses, ipv6Addresses...)
		}
	}
	return allAddresses, nil
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
	priorityClassName := k.getPriorityClassName(svc)
	localTraffic := servicehelper.RequestsOnlyLocalTraffic(svc)
	sourceRangesSet, err := servicehelper.GetLoadBalancerSourceRanges(svc)
	if err != nil {
		return nil, err
	}
	sourceRanges := strings.Join(sourceRangesSet.StringSlice(), ",")
	securityContext := &core.PodSecurityContext{}

	for _, ipFamily := range svc.Spec.IPFamilies {
		switch ipFamily {
		case core.IPv4Protocol:
			securityContext.Sysctls = append(securityContext.Sysctls, core.Sysctl{Name: "net.ipv4.ip_forward", Value: "1"})
		case core.IPv6Protocol:
			securityContext.Sysctls = append(securityContext.Sysctls, core.Sysctl{Name: "net.ipv6.conf.all.forwarding", Value: "1"})
			if sourceRanges == "0.0.0.0/0" {
				// The upstream default load-balancer source range only includes IPv4, even if the service is IPv6-only or dual-stack.
				// If using the default range, and IPv6 is enabled, also allow IPv6.
				sourceRanges += ",::/0"
			}
		}
	}

	ds := &apps.DaemonSet{
		ObjectMeta: meta.ObjectMeta{
			Name:      name,
			Namespace: k.LBNamespace,
			Labels: labels.Set{
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
				MatchLabels: labels.Set{
					"app": name,
				},
			},
			Template: core.PodTemplateSpec{
				ObjectMeta: meta.ObjectMeta{
					Labels: labels.Set{
						"app":             name,
						svcNameLabel:      svc.Name,
						svcNamespaceLabel: svc.Namespace,
					},
				},
				Spec: core.PodSpec{
					PriorityClassName:            priorityClassName,
					ServiceAccountName:           "svclb",
					AutomountServiceAccountToken: utilsptr.To(false),
					SecurityContext:              securityContext,
					Tolerations: []core.Toleration{
						{
							Key:      util.MasterRoleLabelKey,
							Operator: "Exists",
							Effect:   "NoSchedule",
						},
						{
							Key:      util.ControlPlaneRoleLabelKey,
							Operator: "Exists",
							Effect:   "NoSchedule",
						},
						{
							Key:      "CriticalAddonsOnly",
							Operator: "Exists",
						},
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
					Value: sourceRanges,
				},
				{
					Name:  "DEST_PROTO",
					Value: string(port.Protocol),
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

		if localTraffic {
			container.Env = append(container.Env,
				core.EnvVar{
					Name:  "DEST_PORT",
					Value: strconv.Itoa(int(port.NodePort)),
				},
				core.EnvVar{
					Name: "DEST_IPS",
					ValueFrom: &core.EnvVarSource{
						FieldRef: &core.ObjectFieldSelector{
							FieldPath: getHostIPsFieldPath(),
						},
					},
				},
			)
		} else {
			container.Env = append(container.Env,
				core.EnvVar{
					Name:  "DEST_PORT",
					Value: strconv.Itoa(int(port.Port)),
				},
				core.EnvVar{
					Name:  "DEST_IPS",
					Value: strings.Join(svc.Spec.ClusterIPs, ","),
				},
			)
		}

		ds.Spec.Template.Spec.Containers = append(ds.Spec.Template.Spec.Containers, container)
	}

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

	// Fetch tolerations from the "svccontroller.k3s.cattle.io/tolerations" annotation on the service
	// and append them to the DaemonSet's pod tolerations.
	tolerations, err := k.getTolerations(svc)
	if err != nil {
		return nil, err
	}
	ds.Spec.Template.Spec.Tolerations = append(ds.Spec.Template.Spec.Tolerations, tolerations...)

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

	nodeSelector := labels.SelectorFromSet(labels.Set{nodeSelectorLabel: fmt.Sprintf("%t", !enableNodeSelector)})
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

// getPriorityClassName returns the value of the priority class name annotation on the service,
// or the system default priority class name.
func (k *k3s) getPriorityClassName(svc *core.Service) string {
	if svc != nil {
		if v, ok := svc.Annotations[priorityAnnotation]; ok {
			return v
		}
	}
	return k.LBDefaultPriorityClassName
}

// getTolerations retrieves the tolerations from a service's annotations. 
// It parses the tolerations from a JSON or YAML string stored in the annotations. 
func (k *k3s) getTolerations(svc *core.Service) ([]core.Toleration, error) {
	tolerationsStr, ok := svc.Annotations[tolerationsAnnotation]
	if !ok {
		return []core.Toleration{}, nil
	}

	var tolerations []core.Toleration
	if err := json.Unmarshal([]byte(tolerationsStr), &tolerations); err != nil {
		if err := yaml.Unmarshal([]byte(tolerationsStr), &tolerations); err != nil {
			return nil, fmt.Errorf("failed to parse tolerations from annotation %s: %v", tolerationsAnnotation, err)
		}
	}

	for i := range tolerations {
		if err := validateToleration(&tolerations[i]); err != nil {
			return nil, fmt.Errorf("validation failed for toleration %d: %v", i, err)
		}
	}

	return tolerations, nil
}

// validateToleration ensures a toleration has valid fields according to its operator.
func validateToleration(toleration *core.Toleration) error {
	if toleration.Operator == "" {
		toleration.Operator = core.TolerationOpEqual
	}

	if toleration.Key == "" && toleration.Operator != core.TolerationOpExists {
		return fmt.Errorf("toleration with empty key must have operator 'Exists'")
	}

	if toleration.Operator == core.TolerationOpExists && toleration.Value != "" {
		return fmt.Errorf("toleration with operator 'Exists' must have an empty value")
	}

	return nil
}

// generateName generates a distinct name for the DaemonSet based on the service name and UID
func generateName(svc *core.Service) string {
	name := svc.Name
	// ensure that the service name plus prefix and uuid aren't overly long, but
	// don't cut the service name at a trailing hyphen.
	if len(name) > 48 {
		trimlen := 48
		for name[trimlen-1] == '-' {
			trimlen--
		}
		name = name[0:trimlen]
	}
	return fmt.Sprintf("svclb-%s-%s", name, svc.UID[:8])
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

func getHostIPsFieldPath() string {
	if utilfeature.DefaultFeatureGate.Enabled(features.PodHostIPs) {
		return "status.hostIPs"
	}
	return "status.hostIP"
}
