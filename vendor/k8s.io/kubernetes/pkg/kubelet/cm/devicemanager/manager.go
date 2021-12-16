/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package devicemanager

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	cadvisorapi "github.com/google/cadvisor/info/v1"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	errorsutil "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	v1helper "k8s.io/kubernetes/pkg/apis/core/v1/helper"
	"k8s.io/kubernetes/pkg/features"
	"k8s.io/kubernetes/pkg/kubelet/checkpointmanager"
	"k8s.io/kubernetes/pkg/kubelet/checkpointmanager/errors"
	"k8s.io/kubernetes/pkg/kubelet/cm/devicemanager/checkpoint"
	"k8s.io/kubernetes/pkg/kubelet/cm/topologymanager"
	"k8s.io/kubernetes/pkg/kubelet/config"
	"k8s.io/kubernetes/pkg/kubelet/lifecycle"
	"k8s.io/kubernetes/pkg/kubelet/metrics"
	"k8s.io/kubernetes/pkg/kubelet/pluginmanager/cache"
	schedulerframework "k8s.io/kubernetes/pkg/scheduler/framework"
	"k8s.io/kubernetes/pkg/util/selinux"
)

// ActivePodsFunc is a function that returns a list of pods to reconcile.
type ActivePodsFunc func() []*v1.Pod

// monitorCallback is the function called when a device's health state changes,
// or new devices are reported, or old devices are deleted.
// Updated contains the most recent state of the Device.
type monitorCallback func(resourceName string, devices []pluginapi.Device)

// ManagerImpl is the structure in charge of managing Device Plugins.
type ManagerImpl struct {
	socketname string
	socketdir  string

	endpoints map[string]endpointInfo // Key is ResourceName
	mutex     sync.Mutex

	server *grpc.Server
	wg     sync.WaitGroup

	// activePods is a method for listing active pods on the node
	// so the amount of pluginResources requested by existing pods
	// could be counted when updating allocated devices
	activePods ActivePodsFunc

	// sourcesReady provides the readiness of kubelet configuration sources such as apiserver update readiness.
	// We use it to determine when we can purge inactive pods from checkpointed state.
	sourcesReady config.SourcesReady

	// callback is used for updating devices' states in one time call.
	// e.g. a new device is advertised, two old devices are deleted and a running device fails.
	callback monitorCallback

	// allDevices holds all the devices currently registered to the device manager
	allDevices ResourceDeviceInstances

	// healthyDevices contains all of the registered healthy resourceNames and their exported device IDs.
	healthyDevices map[string]sets.String

	// unhealthyDevices contains all of the unhealthy devices and their exported device IDs.
	unhealthyDevices map[string]sets.String

	// allocatedDevices contains allocated deviceIds, keyed by resourceName.
	allocatedDevices map[string]sets.String

	// podDevices contains pod to allocated device mapping.
	podDevices        *podDevices
	checkpointManager checkpointmanager.CheckpointManager

	// List of NUMA Nodes available on the underlying machine
	numaNodes []int

	// Store of Topology Affinties that the Device Manager can query.
	topologyAffinityStore topologymanager.Store

	// devicesToReuse contains devices that can be reused as they have been allocated to
	// init containers.
	devicesToReuse PodReusableDevices

	// pendingAdmissionPod contain the pod during the admission phase
	pendingAdmissionPod *v1.Pod
}

type endpointInfo struct {
	e    endpoint
	opts *pluginapi.DevicePluginOptions
}

type sourcesReadyStub struct{}

// PodReusableDevices is a map by pod name of devices to reuse.
type PodReusableDevices map[string]map[string]sets.String

func (s *sourcesReadyStub) AddSource(source string) {}
func (s *sourcesReadyStub) AllReady() bool          { return true }

// NewManagerImpl creates a new manager.
func NewManagerImpl(topology []cadvisorapi.Node, topologyAffinityStore topologymanager.Store) (*ManagerImpl, error) {
	socketPath := pluginapi.KubeletSocket
	if runtime.GOOS == "windows" {
		socketPath = os.Getenv("SYSTEMDRIVE") + pluginapi.KubeletSocketWindows
	}
	return newManagerImpl(socketPath, topology, topologyAffinityStore)
}

func newManagerImpl(socketPath string, topology []cadvisorapi.Node, topologyAffinityStore topologymanager.Store) (*ManagerImpl, error) {
	klog.V(2).InfoS("Creating Device Plugin manager", "path", socketPath)

	if socketPath == "" || !filepath.IsAbs(socketPath) {
		return nil, fmt.Errorf(errBadSocket+" %s", socketPath)
	}

	var numaNodes []int
	for _, node := range topology {
		numaNodes = append(numaNodes, node.Id)
	}

	dir, file := filepath.Split(socketPath)
	manager := &ManagerImpl{
		endpoints: make(map[string]endpointInfo),

		socketname:            file,
		socketdir:             dir,
		allDevices:            NewResourceDeviceInstances(),
		healthyDevices:        make(map[string]sets.String),
		unhealthyDevices:      make(map[string]sets.String),
		allocatedDevices:      make(map[string]sets.String),
		podDevices:            newPodDevices(),
		numaNodes:             numaNodes,
		topologyAffinityStore: topologyAffinityStore,
		devicesToReuse:        make(PodReusableDevices),
	}
	manager.callback = manager.genericDeviceUpdateCallback

	// The following structures are populated with real implementations in manager.Start()
	// Before that, initializes them to perform no-op operations.
	manager.activePods = func() []*v1.Pod { return []*v1.Pod{} }
	manager.sourcesReady = &sourcesReadyStub{}
	checkpointManager, err := checkpointmanager.NewCheckpointManager(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize checkpoint manager: %v", err)
	}
	manager.checkpointManager = checkpointManager

	return manager, nil
}

