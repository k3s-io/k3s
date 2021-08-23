// +build linux

/*
Copyright 2015 The Kubernetes Authors.

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

package cm

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	cgroupfs "github.com/opencontainers/runc/libcontainer/cgroups/fs"
	cgroupfs2 "github.com/opencontainers/runc/libcontainer/cgroups/fs2"
	"github.com/opencontainers/runc/libcontainer/configs"
	"k8s.io/klog/v2"
	"k8s.io/mount-utils"
	utilio "k8s.io/utils/io"
	utilpath "k8s.io/utils/path"

	libcontaineruserns "github.com/opencontainers/runc/libcontainer/userns"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	utilversion "k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apimachinery/pkg/util/wait"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/tools/record"
	internalapi "k8s.io/cri-api/pkg/apis"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
	kubefeatures "k8s.io/kubernetes/pkg/features"
	"k8s.io/kubernetes/pkg/kubelet/cadvisor"
	"k8s.io/kubernetes/pkg/kubelet/cm/admission"
	"k8s.io/kubernetes/pkg/kubelet/cm/containermap"
	"k8s.io/kubernetes/pkg/kubelet/cm/cpumanager"
	"k8s.io/kubernetes/pkg/kubelet/cm/devicemanager"
	"k8s.io/kubernetes/pkg/kubelet/cm/memorymanager"
	memorymanagerstate "k8s.io/kubernetes/pkg/kubelet/cm/memorymanager/state"
	"k8s.io/kubernetes/pkg/kubelet/cm/topologymanager"
	cmutil "k8s.io/kubernetes/pkg/kubelet/cm/util"
	"k8s.io/kubernetes/pkg/kubelet/config"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/kubelet/lifecycle"
	"k8s.io/kubernetes/pkg/kubelet/pluginmanager/cache"
	"k8s.io/kubernetes/pkg/kubelet/qos"
	"k8s.io/kubernetes/pkg/kubelet/stats/pidlimit"
	"k8s.io/kubernetes/pkg/kubelet/status"
	schedulerframework "k8s.io/kubernetes/pkg/scheduler/framework"
	"k8s.io/kubernetes/pkg/util/oom"
	"k8s.io/kubernetes/pkg/util/procfs"
	utilsysctl "k8s.io/kubernetes/pkg/util/sysctl"
)

const (
	dockerProcessName = "dockerd"
	// dockerd option --pidfile can specify path to use for daemon PID file, pid file path is default "/var/run/docker.pid"
	dockerPidFile         = "/var/run/docker.pid"
	containerdProcessName = "containerd"
	maxPidFileLength      = 1 << 10 // 1KB
)

var (
	// The docker version in which containerd was introduced.
	containerdAPIVersion = utilversion.MustParseGeneric("1.23")
)

// A non-user container tracked by the Kubelet.
type systemContainer struct {
	// Absolute name of the container.
	name string

	// CPU limit in millicores.
	cpuMillicores int64

	// Function that ensures the state of the container.
	// m is the cgroup manager for the specified container.
	ensureStateFunc func(m cgroups.Manager) error

	// Manager for the cgroups of the external container.
	manager cgroups.Manager
}

func newSystemCgroups(containerName string) (*systemContainer, error) {
	manager, err := createManager(containerName)
	if err != nil {
		return nil, err
	}
	return &systemContainer{
		name:    containerName,
		manager: manager,
	}, nil
}

type containerManagerImpl struct {
	sync.RWMutex
	cadvisorInterface cadvisor.Interface
	mountUtil         mount.Interface
	NodeConfig
	status Status
	// External containers being managed.
	systemContainers []*systemContainer
	// Tasks that are run periodically
	periodicTasks []func()
	// Holds all the mounted cgroup subsystems
	subsystems *CgroupSubsystems
	nodeInfo   *v1.Node
	// Interface for cgroup management
	cgroupManager CgroupManager
	// Capacity of this node.
	capacity v1.ResourceList
	// Capacity of this node, including internal resources.
	internalCapacity v1.ResourceList
	// Absolute cgroupfs path to a cgroup that Kubelet needs to place all pods under.
	// This path include a top level container for enforcing Node Allocatable.
	cgroupRoot CgroupName
	// Event recorder interface.
	recorder record.EventRecorder
	// Interface for QoS cgroup management
	qosContainerManager QOSContainerManager
	// Interface for exporting and allocating devices reported by device plugins.
	deviceManager devicemanager.Manager
	// Interface for CPU affinity management.
	cpuManager cpumanager.Manager
	// Interface for memory affinity management.
	memoryManager memorymanager.Manager
	// Interface for Topology resource co-ordination
	topologyManager topologymanager.Manager
}

type features struct {
	cpuHardcapping bool
}

var _ ContainerManager = &containerManagerImpl{}

// checks if the required cgroups subsystems are mounted.
// As of now, only 'cpu' and 'memory' are required.
// cpu quota is a soft requirement.
func validateSystemRequirements(mountUtil mount.Interface) (features, error) {
	const (
		cgroupMountType = "cgroup"
		localErr        = "system validation failed"
	)
	var (
		cpuMountPoint string
		f             features
	)
	mountPoints, err := mountUtil.List()
	if err != nil {
		return f, fmt.Errorf("%s - %v", localErr, err)
	}

	if cgroups.IsCgroup2UnifiedMode() {
		f.cpuHardcapping = true
		return f, nil
	}

	expectedCgroups := sets.NewString("cpu", "cpuacct", "memory")
	for _, mountPoint := range mountPoints {
		if mountPoint.Type == cgroupMountType {
			for _, opt := range mountPoint.Opts {
				if expectedCgroups.Has(opt) {
					expectedCgroups.Delete(opt)
				}
				if opt == "cpu" {
					cpuMountPoint = mountPoint.Path
				}
			}
		}
	}

	if expectedCgroups.Len() > 0 {
		return f, fmt.Errorf("%s - Following Cgroup subsystem not mounted: %v", localErr, expectedCgroups.List())
	}

	// Check if cpu quota is available.
	// CPU cgroup is required and so it expected to be mounted at this point.
	periodExists, err := utilpath.Exists(utilpath.CheckFollowSymlink, path.Join(cpuMountPoint, "cpu.cfs_period_us"))
	if err != nil {
		klog.ErrorS(err, "Failed to detect if CPU cgroup cpu.cfs_period_us is available")
	}
	quotaExists, err := utilpath.Exists(utilpath.CheckFollowSymlink, path.Join(cpuMountPoint, "cpu.cfs_quota_us"))
	if err != nil {
		klog.ErrorS(err, "Failed to detect if CPU cgroup cpu.cfs_quota_us is available")
	}
	if quotaExists && periodExists {
		f.cpuHardcapping = true
	}
	return f, nil
}

// TODO(vmarmol): Add limits to the system containers.
// Takes the absolute name of the specified containers.
// Empty container name disables use of the specified container.
func NewContainerManager(mountUtil mount.Interface, cadvisorInterface cadvisor.Interface, nodeConfig NodeConfig, failSwapOn bool, devicePluginEnabled bool, recorder record.EventRecorder) (ContainerManager, error) {
	subsystems, err := GetCgroupSubsystems()
	if err != nil {
		return nil, fmt.Errorf("failed to get mounted cgroup subsystems: %v", err)
	}

	if failSwapOn {
		// Check whether swap is enabled. The Kubelet does not support running with swap enabled.
		swapFile := "/proc/swaps"
		swapData, err := ioutil.ReadFile(swapFile)
		if err != nil {
			if os.IsNotExist(err) {
				klog.InfoS("File does not exist, assuming that swap is disabled", "path", swapFile)
			} else {
				return nil, err
			}
		} else {
			swapData = bytes.TrimSpace(swapData) // extra trailing \n
			swapLines := strings.Split(string(swapData), "\n")

			// If there is more than one line (table headers) in /proc/swaps, swap is enabled and we should
			// error out unless --fail-swap-on is set to false.
			if len(swapLines) > 1 {
				return nil, fmt.Errorf("running with swap on is not supported, please disable swap! or set --fail-swap-on flag to false. /proc/swaps contained: %v", swapLines)
			}
		}
	}

	var internalCapacity = v1.ResourceList{}
	// It is safe to invoke `MachineInfo` on cAdvisor before logically initializing cAdvisor here because
	// machine info is computed and cached once as part of cAdvisor object creation.
	// But `RootFsInfo` and `ImagesFsInfo` are not available at this moment so they will be called later during manager starts
	machineInfo, err := cadvisorInterface.MachineInfo()
	if err != nil {
		return nil, err
	}
	capacity := cadvisor.CapacityFromMachineInfo(machineInfo)
	for k, v := range capacity {
		internalCapacity[k] = v
	}
	pidlimits, err := pidlimit.Stats()
	if err == nil && pidlimits != nil && pidlimits.MaxPID != nil {
		internalCapacity[pidlimit.PIDs] = *resource.NewQuantity(
			int64(*pidlimits.MaxPID),
			resource.DecimalSI)
	}

	// Turn CgroupRoot from a string (in cgroupfs path format) to internal CgroupName
	cgroupRoot := ParseCgroupfsToCgroupName(nodeConfig.CgroupRoot)
	cgroupManager := NewCgroupManager(subsystems, nodeConfig.CgroupDriver)
	// Check if Cgroup-root actually exists on the node
	if nodeConfig.CgroupsPerQOS {
		// this does default to / when enabled, but this tests against regressions.
		if nodeConfig.CgroupRoot == "" {
			return nil, fmt.Errorf("invalid configuration: cgroups-per-qos was specified and cgroup-root was not specified. To enable the QoS cgroup hierarchy you need to specify a valid cgroup-root")
		}

		// we need to check that the cgroup root actually exists for each subsystem
		// of note, we always use the cgroupfs driver when performing this check since
		// the input is provided in that format.
		// this is important because we do not want any name conversion to occur.
		if !cgroupManager.Exists(cgroupRoot) {
			return nil, fmt.Errorf("invalid configuration: cgroup-root %q doesn't exist", cgroupRoot)
		}
		klog.InfoS("Container manager verified user specified cgroup-root exists", "cgroupRoot", cgroupRoot)
		// Include the top level cgroup for enforcing node allocatable into cgroup-root.
		// This way, all sub modules can avoid having to understand the concept of node allocatable.
		cgroupRoot = NewCgroupName(cgroupRoot, defaultNodeAllocatableCgroupName)
	}
	klog.InfoS("Creating Container Manager object based on Node Config", "nodeConfig", nodeConfig)

	qosContainerManager, err := NewQOSContainerManager(subsystems, cgroupRoot, nodeConfig, cgroupManager)
	if err != nil {
		return nil, err
	}

	cm := &containerManagerImpl{
		cadvisorInterface:   cadvisorInterface,
		mountUtil:           mountUtil,
		NodeConfig:          nodeConfig,
		subsystems:          subsystems,
		cgroupManager:       cgroupManager,
		capacity:            capacity,
		internalCapacity:    internalCapacity,
		cgroupRoot:          cgroupRoot,
		recorder:            recorder,
		qosContainerManager: qosContainerManager,
	}

	if utilfeature.DefaultFeatureGate.Enabled(kubefeatures.TopologyManager) {
		cm.topologyManager, err = topologymanager.NewManager(
			machineInfo.Topology,
			nodeConfig.ExperimentalTopologyManagerPolicy,
			nodeConfig.ExperimentalTopologyManagerScope,
		)

		if err != nil {
			return nil, err
		}

	} else {
		cm.topologyManager = topologymanager.NewFakeManager()
	}

	klog.InfoS("Creating device plugin manager", "devicePluginEnabled", devicePluginEnabled)
	if devicePluginEnabled {
		cm.deviceManager, err = devicemanager.NewManagerImpl(machineInfo.Topology, cm.topologyManager)
		cm.topologyManager.AddHintProvider(cm.deviceManager)
	} else {
		cm.deviceManager, err = devicemanager.NewManagerStub()
	}
	if err != nil {
		return nil, err
	}

	// Initialize CPU manager
	if utilfeature.DefaultFeatureGate.Enabled(kubefeatures.CPUManager) {
		cm.cpuManager, err = cpumanager.NewManager(
			nodeConfig.ExperimentalCPUManagerPolicy,
			nodeConfig.ExperimentalCPUManagerPolicyOptions,
			nodeConfig.ExperimentalCPUManagerReconcilePeriod,
			machineInfo,
			nodeConfig.NodeAllocatableConfig.ReservedSystemCPUs,
			cm.GetNodeAllocatableReservation(),
			nodeConfig.KubeletRootDir,
			cm.topologyManager,
		)
		if err != nil {
			klog.ErrorS(err, "Failed to initialize cpu manager")
			return nil, err
		}
		cm.topologyManager.AddHintProvider(cm.cpuManager)
	}

	if utilfeature.DefaultFeatureGate.Enabled(kubefeatures.MemoryManager) {
		cm.memoryManager, err = memorymanager.NewManager(
			nodeConfig.ExperimentalMemoryManagerPolicy,
			machineInfo,
			cm.GetNodeAllocatableReservation(),
			nodeConfig.ExperimentalMemoryManagerReservedMemory,
			nodeConfig.KubeletRootDir,
			cm.topologyManager,
		)
		if err != nil {
			klog.ErrorS(err, "Failed to initialize memory manager")
			return nil, err
		}
		cm.topologyManager.AddHintProvider(cm.memoryManager)
	}

	return cm, nil
}

// NewPodContainerManager is a factory method returns a PodContainerManager object
// If qosCgroups are enabled then it returns the general pod container manager
// otherwise it returns a no-op manager which essentially does nothing
func (cm *containerManagerImpl) NewPodContainerManager() PodContainerManager {
	if cm.NodeConfig.CgroupsPerQOS {
		return &podContainerManagerImpl{
			qosContainersInfo: cm.GetQOSContainersInfo(),
			subsystems:        cm.subsystems,
			cgroupManager:     cm.cgroupManager,
			podPidsLimit:      cm.ExperimentalPodPidsLimit,
			enforceCPULimits:  cm.EnforceCPULimits,
			cpuCFSQuotaPeriod: uint64(cm.CPUCFSQuotaPeriod / time.Microsecond),
		}
	}
	return &podContainerManagerNoop{
		cgroupRoot: cm.cgroupRoot,
	}
}

func (cm *containerManagerImpl) InternalContainerLifecycle() InternalContainerLifecycle {
	return &internalContainerLifecycleImpl{cm.cpuManager, cm.memoryManager, cm.topologyManager}
}

// Create a cgroup container manager.
func createManager(containerName string) (cgroups.Manager, error) {
	cg := &configs.Cgroup{
		Parent: "/",
		Name:   containerName,
		Resources: &configs.Resources{
			SkipDevices: true,
		},
	}

	if cgroups.IsCgroup2UnifiedMode() {
		return cgroupfs2.NewManager(cg, "", false)

	}
	return cgroupfs.NewManager(cg, nil, false), nil
}

type KernelTunableBehavior string

const (
	KernelTunableWarn   KernelTunableBehavior = "warn"
	KernelTunableError  KernelTunableBehavior = "error"
	KernelTunableModify KernelTunableBehavior = "modify"
)

// setupKernelTunables validates kernel tunable flags are set as expected
// depending upon the specified option, it will either warn, error, or modify the kernel tunable flags
func setupKernelTunables(option KernelTunableBehavior) error {
	desiredState := map[string]int{
		utilsysctl.VMOvercommitMemory: utilsysctl.VMOvercommitMemoryAlways,
		utilsysctl.VMPanicOnOOM:       utilsysctl.VMPanicOnOOMInvokeOOMKiller,
		utilsysctl.KernelPanic:        utilsysctl.KernelPanicRebootTimeout,
		utilsysctl.KernelPanicOnOops:  utilsysctl.KernelPanicOnOopsAlways,
		utilsysctl.RootMaxKeys:        utilsysctl.RootMaxKeysSetting,
		utilsysctl.RootMaxBytes:       utilsysctl.RootMaxBytesSetting,
	}

	sysctl := utilsysctl.New()

	errList := []error{}
	for flag, expectedValue := range desiredState {
		val, err := sysctl.GetSysctl(flag)
		if err != nil {
			errList = append(errList, err)
			continue
		}
		if val == expectedValue {
			continue
		}

		switch option {
		case KernelTunableError:
			errList = append(errList, fmt.Errorf("invalid kernel flag: %v, expected value: %v, actual value: %v", flag, expectedValue, val))
		case KernelTunableWarn:
			klog.V(2).InfoS("Invalid kernel flag", "flag", flag, "expectedValue", expectedValue, "actualValue", val)
		case KernelTunableModify:
			klog.V(2).InfoS("Updating kernel flag", "flag", flag, "expectedValue", expectedValue, "actualValue", val)
			err = sysctl.SetSysctl(flag, expectedValue)
			if err != nil {
				if libcontaineruserns.RunningInUserNS() {
					if utilfeature.DefaultFeatureGate.Enabled(kubefeatures.KubeletInUserNamespace) {
						klog.V(2).InfoS("Updating kernel flag failed (running in UserNS, ignoring)", "flag", flag, "err", err)
						continue
					}
					klog.ErrorS(err, "Updating kernel flag failed (Hint: enable KubeletInUserNamespace feature flag to ignore the error)", "flag", flag)
				}
				errList = append(errList, err)
			}
		}
	}
	return utilerrors.NewAggregate(errList)
}

func (cm *containerManagerImpl) setupNode(activePods ActivePodsFunc) error {
	f, err := validateSystemRequirements(cm.mountUtil)
	if err != nil {
		return err
	}
	if !f.cpuHardcapping {
		cm.status.SoftRequirements = fmt.Errorf("CPU hardcapping unsupported")
	}
	b := KernelTunableModify
	if cm.GetNodeConfig().ProtectKernelDefaults {
		b = KernelTunableError
	}
	if err := setupKernelTunables(b); err != nil {
		return err
	}

	// Setup top level qos containers only if CgroupsPerQOS flag is specified as true
	if cm.NodeConfig.CgroupsPerQOS {
		if err := cm.createNodeAllocatableCgroups(); err != nil {
			return err
		}
		err = cm.qosContainerManager.Start(cm.GetNodeAllocatableAbsolute, activePods)
		if err != nil {
			return fmt.Errorf("failed to initialize top level QOS containers: %v", err)
		}
	}

	// Enforce Node Allocatable (if required)
	if err := cm.enforceNodeAllocatableCgroups(); err != nil {
		return err
	}

	systemContainers := []*systemContainer{}
	if cm.ContainerRuntime == "docker" {
		// With the docker-CRI integration, dockershim manages the cgroups
		// and oom score for the docker processes.
		// Check the cgroup for docker periodically, so kubelet can serve stats for the docker runtime.
		// TODO(KEP#866): remove special processing for CRI "docker" enablement
		cm.periodicTasks = append(cm.periodicTasks, func() {
			klog.V(4).InfoS("Adding periodic tasks for docker CRI integration")
			cont, err := getContainerNameForProcess(dockerProcessName, dockerPidFile)
			if err != nil {
				klog.ErrorS(err, "Failed to get container name for process")
				return
			}
			klog.V(2).InfoS("Discovered runtime cgroup name", "cgroupName", cont)
			cm.Lock()
			defer cm.Unlock()
			cm.RuntimeCgroupsName = cont
		})
	}

	if cm.SystemCgroupsName != "" {
		if cm.SystemCgroupsName == "/" {
			return fmt.Errorf("system container cannot be root (\"/\")")
		}
		cont, err := newSystemCgroups(cm.SystemCgroupsName)
		if err != nil {
			return err
		}
		cont.ensureStateFunc = func(manager cgroups.Manager) error {
			return ensureSystemCgroups("/", manager)
		}
		systemContainers = append(systemContainers, cont)
	}

	if cm.KubeletCgroupsName != "" {
		cont, err := newSystemCgroups(cm.KubeletCgroupsName)
		if err != nil {
			return err
		}

		cont.ensureStateFunc = func(_ cgroups.Manager) error {
			return ensureProcessInContainerWithOOMScore(os.Getpid(), qos.KubeletOOMScoreAdj, cont.manager)
		}
		systemContainers = append(systemContainers, cont)
	} else {
		cm.periodicTasks = append(cm.periodicTasks, func() {
			if err := ensureProcessInContainerWithOOMScore(os.Getpid(), qos.KubeletOOMScoreAdj, nil); err != nil {
				klog.ErrorS(err, "Failed to ensure process in container with oom score")
				return
			}
			cont, err := getContainer(os.Getpid())
			if err != nil {
				klog.ErrorS(err, "Failed to find cgroups of kubelet")
				return
			}
			cm.Lock()
			defer cm.Unlock()

			cm.KubeletCgroupsName = cont
		})
	}

	cm.systemContainers = systemContainers
	return nil
}

func getContainerNameForProcess(name, pidFile string) (string, error) {
	pids, err := getPidsForProcess(name, pidFile)
	if err != nil {
		return "", fmt.Errorf("failed to detect process id for %q - %v", name, err)
	}
	if len(pids) == 0 {
		return "", nil
	}
	cont, err := getContainer(pids[0])
	if err != nil {
		return "", err
	}
	return cont, nil
}

func (cm *containerManagerImpl) GetNodeConfig() NodeConfig {
	cm.RLock()
	defer cm.RUnlock()
	return cm.NodeConfig
}

// GetPodCgroupRoot returns the literal cgroupfs value for the cgroup containing all pods.
func (cm *containerManagerImpl) GetPodCgroupRoot() string {
	return cm.cgroupManager.Name(cm.cgroupRoot)
}

func (cm *containerManagerImpl) GetMountedSubsystems() *CgroupSubsystems {
	return cm.subsystems
}

func (cm *containerManagerImpl) GetQOSContainersInfo() QOSContainersInfo {
	return cm.qosContainerManager.GetQOSContainersInfo()
}

func (cm *containerManagerImpl) UpdateQOSCgroups() error {
	return cm.qosContainerManager.UpdateCgroups()
}

func (cm *containerManagerImpl) Status() Status {
	cm.RLock()
	defer cm.RUnlock()
	return cm.status
}

func (cm *containerManagerImpl) Start(node *v1.Node,
	activePods ActivePodsFunc,
	sourcesReady config.SourcesReady,
	podStatusProvider status.PodStatusProvider,
	runtimeService internalapi.RuntimeService) error {

	// Initialize CPU manager
	if utilfeature.DefaultFeatureGate.Enabled(kubefeatures.CPUManager) {
		containerMap, err := buildContainerMapFromRuntime(runtimeService)
		if err != nil {
			return fmt.Errorf("failed to build map of initial containers from runtime: %v", err)
		}
		err = cm.cpuManager.Start(cpumanager.ActivePodsFunc(activePods), sourcesReady, podStatusProvider, runtimeService, containerMap)
		if err != nil {
			return fmt.Errorf("start cpu manager error: %v", err)
		}
	}

	// Initialize memory manager
	if utilfeature.DefaultFeatureGate.Enabled(kubefeatures.MemoryManager) {
		containerMap, err := buildContainerMapFromRuntime(runtimeService)
		if err != nil {
			return fmt.Errorf("failed to build map of initial containers from runtime: %v", err)
		}
		err = cm.memoryManager.Start(memorymanager.ActivePodsFunc(activePods), sourcesReady, podStatusProvider, runtimeService, containerMap)
		if err != nil {
			return fmt.Errorf("start memory manager error: %v", err)
		}
	}

	// cache the node Info including resource capacity and
	// allocatable of the node
	cm.nodeInfo = node

	if utilfeature.DefaultFeatureGate.Enabled(kubefeatures.LocalStorageCapacityIsolation) {
		rootfs, err := cm.cadvisorInterface.RootFsInfo()
		if err != nil {
			return fmt.Errorf("failed to get rootfs info: %v", err)
		}
		for rName, rCap := range cadvisor.EphemeralStorageCapacityFromFsInfo(rootfs) {
			cm.capacity[rName] = rCap
		}
	}

	// Ensure that node allocatable configuration is valid.
	if err := cm.validateNodeAllocatable(); err != nil {
		return err
	}

	// Setup the node
	if err := cm.setupNode(activePods); err != nil {
		return err
	}

	// Don't run a background thread if there are no ensureStateFuncs.
	hasEnsureStateFuncs := false
	for _, cont := range cm.systemContainers {
		if cont.ensureStateFunc != nil {
			hasEnsureStateFuncs = true
			break
		}
	}
	if hasEnsureStateFuncs {
		// Run ensure state functions every minute.
		go wait.Until(func() {
			for _, cont := range cm.systemContainers {
				if cont.ensureStateFunc != nil {
					if err := cont.ensureStateFunc(cont.manager); err != nil {
						klog.InfoS("Failed to ensure state", "containerName", cont.name, "err", err)
					}
				}
			}
		}, time.Minute, wait.NeverStop)

	}

	if len(cm.periodicTasks) > 0 {
		go wait.Until(func() {
			for _, task := range cm.periodicTasks {
				if task != nil {
					task()
				}
			}
		}, 5*time.Minute, wait.NeverStop)
	}

	// Starts device manager.
	if err := cm.deviceManager.Start(devicemanager.ActivePodsFunc(activePods), sourcesReady); err != nil {
		return err
	}

	return nil
}

func (cm *containerManagerImpl) GetPluginRegistrationHandler() cache.PluginHandler {
	return cm.deviceManager.GetWatcherHandler()
}

// TODO: move the GetResources logic to PodContainerManager.
func (cm *containerManagerImpl) GetResources(pod *v1.Pod, container *v1.Container) (*kubecontainer.RunContainerOptions, error) {
	opts := &kubecontainer.RunContainerOptions{}
	// Allocate should already be called during predicateAdmitHandler.Admit(),
	// just try to fetch device runtime information from cached state here
	devOpts, err := cm.deviceManager.GetDeviceRunContainerOptions(pod, container)
	if err != nil {
		return nil, err
	} else if devOpts == nil {
		return opts, nil
	}
	opts.Devices = append(opts.Devices, devOpts.Devices...)
	opts.Mounts = append(opts.Mounts, devOpts.Mounts...)
	opts.Envs = append(opts.Envs, devOpts.Envs...)
	opts.Annotations = append(opts.Annotations, devOpts.Annotations...)
	return opts, nil
}

func (cm *containerManagerImpl) UpdatePluginResources(node *schedulerframework.NodeInfo, attrs *lifecycle.PodAdmitAttributes) error {
	return cm.deviceManager.UpdatePluginResources(node, attrs)
}

func (cm *containerManagerImpl) GetAllocateResourcesPodAdmitHandler() lifecycle.PodAdmitHandler {
	if utilfeature.DefaultFeatureGate.Enabled(kubefeatures.TopologyManager) {
		return cm.topologyManager
	}
	// TODO: we need to think about a better way to do this. This will work for
	// now so long as we have only the cpuManager and deviceManager relying on
	// allocations here. However, going forward it is not generalized enough to
	// work as we add more and more hint providers that the TopologyManager
	// needs to call Allocate() on (that may not be directly intstantiated
	// inside this component).
	return &resourceAllocator{cm.cpuManager, cm.memoryManager, cm.deviceManager}
}

type resourceAllocator struct {
	cpuManager    cpumanager.Manager
	memoryManager memorymanager.Manager
	deviceManager devicemanager.Manager
}

func (m *resourceAllocator) Admit(attrs *lifecycle.PodAdmitAttributes) lifecycle.PodAdmitResult {
	pod := attrs.Pod

	for _, container := range append(pod.Spec.InitContainers, pod.Spec.Containers...) {
		err := m.deviceManager.Allocate(pod, &container)
		if err != nil {
			return admission.GetPodAdmitResult(err)
		}

		if m.cpuManager != nil {
			err = m.cpuManager.Allocate(pod, &container)
			if err != nil {
				return admission.GetPodAdmitResult(err)
			}
		}

		if m.memoryManager != nil {
			err = m.memoryManager.Allocate(pod, &container)
			if err != nil {
				return admission.GetPodAdmitResult(err)
			}
		}
	}

	return admission.GetPodAdmitResult(nil)
}

func (cm *containerManagerImpl) SystemCgroupsLimit() v1.ResourceList {
	cpuLimit := int64(0)

	// Sum up resources of all external containers.
	for _, cont := range cm.systemContainers {
		cpuLimit += cont.cpuMillicores
	}

	return v1.ResourceList{
		v1.ResourceCPU: *resource.NewMilliQuantity(
			cpuLimit,
			resource.DecimalSI),
	}
}

func buildContainerMapFromRuntime(runtimeService internalapi.RuntimeService) (containermap.ContainerMap, error) {
	podSandboxMap := make(map[string]string)
	podSandboxList, _ := runtimeService.ListPodSandbox(nil)
	for _, p := range podSandboxList {
		podSandboxMap[p.Id] = p.Metadata.Uid
	}

	containerMap := containermap.NewContainerMap()
	containerList, _ := runtimeService.ListContainers(nil)
	for _, c := range containerList {
		if _, exists := podSandboxMap[c.PodSandboxId]; !exists {
			return nil, fmt.Errorf("no PodsandBox found with Id '%s'", c.PodSandboxId)
		}
		containerMap.Add(podSandboxMap[c.PodSandboxId], c.Metadata.Name, c.Id)
	}

	return containerMap, nil
}

func isProcessRunningInHost(pid int) (bool, error) {
	// Get init pid namespace.
	initPidNs, err := os.Readlink("/proc/1/ns/pid")
	if err != nil {
		return false, fmt.Errorf("failed to find pid namespace of init process")
	}
	klog.V(10).InfoS("Found init PID namespace", "namespace", initPidNs)
	processPidNs, err := os.Readlink(fmt.Sprintf("/proc/%d/ns/pid", pid))
	if err != nil {
		return false, fmt.Errorf("failed to find pid namespace of process %q", pid)
	}
	klog.V(10).InfoS("Process info", "pid", pid, "namespace", processPidNs)
	return initPidNs == processPidNs, nil
}

func getPidFromPidFile(pidFile string) (int, error) {
	file, err := os.Open(pidFile)
	if err != nil {
		return 0, fmt.Errorf("error opening pid file %s: %v", pidFile, err)
	}
	defer file.Close()

	data, err := utilio.ReadAtMost(file, maxPidFileLength)
	if err != nil {
		return 0, fmt.Errorf("error reading pid file %s: %v", pidFile, err)
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, fmt.Errorf("error parsing %s as a number: %v", string(data), err)
	}

	return pid, nil
}

func getPidsForProcess(name, pidFile string) ([]int, error) {
	if len(pidFile) == 0 {
		return procfs.PidOf(name)
	}

	pid, err := getPidFromPidFile(pidFile)
	if err == nil {
		return []int{pid}, nil
	}

	// Try to lookup pid by process name
	pids, err2 := procfs.PidOf(name)
	if err2 == nil {
		return pids, nil
	}

	// Return error from getPidFromPidFile since that should have worked
	// and is the real source of the problem.
	klog.V(4).InfoS("Unable to get pid from file", "path", pidFile, "err", err)
	return []int{}, err
}

// Ensures that the Docker daemon is in the desired container.
// Temporarily export the function to be used by dockershim.
// TODO(yujuhong): Move this function to dockershim once kubelet migrates to
// dockershim as the default.
func EnsureDockerInContainer(dockerAPIVersion *utilversion.Version, oomScoreAdj int, manager cgroups.Manager) error {
	type process struct{ name, file string }
	dockerProcs := []process{{dockerProcessName, dockerPidFile}}
	if dockerAPIVersion.AtLeast(containerdAPIVersion) {
		// By default containerd is started separately, so there is no pid file.
		containerdPidFile := ""
		dockerProcs = append(dockerProcs, process{containerdProcessName, containerdPidFile})
	}
	var errs []error
	for _, proc := range dockerProcs {
		pids, err := getPidsForProcess(proc.name, proc.file)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to get pids for %q: %v", proc.name, err))
			continue
		}

		// Move if the pid is not already in the desired container.
		for _, pid := range pids {
			if err := ensureProcessInContainerWithOOMScore(pid, oomScoreAdj, manager); err != nil {
				errs = append(errs, fmt.Errorf("errors moving %q pid: %v", proc.name, err))
			}
		}
	}
	return utilerrors.NewAggregate(errs)
}

func ensureProcessInContainerWithOOMScore(pid int, oomScoreAdj int, manager cgroups.Manager) error {
	if runningInHost, err := isProcessRunningInHost(pid); err != nil {
		// Err on the side of caution. Avoid moving the docker daemon unless we are able to identify its context.
		return err
	} else if !runningInHost {
		// Process is running inside a container. Don't touch that.
		klog.V(2).InfoS("PID is not running in the host namespace", "pid", pid)
		return nil
	}

	var errs []error
	if manager != nil {
		cont, err := getContainer(pid)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to find container of PID %d: %v", pid, err))
		}

		name := ""
		cgroups, err := manager.GetCgroups()
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to get cgroups for %d: %v", pid, err))
		} else {
			name = cgroups.Name
		}

		if cont != name {
			err = manager.Apply(pid)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to move PID %d (in %q) to %q: %v", pid, cont, name, err))
			}
		}
	}

	// Also apply oom-score-adj to processes
	oomAdjuster := oom.NewOOMAdjuster()
	klog.V(5).InfoS("Attempting to apply oom_score_adj to process", "oomScoreAdj", oomScoreAdj, "pid", pid)
	if err := oomAdjuster.ApplyOOMScoreAdj(pid, oomScoreAdj); err != nil {
		klog.V(3).InfoS("Failed to apply oom_score_adj to process", "oomScoreAdj", oomScoreAdj, "pid", pid, "err", err)
		errs = append(errs, fmt.Errorf("failed to apply oom score %d to PID %d: %v", oomScoreAdj, pid, err))
	}
	return utilerrors.NewAggregate(errs)
}

// getContainer returns the cgroup associated with the specified pid.
// It enforces a unified hierarchy for memory and cpu cgroups.
// On systemd environments, it uses the name=systemd cgroup for the specified pid.
func getContainer(pid int) (string, error) {
	cgs, err := cgroups.ParseCgroupFile(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil {
		return "", err
	}

	if cgroups.IsCgroup2UnifiedMode() {
		c, found := cgs[""]
		if !found {
			return "", cgroups.NewNotFoundError("unified")
		}
		return c, nil
	}

	cpu, found := cgs["cpu"]
	if !found {
		return "", cgroups.NewNotFoundError("cpu")
	}
	memory, found := cgs["memory"]
	if !found {
		return "", cgroups.NewNotFoundError("memory")
	}

	// since we use this container for accounting, we need to ensure its a unified hierarchy.
	if cpu != memory {
		return "", fmt.Errorf("cpu and memory cgroup hierarchy not unified.  cpu: %s, memory: %s", cpu, memory)
	}

	// on systemd, every pid is in a unified cgroup hierarchy (name=systemd as seen in systemd-cgls)
	// cpu and memory accounting is off by default, users may choose to enable it per unit or globally.
	// users could enable CPU and memory accounting globally via /etc/systemd/system.conf (DefaultCPUAccounting=true DefaultMemoryAccounting=true).
	// users could also enable CPU and memory accounting per unit via CPUAccounting=true and MemoryAccounting=true
	// we only warn if accounting is not enabled for CPU or memory so as to not break local development flows where kubelet is launched in a terminal.
	// for example, the cgroup for the user session will be something like /user.slice/user-X.slice/session-X.scope, but the cpu and memory
	// cgroup will be the closest ancestor where accounting is performed (most likely /) on systems that launch docker containers.
	// as a result, on those systems, you will not get cpu or memory accounting statistics for kubelet.
	// in addition, you would not get memory or cpu accounting for the runtime unless accounting was enabled on its unit (or globally).
	if systemd, found := cgs["name=systemd"]; found {
		if systemd != cpu {
			klog.InfoS("CPUAccounting not enabled for process", "pid", pid)
		}
		if systemd != memory {
			klog.InfoS("MemoryAccounting not enabled for process", "pid", pid)
		}
		return systemd, nil
	}

	return cpu, nil
}

// Ensures the system container is created and all non-kernel threads and process 1
// without a container are moved to it.
//
// The reason of leaving kernel threads at root cgroup is that we don't want to tie the
// execution of these threads with to-be defined /system quota and create priority inversions.
//
func ensureSystemCgroups(rootCgroupPath string, manager cgroups.Manager) error {
	// Move non-kernel PIDs to the system container.
	// Only keep errors on latest attempt.
	var finalErr error
	for i := 0; i <= 10; i++ {
		allPids, err := cmutil.GetPids(rootCgroupPath)
		if err != nil {
			finalErr = fmt.Errorf("failed to list PIDs for root: %v", err)
			continue
		}

		// Remove kernel pids and other protected PIDs (pid 1, PIDs already in system & kubelet containers)
		pids := make([]int, 0, len(allPids))
		for _, pid := range allPids {
			if pid == 1 || isKernelPid(pid) {
				continue
			}

			pids = append(pids, pid)
		}

		// Check if we have moved all the non-kernel PIDs.
		if len(pids) == 0 {
			return nil
		}

		klog.V(3).InfoS("Moving non-kernel processes", "pids", pids)
		for _, pid := range pids {
			err := manager.Apply(pid)
			if err != nil {
				name := ""
				cgroups, err := manager.GetCgroups()
				if err == nil {
					name = cgroups.Name
				}

				finalErr = fmt.Errorf("failed to move PID %d into the system container %q: %v", pid, name, err)
			}
		}

	}

	return finalErr
}

// Determines whether the specified PID is a kernel PID.
func isKernelPid(pid int) bool {
	// Kernel threads have no associated executable.
	_, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	return err != nil && os.IsNotExist(err)
}

func (cm *containerManagerImpl) GetCapacity() v1.ResourceList {
	return cm.capacity
}

func (cm *containerManagerImpl) GetDevicePluginResourceCapacity() (v1.ResourceList, v1.ResourceList, []string) {
	return cm.deviceManager.GetCapacity()
}

func (cm *containerManagerImpl) GetDevices(podUID, containerName string) []*podresourcesapi.ContainerDevices {
	return containerDevicesFromResourceDeviceInstances(cm.deviceManager.GetDevices(podUID, containerName))
}

func (cm *containerManagerImpl) GetAllocatableDevices() []*podresourcesapi.ContainerDevices {
	return containerDevicesFromResourceDeviceInstances(cm.deviceManager.GetAllocatableDevices())
}

func (cm *containerManagerImpl) GetCPUs(podUID, containerName string) []int64 {
	if cm.cpuManager != nil {
		return cm.cpuManager.GetCPUs(podUID, containerName).ToSliceNoSortInt64()
	}
	return []int64{}
}

func (cm *containerManagerImpl) GetAllocatableCPUs() []int64 {
	if cm.cpuManager != nil {
		return cm.cpuManager.GetAllocatableCPUs().ToSliceNoSortInt64()
	}
	return []int64{}
}

func (cm *containerManagerImpl) GetMemory(podUID, containerName string) []*podresourcesapi.ContainerMemory {
	if cm.memoryManager == nil {
		return []*podresourcesapi.ContainerMemory{}
	}

	return containerMemoryFromBlock(cm.memoryManager.GetMemory(podUID, containerName))
}

func (cm *containerManagerImpl) GetAllocatableMemory() []*podresourcesapi.ContainerMemory {
	if cm.memoryManager == nil {
		return []*podresourcesapi.ContainerMemory{}
	}

	return containerMemoryFromBlock(cm.memoryManager.GetAllocatableMemory())
}

func (cm *containerManagerImpl) ShouldResetExtendedResourceCapacity() bool {
	return cm.deviceManager.ShouldResetExtendedResourceCapacity()
}

func (cm *containerManagerImpl) UpdateAllocatedDevices() {
	cm.deviceManager.UpdateAllocatedDevices()
}

func containerMemoryFromBlock(blocks []memorymanagerstate.Block) []*podresourcesapi.ContainerMemory {
	var containerMemories []*podresourcesapi.ContainerMemory

	for _, b := range blocks {
		containerMemory := podresourcesapi.ContainerMemory{
			MemoryType: string(b.Type),
			Size_:      b.Size,
			Topology: &podresourcesapi.TopologyInfo{
				Nodes: []*podresourcesapi.NUMANode{},
			},
		}

		for _, numaNodeID := range b.NUMAAffinity {
			containerMemory.Topology.Nodes = append(containerMemory.Topology.Nodes, &podresourcesapi.NUMANode{ID: int64(numaNodeID)})
		}

		containerMemories = append(containerMemories, &containerMemory)
	}

	return containerMemories
}