func (m *ManagerImpl) genericDeviceUpdateCallback(resourceName string, devices []pluginapi.Device) {
	m.mutex.Lock()
	m.healthyDevices[resourceName] = sets.NewString()
	m.unhealthyDevices[resourceName] = sets.NewString()
	m.allDevices[resourceName] = make(map[string]pluginapi.Device)
	for _, dev := range devices {
		m.allDevices[resourceName][dev.ID] = dev
		if dev.Health == pluginapi.Healthy {
			m.healthyDevices[resourceName].Insert(dev.ID)
		} else {
			m.unhealthyDevices[resourceName].Insert(dev.ID)
		}
	}
	m.mutex.Unlock()
	if err := m.writeCheckpoint(); err != nil {
		klog.ErrorS(err, "Writing checkpoint encountered")
	}
}

func (m *ManagerImpl) removeContents(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	var errs []error
	for _, name := range names {
		filePath := filepath.Join(dir, name)
		if filePath == m.checkpointFile() {
			continue
		}
		// TODO: Until the bug - https://github.com/golang/go/issues/33357 is fixed, os.stat wouldn't return the
		// right mode(socket) on windows. Hence deleting the file, without checking whether
		// its a socket, on windows.
		stat, err := os.Lstat(filePath)
		if err != nil {
			klog.ErrorS(err, "Failed to stat file", "path", filePath)
			continue
		}
		if stat.IsDir() {
			continue
		}
		err = os.RemoveAll(filePath)
		if err != nil {
			errs = append(errs, err)
			klog.ErrorS(err, "Failed to remove file", "path", filePath)
			continue
		}
	}
	return errorsutil.NewAggregate(errs)
}

// checkpointFile returns device plugin checkpoint file path.
func (m *ManagerImpl) checkpointFile() string {
	return filepath.Join(m.socketdir, kubeletDeviceManagerCheckpoint)
}

// Start starts the Device Plugin Manager and start initialization of
// podDevices and allocatedDevices information from checkpointed state and
// starts device plugin registration service.
func (m *ManagerImpl) Start(activePods ActivePodsFunc, sourcesReady config.SourcesReady) error {
	klog.V(2).InfoS("Starting Device Plugin manager")

	m.activePods = activePods
	m.sourcesReady = sourcesReady

	// Loads in allocatedDevices information from disk.
	err := m.readCheckpoint()
	if err != nil {
		klog.InfoS("Continue after failing to read checkpoint file. Device allocation info may NOT be up-to-date", "err", err)
	}

	socketPath := filepath.Join(m.socketdir, m.socketname)
	if err = os.MkdirAll(m.socketdir, 0750); err != nil {
		return err
	}
	if selinux.SELinuxEnabled() {
		if err := selinux.SetFileLabel(m.socketdir, config.KubeletPluginsDirSELinuxLabel); err != nil {
			klog.InfoS("Unprivileged containerized plugins might not work. Could not set selinux context on socket dir", "path", m.socketdir, "err", err)
		}
	}

	// Removes all stale sockets in m.socketdir. Device plugins can monitor
	// this and use it as a signal to re-register with the new Kubelet.
	if err := m.removeContents(m.socketdir); err != nil {
		klog.ErrorS(err, "Fail to clean up stale content under socket dir", "path", m.socketdir)
	}

	s, err := net.Listen("unix", socketPath)
	if err != nil {
		klog.ErrorS(err, "Failed to listen to socket while starting device plugin registry")
		return err
	}

	m.wg.Add(1)
	m.server = grpc.NewServer([]grpc.ServerOption{}...)

	pluginapi.RegisterRegistrationServer(m.server, m)
	go func() {
		defer m.wg.Done()
		m.server.Serve(s)
	}()

	klog.V(2).InfoS("Serving device plugin registration server on socket", "path", socketPath)

	return nil
}

// GetWatcherHandler returns the plugin handler
func (m *ManagerImpl) GetWatcherHandler() cache.PluginHandler {
	if f, err := os.Create(m.socketdir + "DEPRECATION"); err != nil {
		klog.ErrorS(err, "Failed to create deprecation file at socket dir", "path", m.socketdir)
	} else {
		f.Close()
		klog.V(4).InfoS("Created deprecation file", "path", f.Name())
	}

	return cache.PluginHandler(m)
}

// ValidatePlugin validates a plugin if the version is correct and the name has the format of an extended resource
func (m *ManagerImpl) ValidatePlugin(pluginName string, endpoint string, versions []string) error {
	klog.V(2).InfoS("Got Plugin at endpoint with versions", "plugin", pluginName, "endpoint", endpoint, "versions", versions)

	if !m.isVersionCompatibleWithPlugin(versions) {
		return fmt.Errorf("manager version, %s, is not among plugin supported versions %v", pluginapi.Version, versions)
	}

	if !v1helper.IsExtendedResourceName(v1.ResourceName(pluginName)) {
		return fmt.Errorf("invalid name of device plugin socket: %s", fmt.Sprintf(errInvalidResourceName, pluginName))
	}

	return nil
}

// RegisterPlugin starts the endpoint and registers it
// TODO: Start the endpoint and wait for the First ListAndWatch call
//       before registering the plugin
func (m *ManagerImpl) RegisterPlugin(pluginName string, endpoint string, versions []string) error {
	klog.V(2).InfoS("Registering plugin at endpoint", "plugin", pluginName, "endpoint", endpoint)

	e, err := newEndpointImpl(endpoint, pluginName, m.callback)
	if err != nil {
		return fmt.Errorf("failed to dial device plugin with socketPath %s: %v", endpoint, err)
	}

	options, err := e.client.GetDevicePluginOptions(context.Background(), &pluginapi.Empty{})
	if err != nil {
		return fmt.Errorf("failed to get device plugin options: %v", err)
	}

	m.registerEndpoint(pluginName, options, e)
	go m.runEndpoint(pluginName, e)

	return nil
}

// DeRegisterPlugin deregisters the plugin
// TODO work on the behavior for deregistering plugins
// e.g: Should we delete the resource
func (m *ManagerImpl) DeRegisterPlugin(pluginName string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Note: This will mark the resource unhealthy as per the behavior
	// in runEndpoint
	if eI, ok := m.endpoints[pluginName]; ok {
		eI.e.stop()
	}
}

func (m *ManagerImpl) isVersionCompatibleWithPlugin(versions []string) bool {
	// TODO(vikasc): Currently this is fine as we only have a single supported version. When we do need to support
	// multiple versions in the future, we may need to extend this function to return a supported version.
	// E.g., say kubelet supports v1beta1 and v1beta2, and we get v1alpha1 and v1beta1 from a device plugin,
	// this function should return v1beta1
	for _, version := range versions {
		for _, supportedVersion := range pluginapi.SupportedVersions {
			if version == supportedVersion {
				return true
			}
		}
	}
	return false
}

// Allocate is the call that you can use to allocate a set of devices
// from the registered device plugins.
func (m *ManagerImpl) Allocate(pod *v1.Pod, container *v1.Container) error {
	// The pod is during the admission phase. We need to save the pod to avoid it
	// being cleaned before the admission ended
	m.setPodPendingAdmission(pod)

	if _, ok := m.devicesToReuse[string(pod.UID)]; !ok {
		m.devicesToReuse[string(pod.UID)] = make(map[string]sets.String)
	}
	// If pod entries to m.devicesToReuse other than the current pod exist, delete them.
	for podUID := range m.devicesToReuse {
		if podUID != string(pod.UID) {
			delete(m.devicesToReuse, podUID)
		}
	}
	// Allocate resources for init containers first as we know the caller always loops
	// through init containers before looping through app containers. Should the caller
	// ever change those semantics, this logic will need to be amended.
	for _, initContainer := range pod.Spec.InitContainers {
		if container.Name == initContainer.Name {
			if err := m.allocateContainerResources(pod, container, m.devicesToReuse[string(pod.UID)]); err != nil {
				return err
			}
			m.podDevices.addContainerAllocatedResources(string(pod.UID), container.Name, m.devicesToReuse[string(pod.UID)])
			return nil
		}
	}
	if err := m.allocateContainerResources(pod, container, m.devicesToReuse[string(pod.UID)]); err != nil {
		return err
	}
	m.podDevices.removeContainerAllocatedResources(string(pod.UID), container.Name, m.devicesToReuse[string(pod.UID)])
	return nil

}

// UpdatePluginResources updates node resources based on devices already allocated to pods.
func (m *ManagerImpl) UpdatePluginResources(node *schedulerframework.NodeInfo, attrs *lifecycle.PodAdmitAttributes) error {
	pod := attrs.Pod

	// quick return if no pluginResources requested
	if !m.podDevices.hasPod(string(pod.UID)) {
		return nil
	}

	m.sanitizeNodeAllocatable(node)
	return nil
}

// Register registers a device plugin.
func (m *ManagerImpl) Register(ctx context.Context, r *pluginapi.RegisterRequest) (*pluginapi.Empty, error) {
	klog.InfoS("Got registration request from device plugin with resource", "resourceName", r.ResourceName)
	metrics.DevicePluginRegistrationCount.WithLabelValues(r.ResourceName).Inc()
	var versionCompatible bool
	for _, v := range pluginapi.SupportedVersions {
		if r.Version == v {
			versionCompatible = true
			break
		}
	}
	if !versionCompatible {
		err := fmt.Errorf(errUnsupportedVersion, r.Version, pluginapi.SupportedVersions)
		klog.InfoS("Bad registration request from device plugin with resource", "resourceName", r.ResourceName, "err", err)
		return &pluginapi.Empty{}, err
	}

	if !v1helper.IsExtendedResourceName(v1.ResourceName(r.ResourceName)) {
		err := fmt.Errorf(errInvalidResourceName, r.ResourceName)
		klog.InfoS("Bad registration request from device plugin", "err", err)
		return &pluginapi.Empty{}, err
	}

	// TODO: for now, always accepts newest device plugin. Later may consider to
	// add some policies here, e.g., verify whether an old device plugin with the
	// same resource name is still alive to determine whether we want to accept
	// the new registration.
	go m.addEndpoint(r)

	return &pluginapi.Empty{}, nil
}

// Stop is the function that can stop the gRPC server.
// Can be called concurrently, more than once, and is safe to call
// without a prior Start.
func (m *ManagerImpl) Stop() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for _, eI := range m.endpoints {
		eI.e.stop()
	}

	if m.server == nil {
		return nil
	}
	m.server.Stop()
	m.wg.Wait()
	m.server = nil
	return nil
}

func (m *ManagerImpl) registerEndpoint(resourceName string, options *pluginapi.DevicePluginOptions, e endpoint) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.endpoints[resourceName] = endpointInfo{e: e, opts: options}
	klog.V(2).InfoS("Registered endpoint", "endpoint", e)
}

func (m *ManagerImpl) runEndpoint(resourceName string, e endpoint) {
	e.run()
	e.stop()

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if old, ok := m.endpoints[resourceName]; ok && old.e == e {
		m.markResourceUnhealthy(resourceName)
	}

	klog.V(2).InfoS("Endpoint became unhealthy", "resourceName", resourceName, "endpoint", e)
}

func (m *ManagerImpl) addEndpoint(r *pluginapi.RegisterRequest) {
	new, err := newEndpointImpl(filepath.Join(m.socketdir, r.Endpoint), r.ResourceName, m.callback)
	if err != nil {
		klog.ErrorS(err, "Failed to dial device plugin with request", "request", r)
		return
	}
	m.registerEndpoint(r.ResourceName, r.Options, new)
	go func() {
		m.runEndpoint(r.ResourceName, new)
	}()
}

func (m *ManagerImpl) markResourceUnhealthy(resourceName string) {
	klog.V(2).InfoS("Mark all resources Unhealthy for resource", "resourceName", resourceName)
	healthyDevices := sets.NewString()
	if _, ok := m.healthyDevices[resourceName]; ok {
		healthyDevices = m.healthyDevices[resourceName]
		m.healthyDevices[resourceName] = sets.NewString()
	}
	if _, ok := m.unhealthyDevices[resourceName]; !ok {
		m.unhealthyDevices[resourceName] = sets.NewString()
	}
	m.unhealthyDevices[resourceName] = m.unhealthyDevices[resourceName].Union(healthyDevices)
}

// GetCapacity is expected to be called when Kubelet updates its node status.
// The first returned variable contains the registered device plugin resource capacity.
// The second returned variable contains the registered device plugin resource allocatable.
// The third returned variable contains previously registered resources that are no longer active.
// Kubelet uses this information to update resource capacity/allocatable in its node status.
// After the call, device plugin can remove the inactive resources from its internal list as the
// change is already reflected in Kubelet node status.
// Note in the special case after Kubelet restarts, device plugin resource capacities can
// temporarily drop to zero till corresponding device plugins re-register. This is OK because
// cm.UpdatePluginResource() run during predicate Admit guarantees we adjust nodeinfo
// capacity for already allocated pods so that they can continue to run. However, new pods
// requiring device plugin resources will not be scheduled till device plugin re-registers.
func (m *ManagerImpl) GetCapacity() (v1.ResourceList, v1.ResourceList, []string) {
	needsUpdateCheckpoint := false
	var capacity = v1.ResourceList{}
	var allocatable = v1.ResourceList{}
	deletedResources := sets.NewString()
	m.mutex.Lock()
	for resourceName, devices := range m.healthyDevices {
		eI, ok := m.endpoints[resourceName]
		if (ok && eI.e.stopGracePeriodExpired()) || !ok {
			// The resources contained in endpoints and (un)healthyDevices
			// should always be consistent. Otherwise, we run with the risk
			// of failing to garbage collect non-existing resources or devices.
			if !ok {
				klog.ErrorS(nil, "Unexpected: healthyDevices and endpoints are out of sync")
			}
			delete(m.endpoints, resourceName)
			delete(m.healthyDevices, resourceName)
			deletedResources.Insert(resourceName)
			needsUpdateCheckpoint = true
		} else {
			capacity[v1.ResourceName(resourceName)] = *resource.NewQuantity(int64(devices.Len()), resource.DecimalSI)
			allocatable[v1.ResourceName(resourceName)] = *resource.NewQuantity(int64(devices.Len()), resource.DecimalSI)
		}
	}
	for resourceName, devices := range m.unhealthyDevices {
		eI, ok := m.endpoints[resourceName]
		if (ok && eI.e.stopGracePeriodExpired()) || !ok {
			if !ok {
				klog.ErrorS(nil, "Unexpected: unhealthyDevices and endpoints are out of sync")
			}
			delete(m.endpoints, resourceName)
			delete(m.unhealthyDevices, resourceName)
			deletedResources.Insert(resourceName)
			needsUpdateCheckpoint = true
		} else {
			capacityCount := capacity[v1.ResourceName(resourceName)]
			unhealthyCount := *resource.NewQuantity(int64(devices.Len()), resource.DecimalSI)
			capacityCount.Add(unhealthyCount)
			capacity[v1.ResourceName(resourceName)] = capacityCount
		}
	}
	m.mutex.Unlock()
	if needsUpdateCheckpoint {
		if err := m.writeCheckpoint(); err != nil {
			klog.ErrorS(err, "Error on writing checkpoint")
		}
	}
	return capacity, allocatable, deletedResources.UnsortedList()
}

// Checkpoints device to container allocation information to disk.
func (m *ManagerImpl) writeCheckpoint() error {
	m.mutex.Lock()
	registeredDevs := make(map[string][]string)
	for resource, devices := range m.healthyDevices {
		registeredDevs[resource] = devices.UnsortedList()
	}
	data := checkpoint.New(m.podDevices.toCheckpointData(),
		registeredDevs)
	m.mutex.Unlock()
	err := m.checkpointManager.CreateCheckpoint(kubeletDeviceManagerCheckpoint, data)
	if err != nil {
		err2 := fmt.Errorf("failed to write checkpoint file %q: %v", kubeletDeviceManagerCheckpoint, err)
		klog.InfoS("Failed to write checkpoint file", "err", err)
		return err2
	}
	return nil
}

// Reads device to container allocation information from disk, and populates
// m.allocatedDevices accordingly.
func (m *ManagerImpl) readCheckpoint() error {
	// the vast majority of time we restore a compatible checkpoint, so we try
	// the current version first. Trying to restore older format checkpoints is
	// relevant only in the kubelet upgrade flow, which happens once in a
	// (long) while.
	cp, err := m.getCheckpointV2()
	if err != nil {
		if err == errors.ErrCheckpointNotFound {
			// no point in trying anything else
			klog.InfoS("Failed to read data from checkpoint", "checkpoint", kubeletDeviceManagerCheckpoint, "err", err)
			return nil
		}

		var errv1 error
		// one last try: maybe it's a old format checkpoint?
		cp, errv1 = m.getCheckpointV1()
		if errv1 != nil {
			klog.InfoS("Failed to read checkpoint V1 file", "err", errv1)
			// intentionally return the parent error. We expect to restore V1 checkpoints
			// a tiny fraction of time, so what matters most is the current checkpoint read error.
			return err
		}
		klog.InfoS("Read data from a V1 checkpoint", "checkpoint", kubeletDeviceManagerCheckpoint)
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()
	podDevices, registeredDevs := cp.GetDataInLatestFormat()
	m.podDevices.fromCheckpointData(podDevices)
	m.allocatedDevices = m.podDevices.devices()
	for resource := range registeredDevs {
		// During start up, creates empty healthyDevices list so that the resource capacity
		// will stay zero till the corresponding device plugin re-registers.
		m.healthyDevices[resource] = sets.NewString()
		m.unhealthyDevices[resource] = sets.NewString()
		m.endpoints[resource] = endpointInfo{e: newStoppedEndpointImpl(resource), opts: nil}
	}
	return nil
}

func (m *ManagerImpl) getCheckpointV2() (checkpoint.DeviceManagerCheckpoint, error) {
	registeredDevs := make(map[string][]string)
	devEntries := make([]checkpoint.PodDevicesEntry, 0)
	cp := checkpoint.New(devEntries, registeredDevs)
	err := m.checkpointManager.GetCheckpoint(kubeletDeviceManagerCheckpoint, cp)
	return cp, err
}

func (m *ManagerImpl) getCheckpointV1() (checkpoint.DeviceManagerCheckpoint, error) {
	registeredDevs := make(map[string][]string)
	devEntries := make([]checkpoint.PodDevicesEntryV1, 0)
	cp := checkpoint.NewV1(devEntries, registeredDevs)
	err := m.checkpointManager.GetCheckpoint(kubeletDeviceManagerCheckpoint, cp)
	return cp, err
}

// UpdateAllocatedDevices frees any Devices that are bound to terminated pods.
func (m *ManagerImpl) UpdateAllocatedDevices() {
	if !m.sourcesReady.AllReady() {
		return
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	activeAndAdmittedPods := m.activePods()
	if m.pendingAdmissionPod != nil {
		activeAndAdmittedPods = append(activeAndAdmittedPods, m.pendingAdmissionPod)
	}

	podsToBeRemoved := m.podDevices.pods()
	for _, pod := range activeAndAdmittedPods {
		podsToBeRemoved.Delete(string(pod.UID))
	}
	if len(podsToBeRemoved) <= 0 {
		return
	}
	klog.V(3).InfoS("Pods to be removed", "podUIDs", podsToBeRemoved.List())
	m.podDevices.delete(podsToBeRemoved.List())
	// Regenerated allocatedDevices after we update pod allocation information.
	m.allocatedDevices = m.podDevices.devices()
}

// Returns list of device Ids we need to allocate with Allocate rpc call.
// Returns empty list in case we don't need to issue the Allocate rpc call.
func (m *ManagerImpl) devicesToAllocate(podUID, contName, resource string, required int, reusableDevices sets.String) (sets.String, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	needed := required
	// Gets list of devices that have already been allocated.
	// This can happen if a container restarts for example.
	devices := m.podDevices.containerDevices(podUID, contName, resource)
	if devices != nil {
		klog.V(3).InfoS("Found pre-allocated devices for resource on pod", "resourceName", resource, "containerName", contName, "podUID", string(podUID), "devices", devices.List())
		needed = needed - devices.Len()
		// A pod's resource is not expected to change once admitted by the API server,
		// so just fail loudly here. We can revisit this part if this no longer holds.
		if needed != 0 {
			return nil, fmt.Errorf("pod %q container %q changed request for resource %q from %d to %d", string(podUID), contName, resource, devices.Len(), required)
		}
	}
	if needed == 0 {
		// No change, no work.
		return nil, nil
	}
	klog.V(3).InfoS("Need devices to allocate for pod", "deviceNumber", needed, "resourceName", resource, "podUID", string(podUID), "containerName", contName)
	// Check if resource registered with devicemanager
	if _, ok := m.healthyDevices[resource]; !ok {
		return nil, fmt.Errorf("can't allocate unregistered device %s", resource)
	}

	// Declare the list of allocated devices.
	// This will be populated and returned below.
	allocated := sets.NewString()

	// Create a closure to help with device allocation
	// Returns 'true' once no more devices need to be allocated.
	allocateRemainingFrom := func(devices sets.String) bool {
		for device := range devices.Difference(allocated) {
			m.allocatedDevices[resource].Insert(device)
			allocated.Insert(device)
			needed--
			if needed == 0 {
				return true
			}
		}
		return false
	}

	// Allocates from reusableDevices list first.
	if allocateRemainingFrom(reusableDevices) {
		return allocated, nil
	}

	// Needs to allocate additional devices.
	if m.allocatedDevices[resource] == nil {
		m.allocatedDevices[resource] = sets.NewString()
	}

	// Gets Devices in use.
	devicesInUse := m.allocatedDevices[resource]
	// Gets Available devices.
	available := m.healthyDevices[resource].Difference(devicesInUse)
	if available.Len() < needed {
		return nil, fmt.Errorf("requested number of devices unavailable for %s. Requested: %d, Available: %d", resource, needed, available.Len())
	}

	// Filters available Devices based on NUMA affinity.
	aligned, unaligned, noAffinity := m.filterByAffinity(podUID, contName, resource, available)

	// If we can allocate all remaining devices from the set of aligned ones, then
	// give the plugin the chance to influence which ones to allocate from that set.
	if needed < aligned.Len() {
		// First allocate from the preferred devices list (if available).
		preferred, err := m.callGetPreferredAllocationIfAvailable(podUID, contName, resource, aligned.Union(allocated), allocated, required)
		if err != nil {
			return nil, err
		}
		if allocateRemainingFrom(preferred.Intersection(aligned)) {
			return allocated, nil
		}
		// Then fallback to allocate from the aligned set if no preferred list
		// is returned (or not enough devices are returned in that list).
		if allocateRemainingFrom(aligned) {
			return allocated, nil
		}

		return nil, fmt.Errorf("unexpectedly allocated less resources than required. Requested: %d, Got: %d", required, required-needed)
	}

	// If we can't allocate all remaining devices from the set of aligned ones,
	// then start by first allocating all of the  aligned devices (to ensure
	// that the alignment guaranteed by the TopologyManager is honored).
	if allocateRemainingFrom(aligned) {
		return allocated, nil
	}

	// Then give the plugin the chance to influence the decision on any
	// remaining devices to allocate.
	preferred, err := m.callGetPreferredAllocationIfAvailable(podUID, contName, resource, available.Union(allocated), allocated, required)
	if err != nil {
		return nil, err
	}
	if allocateRemainingFrom(preferred.Intersection(available)) {
		return allocated, nil
	}

	// Finally, if the plugin did not return a preferred allocation (or didn't
	// return a large enough one), then fall back to allocating the remaining
	// devices from the 'unaligned' and 'noAffinity' sets.
	if allocateRemainingFrom(unaligned) {
		return allocated, nil
	}
	if allocateRemainingFrom(noAffinity) {
		return allocated, nil
	}

	return nil, fmt.Errorf("unexpectedly allocated less resources than required. Requested: %d, Got: %d", required, required-needed)
}

func (m *ManagerImpl) filterByAffinity(podUID, contName, resource string, available sets.String) (sets.String, sets.String, sets.String) {
	// If alignment information is not available, just pass the available list back.
	hint := m.topologyAffinityStore.GetAffinity(podUID, contName)
	if !m.deviceHasTopologyAlignment(resource) || hint.NUMANodeAffinity == nil {
		return sets.NewString(), sets.NewString(), available
	}

	// Build a map of NUMA Nodes to the devices associated with them. A
	// device may be associated to multiple NUMA nodes at the same time. If an
	// available device does not have any NUMA Nodes associated with it, add it
	// to a list of NUMA Nodes for the fake NUMANode -1.
	perNodeDevices := make(map[int]sets.String)
	nodeWithoutTopology := -1
	for d := range available {
		if m.allDevices[resource][d].Topology == nil || len(m.allDevices[resource][d].Topology.Nodes) == 0 {
			if _, ok := perNodeDevices[nodeWithoutTopology]; !ok {
				perNodeDevices[nodeWithoutTopology] = sets.NewString()
			}
			perNodeDevices[nodeWithoutTopology].Insert(d)
			continue
		}

		for _, node := range m.allDevices[resource][d].Topology.Nodes {
			if _, ok := perNodeDevices[int(node.ID)]; !ok {
				perNodeDevices[int(node.ID)] = sets.NewString()
			}
			perNodeDevices[int(node.ID)].Insert(d)
		}
	}

	// Get a flat list of all of the nodes associated with available devices.
	var nodes []int
	for node := range perNodeDevices {
		nodes = append(nodes, node)
	}

	// Sort the list of nodes by:
	// 1) Nodes contained in the 'hint's affinity set
	// 2) Nodes not contained in the 'hint's affinity set
	// 3) The fake NUMANode of -1 (assuming it is included in the list)
	// Within each of the groups above, sort the nodes by how many devices they contain
	sort.Slice(nodes, func(i, j int) bool {
		// If one or the other of nodes[i] or nodes[j] is in the 'hint's affinity set
		if hint.NUMANodeAffinity.IsSet(nodes[i]) && hint.NUMANodeAffinity.IsSet(nodes[j]) {
			return perNodeDevices[nodes[i]].Len() < perNodeDevices[nodes[j]].Len()
		}
		if hint.NUMANodeAffinity.IsSet(nodes[i]) {
			return true
		}
		if hint.NUMANodeAffinity.IsSet(nodes[j]) {
			return false
		}

		// If one or the other of nodes[i] or nodes[j] is the fake NUMA node -1 (they can't both be)
		if nodes[i] == nodeWithoutTopology {
			return false
		}
		if nodes[j] == nodeWithoutTopology {
			return true
		}

		// Otherwise both nodes[i] and nodes[j] are real NUMA nodes that are not in the 'hint's' affinity list.
		return perNodeDevices[nodes[i]].Len() < perNodeDevices[nodes[j]].Len()
	})

	// Generate three sorted lists of devices. Devices in the first list come
	// from valid NUMA Nodes contained in the affinity mask. Devices in the
	// second list come from valid NUMA Nodes not in the affinity mask. Devices
	// in the third list come from devices with no NUMA Node association (i.e.
	// those mapped to the fake NUMA Node -1). Because we loop through the
	// sorted list of NUMA nodes in order, within each list, devices are sorted
	// by their connection to NUMA Nodes with more devices on them.
	var fromAffinity []string
	var notFromAffinity []string
	var withoutTopology []string
	for d := range available {
		// Since the same device may be associated with multiple NUMA Nodes. We
		// need to be careful not to add each device to multiple lists. The
		// logic below ensures this by breaking after the first NUMA node that
		// has the device is encountered.
		for _, n := range nodes {
			if perNodeDevices[n].Has(d) {
				if n == nodeWithoutTopology {
					withoutTopology = append(withoutTopology, d)
				} else if hint.NUMANodeAffinity.IsSet(n) {
					fromAffinity = append(fromAffinity, d)
				} else {
					notFromAffinity = append(notFromAffinity, d)
				}
				break
			}
		}
	}

	// Return all three lists containing the full set of devices across them.
	return sets.NewString(fromAffinity...), sets.NewString(notFromAffinity...), sets.NewString(withoutTopology...)
}

// allocateContainerResources attempts to allocate all of required device
// plugin resources for the input container, issues an Allocate rpc request
// for each new device resource requirement, processes their AllocateResponses,
// and updates the cached containerDevices on success.
func (m *ManagerImpl) allocateContainerResources(pod *v1.Pod, container *v1.Container, devicesToReuse map[string]sets.String) error {
	podUID := string(pod.UID)
	contName := container.Name
	allocatedDevicesUpdated := false
	needsUpdateCheckpoint := false
	// Extended resources are not allowed to be overcommitted.
	// Since device plugin advertises extended resources,
	// therefore Requests must be equal to Limits and iterating
	// over the Limits should be sufficient.
	for k, v := range container.Resources.Limits {
		resource := string(k)
		needed := int(v.Value())
		klog.V(3).InfoS("Looking for needed resources", "needed", needed, "resourceName", resource)
		if !m.isDevicePluginResource(resource) {
			continue
		}
		// Updates allocatedDevices to garbage collect any stranded resources
		// before doing the device plugin allocation.
		if !allocatedDevicesUpdated {
			m.UpdateAllocatedDevices()
			allocatedDevicesUpdated = true
		}
		allocDevices, err := m.devicesToAllocate(podUID, contName, resource, needed, devicesToReuse[resource])
		if err != nil {
			return err
		}
		if allocDevices == nil || len(allocDevices) <= 0 {
			continue
		}

		needsUpdateCheckpoint = true

		startRPCTime := time.Now()
		// Manager.Allocate involves RPC calls to device plugin, which
		// could be heavy-weight. Therefore we want to perform this operation outside
		// mutex lock. Note if Allocate call fails, we may leave container resources
		// partially allocated for the failed container. We rely on UpdateAllocatedDevices()
		// to garbage collect these resources later. Another side effect is that if
		// we have X resource A and Y resource B in total, and two containers, container1
		// and container2 both require X resource A and Y resource B. Both allocation
		// requests may fail if we serve them in mixed order.
		// TODO: may revisit this part later if we see inefficient resource allocation
		// in real use as the result of this. Should also consider to parallelize device
		// plugin Allocate grpc calls if it becomes common that a container may require
		// resources from multiple device plugins.
		m.mutex.Lock()
		eI, ok := m.endpoints[resource]
		m.mutex.Unlock()
		if !ok {
			m.mutex.Lock()
			m.allocatedDevices = m.podDevices.devices()
			m.mutex.Unlock()
			return fmt.Errorf("unknown Device Plugin %s", resource)
		}

		devs := allocDevices.UnsortedList()
		// TODO: refactor this part of code to just append a ContainerAllocationRequest
		// in a passed in AllocateRequest pointer, and issues a single Allocate call per pod.
		klog.V(3).InfoS("Making allocation request for device plugin", "devices", devs, "resourceName", resource)
		resp, err := eI.e.allocate(devs)
		metrics.DevicePluginAllocationDuration.WithLabelValues(resource).Observe(metrics.SinceInSeconds(startRPCTime))
		if err != nil {
			// In case of allocation failure, we want to restore m.allocatedDevices
			// to the actual allocated state from m.podDevices.
			m.mutex.Lock()
			m.allocatedDevices = m.podDevices.devices()
			m.mutex.Unlock()
			return err
		}

		if len(resp.ContainerResponses) == 0 {
			return fmt.Errorf("no containers return in allocation response %v", resp)
		}

		allocDevicesWithNUMA := checkpoint.NewDevicesPerNUMA()
		// Update internal cached podDevices state.
		m.mutex.Lock()
		for dev := range allocDevices {
			if m.allDevices[resource][dev].Topology == nil || len(m.allDevices[resource][dev].Topology.Nodes) == 0 {
				allocDevicesWithNUMA[0] = append(allocDevicesWithNUMA[0], dev)
				continue
			}
			for idx := range m.allDevices[resource][dev].Topology.Nodes {
				node := m.allDevices[resource][dev].Topology.Nodes[idx]
				allocDevicesWithNUMA[node.ID] = append(allocDevicesWithNUMA[node.ID], dev)
			}
		}
		m.mutex.Unlock()
		m.podDevices.insert(podUID, contName, resource, allocDevicesWithNUMA, resp.ContainerResponses[0])
	}

	if needsUpdateCheckpoint {
		return m.writeCheckpoint()
	}

	return nil
}

// GetDeviceRunContainerOptions checks whether we have cached containerDevices
// for the passed-in <pod, container> and returns its DeviceRunContainerOptions
// for the found one. An empty struct is returned in case no cached state is found.
func (m *ManagerImpl) GetDeviceRunContainerOptions(pod *v1.Pod, container *v1.Container) (*DeviceRunContainerOptions, error) {
	podUID := string(pod.UID)
	contName := container.Name
	needsReAllocate := false
	for k, v := range container.Resources.Limits {
		resource := string(k)
		if !m.isDevicePluginResource(resource) || v.Value() == 0 {
			continue
		}
		err := m.callPreStartContainerIfNeeded(podUID, contName, resource)
		if err != nil {
			return nil, err
		}
		// This is a device plugin resource yet we don't have cached
		// resource state. This is likely due to a race during node
		// restart. We re-issue allocate request to cover this race.
		if m.podDevices.containerDevices(podUID, contName, resource) == nil {
			needsReAllocate = true
		}
	}
	if needsReAllocate {
		klog.V(2).InfoS("Needs to re-allocate device plugin resources for pod", "pod", klog.KObj(pod), "containerName", container.Name)
		if err := m.Allocate(pod, container); err != nil {
			return nil, err
		}
	}
	return m.podDevices.deviceRunContainerOptions(string(pod.UID), container.Name), nil
}

// callPreStartContainerIfNeeded issues PreStartContainer grpc call for device plugin resource
// with PreStartRequired option set.
func (m *ManagerImpl) callPreStartContainerIfNeeded(podUID, contName, resource string) error {
	m.mutex.Lock()
	eI, ok := m.endpoints[resource]
	if !ok {
		m.mutex.Unlock()
		return fmt.Errorf("endpoint not found in cache for a registered resource: %s", resource)
	}

	if eI.opts == nil || !eI.opts.PreStartRequired {
		m.mutex.Unlock()
		klog.V(4).InfoS("Plugin options indicate to skip PreStartContainer for resource", "resourceName", resource)
		return nil
	}

	devices := m.podDevices.containerDevices(podUID, contName, resource)
	if devices == nil {
		m.mutex.Unlock()
		return fmt.Errorf("no devices found allocated in local cache for pod %s, container %s, resource %s", string(podUID), contName, resource)
	}

	m.mutex.Unlock()
	devs := devices.UnsortedList()
	klog.V(4).InfoS("Issuing a PreStartContainer call for container", "containerName", contName, "podUID", string(podUID))
	_, err := eI.e.preStartContainer(devs)
	if err != nil {
		return fmt.Errorf("device plugin PreStartContainer rpc failed with err: %v", err)
	}
	// TODO: Add metrics support for init RPC
	return nil
}

// callGetPreferredAllocationIfAvailable issues GetPreferredAllocation grpc
// call for device plugin resource with GetPreferredAllocationAvailable option set.
func (m *ManagerImpl) callGetPreferredAllocationIfAvailable(podUID, contName, resource string, available, mustInclude sets.String, size int) (sets.String, error) {
	eI, ok := m.endpoints[resource]
	if !ok {
		return nil, fmt.Errorf("endpoint not found in cache for a registered resource: %s", resource)
	}

	if eI.opts == nil || !eI.opts.GetPreferredAllocationAvailable {
		klog.V(4).InfoS("Plugin options indicate to skip GetPreferredAllocation for resource", "resourceName", resource)
		return nil, nil
	}

	m.mutex.Unlock()
	klog.V(4).InfoS("Issuing a GetPreferredAllocation call for container", "containerName", contName, "podUID", string(podUID))
	resp, err := eI.e.getPreferredAllocation(available.UnsortedList(), mustInclude.UnsortedList(), size)
	m.mutex.Lock()
	if err != nil {
		return nil, fmt.Errorf("device plugin GetPreferredAllocation rpc failed with err: %v", err)
	}
	if resp != nil && len(resp.ContainerResponses) > 0 {
		return sets.NewString(resp.ContainerResponses[0].DeviceIDs...), nil
	}
	return sets.NewString(), nil
}

// sanitizeNodeAllocatable scans through allocatedDevices in the device manager
// and if necessary, updates allocatableResource in nodeInfo to at least equal to
// the allocated capacity. This allows pods that have already been scheduled on
// the node to pass GeneralPredicates admission checking even upon device plugin failure.
func (m *ManagerImpl) sanitizeNodeAllocatable(node *schedulerframework.NodeInfo) {
	var newAllocatableResource *schedulerframework.Resource
	allocatableResource := node.Allocatable
	if allocatableResource.ScalarResources == nil {
		allocatableResource.ScalarResources = make(map[v1.ResourceName]int64)
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()
	for resource, devices := range m.allocatedDevices {
		needed := devices.Len()
		quant, ok := allocatableResource.ScalarResources[v1.ResourceName(resource)]
		if ok && int(quant) >= needed {
			continue
		}
		// Needs to update nodeInfo.AllocatableResource to make sure
		// NodeInfo.allocatableResource at least equal to the capacity already allocated.
		if newAllocatableResource == nil {
			newAllocatableResource = allocatableResource.Clone()
		}
		newAllocatableResource.ScalarResources[v1.ResourceName(resource)] = int64(needed)
	}
	if newAllocatableResource != nil {
		node.Allocatable = newAllocatableResource
	}
}

func (m *ManagerImpl) isDevicePluginResource(resource string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	_, registeredResource := m.healthyDevices[resource]
	_, allocatedResource := m.allocatedDevices[resource]
	// Return true if this is either an active device plugin resource or
	// a resource we have previously allocated.
	if registeredResource || allocatedResource {
		return true
	}
	return false
}

// GetAllocatableDevices returns information about all the devices known to the manager
func (m *ManagerImpl) GetAllocatableDevices() ResourceDeviceInstances {
	m.mutex.Lock()
	resp := m.allDevices.Clone()
	m.mutex.Unlock()
	klog.V(4).InfoS("Known devices", "numDevices", len(resp))
	return resp
}

// GetDevices returns the devices used by the specified container
func (m *ManagerImpl) GetDevices(podUID, containerName string) ResourceDeviceInstances {
	return m.podDevices.getContainerDevices(podUID, containerName)
}

// ShouldResetExtendedResourceCapacity returns whether the extended resources should be zeroed or not,
// depending on whether the node has been recreated. Absence of the checkpoint file strongly indicates the node
// has been recreated.
func (m *ManagerImpl) ShouldResetExtendedResourceCapacity() bool {
	if utilfeature.DefaultFeatureGate.Enabled(features.DevicePlugins) {
		checkpoints, err := m.checkpointManager.ListCheckpoints()
		if err != nil {
			return false
		}
		return len(checkpoints) == 0
	}
	return false
}

func (m *ManagerImpl) setPodPendingAdmission(pod *v1.Pod) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.pendingAdmissionPod = pod
}
