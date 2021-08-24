// +build !dockerless

/*
Copyright 2016 The Kubernetes Authors.

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

package dockershim

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	dockerfilters "github.com/docker/docker/api/types/filters"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/kubelet/checkpointmanager"
	"k8s.io/kubernetes/pkg/kubelet/checkpointmanager/errors"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/kubelet/dockershim/libdocker"
	"k8s.io/kubernetes/pkg/kubelet/types"
)

const (
	defaultSandboxImage = "k8s.gcr.io/pause:3.5"

	// Various default sandbox resources requests/limits.
	defaultSandboxCPUshares int64 = 2

	// defaultSandboxOOMAdj is the oom score adjustment for the docker
	// sandbox container. Using this OOM adj makes it very unlikely, but not
	// impossible, that the defaultSandox will experience an oom kill. -998
	// is chosen to signify sandbox should be OOM killed before other more
	// vital processes like the docker daemon, the kubelet, etc...
	defaultSandboxOOMAdj int = -998

	// Name of the underlying container runtime
	runtimeName = "docker"
)

var (
	// Termination grace period
	defaultSandboxGracePeriod = time.Duration(10) * time.Second
)

// Returns whether the sandbox network is ready, and whether the sandbox is known
func (ds *dockerService) getNetworkReady(podSandboxID string) (bool, bool) {
	ds.networkReadyLock.Lock()
	defer ds.networkReadyLock.Unlock()
	ready, ok := ds.networkReady[podSandboxID]
	return ready, ok
}

func (ds *dockerService) setNetworkReady(podSandboxID string, ready bool) {
	ds.networkReadyLock.Lock()
	defer ds.networkReadyLock.Unlock()
	ds.networkReady[podSandboxID] = ready
}

func (ds *dockerService) clearNetworkReady(podSandboxID string) {
	ds.networkReadyLock.Lock()
	defer ds.networkReadyLock.Unlock()
	delete(ds.networkReady, podSandboxID)
}

// RunPodSandbox creates and starts a pod-level sandbox. Runtimes should ensure
// the sandbox is in ready state.
// For docker, PodSandbox is implemented by a container holding the network
// namespace for the pod.
// Note: docker doesn't use LogDirectory (yet).
func (ds *dockerService) RunPodSandbox(ctx context.Context, r *runtimeapi.RunPodSandboxRequest) (*runtimeapi.RunPodSandboxResponse, error) {
	config := r.GetConfig()

	// Step 1: Pull the image for the sandbox.
	image := defaultSandboxImage
	podSandboxImage := ds.podSandboxImage
	if len(podSandboxImage) != 0 {
		image = podSandboxImage
	}

	// NOTE: To use a custom sandbox image in a private repository, users need to configure the nodes with credentials properly.
	// see: https://kubernetes.io/docs/user-guide/images/#configuring-nodes-to-authenticate-to-a-private-registry
	// Only pull sandbox image when it's not present - v1.PullIfNotPresent.
	if err := ensureSandboxImageExists(ds.client, image); err != nil {
		return nil, err
	}

	// Step 2: Create the sandbox container.
	if r.GetRuntimeHandler() != "" && r.GetRuntimeHandler() != runtimeName {
		return nil, fmt.Errorf("RuntimeHandler %q not supported", r.GetRuntimeHandler())
	}
	createConfig, err := ds.makeSandboxDockerConfig(config, image)
	if err != nil {
		return nil, fmt.Errorf("failed to make sandbox docker config for pod %q: %v", config.Metadata.Name, err)
	}
	createResp, err := ds.client.CreateContainer(*createConfig)
	if err != nil {
		createResp, err = recoverFromCreationConflictIfNeeded(ds.client, *createConfig, err)
	}

	if err != nil || createResp == nil {
		return nil, fmt.Errorf("failed to create a sandbox for pod %q: %v", config.Metadata.Name, err)
	}
	resp := &runtimeapi.RunPodSandboxResponse{PodSandboxId: createResp.ID}

	ds.setNetworkReady(createResp.ID, false)
	defer func(e *error) {
		// Set networking ready depending on the error return of
		// the parent function
		if *e == nil {
			ds.setNetworkReady(createResp.ID, true)
		}
	}(&err)

	// Step 3: Create Sandbox Checkpoint.
	if err = ds.checkpointManager.CreateCheckpoint(createResp.ID, constructPodSandboxCheckpoint(config)); err != nil {
		return nil, err
	}

	// Step 4: Start the sandbox container.
	// Assume kubelet's garbage collector would remove the sandbox later, if
	// startContainer failed.
	err = ds.client.StartContainer(createResp.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to start sandbox container for pod %q: %v", config.Metadata.Name, err)
	}

	// Rewrite resolv.conf file generated by docker.
	// NOTE: cluster dns settings aren't passed anymore to docker api in all cases,
	// not only for pods with host network: the resolver conf will be overwritten
	// after sandbox creation to override docker's behaviour. This resolv.conf
	// file is shared by all containers of the same pod, and needs to be modified
	// only once per pod.
	if dnsConfig := config.GetDnsConfig(); dnsConfig != nil {
		containerInfo, err := ds.client.InspectContainer(createResp.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect sandbox container for pod %q: %v", config.Metadata.Name, err)
		}

		if err := rewriteResolvFile(containerInfo.ResolvConfPath, dnsConfig.Servers, dnsConfig.Searches, dnsConfig.Options); err != nil {
			return nil, fmt.Errorf("rewrite resolv.conf failed for pod %q: %v", config.Metadata.Name, err)
		}
	}

	// Do not invoke network plugins if in hostNetwork mode.
	if config.GetLinux().GetSecurityContext().GetNamespaceOptions().GetNetwork() == runtimeapi.NamespaceMode_NODE {
		return resp, nil
	}

	// Step 5: Setup networking for the sandbox.
	// All pod networking is setup by a CNI plugin discovered at startup time.
	// This plugin assigns the pod ip, sets up routes inside the sandbox,
	// creates interfaces etc. In theory, its jurisdiction ends with pod
	// sandbox networking, but it might insert iptables rules or open ports
	// on the host as well, to satisfy parts of the pod spec that aren't
	// recognized by the CNI standard yet.
	cID := kubecontainer.BuildContainerID(runtimeName, createResp.ID)
	networkOptions := make(map[string]string)
	if dnsConfig := config.GetDnsConfig(); dnsConfig != nil {
		// Build DNS options.
		dnsOption, err := json.Marshal(dnsConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal dns config for pod %q: %v", config.Metadata.Name, err)
		}
		networkOptions["dns"] = string(dnsOption)
	}
	err = ds.network.SetUpPod(config.GetMetadata().Namespace, config.GetMetadata().Name, cID, config.Annotations, networkOptions)
	if err != nil {
		errList := []error{fmt.Errorf("failed to set up sandbox container %q network for pod %q: %v", createResp.ID, config.Metadata.Name, err)}

		// Ensure network resources are cleaned up even if the plugin
		// succeeded but an error happened between that success and here.
		err = ds.network.TearDownPod(config.GetMetadata().Namespace, config.GetMetadata().Name, cID)
		if err != nil {
			errList = append(errList, fmt.Errorf("failed to clean up sandbox container %q network for pod %q: %v", createResp.ID, config.Metadata.Name, err))
		}

		err = ds.client.StopContainer(createResp.ID, defaultSandboxGracePeriod)
		if err != nil {
			errList = append(errList, fmt.Errorf("failed to stop sandbox container %q for pod %q: %v", createResp.ID, config.Metadata.Name, err))
		}

		return resp, utilerrors.NewAggregate(errList)
	}

	return resp, nil
}

// StopPodSandbox stops the sandbox. If there are any running containers in the
// sandbox, they should be force terminated.
// TODO: This function blocks sandbox teardown on networking teardown. Is it
// better to cut our losses assuming an out of band GC routine will cleanup
// after us?
func (ds *dockerService) StopPodSandbox(ctx context.Context, r *runtimeapi.StopPodSandboxRequest) (*runtimeapi.StopPodSandboxResponse, error) {
	var namespace, name string
	var hostNetwork bool

	podSandboxID := r.PodSandboxId
	resp := &runtimeapi.StopPodSandboxResponse{}

	// Try to retrieve minimal sandbox information from docker daemon or sandbox checkpoint.
	inspectResult, metadata, statusErr := ds.getPodSandboxDetails(podSandboxID)
	if statusErr == nil {
		namespace = metadata.Namespace
		name = metadata.Name
		hostNetwork = (networkNamespaceMode(inspectResult) == runtimeapi.NamespaceMode_NODE)
	} else {
		checkpoint := NewPodSandboxCheckpoint("", "", &CheckpointData{})
		checkpointErr := ds.checkpointManager.GetCheckpoint(podSandboxID, checkpoint)

		// Proceed if both sandbox container and checkpoint could not be found. This means that following
		// actions will only have sandbox ID and not have pod namespace and name information.
		// Return error if encounter any unexpected error.
		if checkpointErr != nil {
			if checkpointErr != errors.ErrCheckpointNotFound {
				err := ds.checkpointManager.RemoveCheckpoint(podSandboxID)
				if err != nil {
					klog.ErrorS(err, "Failed to delete corrupt checkpoint for sandbox", "podSandboxID", podSandboxID)
				}
			}
			if libdocker.IsContainerNotFoundError(statusErr) {
				klog.InfoS("Both sandbox container and checkpoint could not be found. Proceed without further sandbox information.", "podSandboxID", podSandboxID)
			} else {
				return nil, utilerrors.NewAggregate([]error{
					fmt.Errorf("failed to get checkpoint for sandbox %q: %v", podSandboxID, checkpointErr),
					fmt.Errorf("failed to get sandbox status: %v", statusErr)})
			}
		} else {
			_, name, namespace, _, hostNetwork = checkpoint.GetData()
		}
	}

	// WARNING: The following operations made the following assumption:
	// 1. kubelet will retry on any error returned by StopPodSandbox.
	// 2. tearing down network and stopping sandbox container can succeed in any sequence.
	// This depends on the implementation detail of network plugin and proper error handling.
	// For kubenet, if tearing down network failed and sandbox container is stopped, kubelet
	// will retry. On retry, kubenet will not be able to retrieve network namespace of the sandbox
	// since it is stopped. With empty network namespace, CNI bridge plugin will conduct best
	// effort clean up and will not return error.
	errList := []error{}
	ready, ok := ds.getNetworkReady(podSandboxID)
	if !hostNetwork && (ready || !ok) {
		// Only tear down the pod network if we haven't done so already
		cID := kubecontainer.BuildContainerID(runtimeName, podSandboxID)
		err := ds.network.TearDownPod(namespace, name, cID)
		if err == nil {
			ds.setNetworkReady(podSandboxID, false)
		} else {
			errList = append(errList, err)
		}
	}
	if err := ds.client.StopContainer(podSandboxID, defaultSandboxGracePeriod); err != nil {
		// Do not return error if the container does not exist
		if !libdocker.IsContainerNotFoundError(err) {
			klog.ErrorS(err, "Failed to stop sandbox", "podSandboxID", podSandboxID)
			errList = append(errList, err)
		} else {
			// remove the checkpoint for any sandbox that is not found in the runtime
			ds.checkpointManager.RemoveCheckpoint(podSandboxID)
		}
	}

	if len(errList) == 0 {
		return resp, nil
	}

	// TODO: Stop all running containers in the sandbox.
	return nil, utilerrors.NewAggregate(errList)
}

// RemovePodSandbox removes the sandbox. If there are running containers in the
// sandbox, they should be forcibly removed.
func (ds *dockerService) RemovePodSandbox(ctx context.Context, r *runtimeapi.RemovePodSandboxRequest) (*runtimeapi.RemovePodSandboxResponse, error) {
	podSandboxID := r.PodSandboxId
	var errs []error

	opts := dockertypes.ContainerListOptions{All: true}

	opts.Filters = dockerfilters.NewArgs()
	f := newDockerFilter(&opts.Filters)
	f.AddLabel(sandboxIDLabelKey, podSandboxID)

	containers, err := ds.client.ListContainers(opts)
	if err != nil {
		errs = append(errs, err)
	}

	// Remove all containers in the sandbox.
	for i := range containers {
		if _, err := ds.RemoveContainer(ctx, &runtimeapi.RemoveContainerRequest{ContainerId: containers[i].ID}); err != nil && !libdocker.IsContainerNotFoundError(err) {
			errs = append(errs, err)
		}
	}

	// Remove the sandbox container.
	err = ds.client.RemoveContainer(podSandboxID, dockertypes.ContainerRemoveOptions{RemoveVolumes: true, Force: true})
	if err == nil || libdocker.IsContainerNotFoundError(err) {
		// Only clear network ready when the sandbox has actually been
		// removed from docker or doesn't exist
		ds.clearNetworkReady(podSandboxID)
	} else {
		errs = append(errs, err)
	}

	// Remove the checkpoint of the sandbox.
	if err := ds.checkpointManager.RemoveCheckpoint(podSandboxID); err != nil {
		errs = append(errs, err)
	}
	if len(errs) == 0 {
		return &runtimeapi.RemovePodSandboxResponse{}, nil
	}
	return nil, utilerrors.NewAggregate(errs)
}

// getIPsFromPlugin interrogates the network plugin for sandbox IPs.
func (ds *dockerService) getIPsFromPlugin(sandbox *dockertypes.ContainerJSON) ([]string, error) {
	metadata, err := parseSandboxName(sandbox.Name)
	if err != nil {
		return nil, err
	}
	msg := fmt.Sprintf("Couldn't find network status for %s/%s through plugin", metadata.Namespace, metadata.Name)
	cID := kubecontainer.BuildContainerID(runtimeName, sandbox.ID)
	networkStatus, err := ds.network.GetPodNetworkStatus(metadata.Namespace, metadata.Name, cID)
	if err != nil {
		return nil, err
	}
	if networkStatus == nil {
		return nil, fmt.Errorf("%v: invalid network status for", msg)
	}

	ips := make([]string, 0)
	for _, ip := range networkStatus.IPs {
		ips = append(ips, ip.String())
	}
	// if we don't have any ip in our list then cni is using classic primary IP only
	if len(ips) == 0 {
		ips = append(ips, networkStatus.IP.String())
	}
	return ips, nil
}

// getIPs returns the ip given the output of `docker inspect` on a pod sandbox,
// first interrogating any registered plugins, then simply trusting the ip
// in the sandbox itself. We look for an ipv4 address before ipv6.
func (ds *dockerService) getIPs(podSandboxID string, sandbox *dockertypes.ContainerJSON) []string {
	if sandbox.NetworkSettings == nil {
		return nil
	}
	if networkNamespaceMode(sandbox) == runtimeapi.NamespaceMode_NODE {
		// For sandboxes using host network, the shim is not responsible for
		// reporting the IP.
		return nil
	}

	// Don't bother getting IP if the pod is known and networking isn't ready
	ready, ok := ds.getNetworkReady(podSandboxID)
	if ok && !ready {
		return nil
	}

	ips, err := ds.getIPsFromPlugin(sandbox)
	if err == nil {
		return ips
	}

	ips = make([]string, 0)
	// TODO: trusting the docker ip is not a great idea. However docker uses
	// eth0 by default and so does CNI, so if we find a docker IP here, we
	// conclude that the plugin must have failed setup, or forgotten its ip.
	// This is not a sensible assumption for plugins across the board, but if
	// a plugin doesn't want this behavior, it can throw an error.
	if sandbox.NetworkSettings.IPAddress != "" {
		ips = append(ips, sandbox.NetworkSettings.IPAddress)
	}
	if sandbox.NetworkSettings.GlobalIPv6Address != "" {
		ips = append(ips, sandbox.NetworkSettings.GlobalIPv6Address)
	}

	// If all else fails, warn but don't return an error, as pod status
	// should generally not return anything except fatal errors
	// FIXME: handle network errors by restarting the pod somehow?
	klog.InfoS("Failed to read pod IP from plugin/docker", "err", err)
	return ips
}

// Returns the inspect container response, the sandbox metadata, and network namespace mode
func (ds *dockerService) getPodSandboxDetails(podSandboxID string) (*dockertypes.ContainerJSON, *runtimeapi.PodSandboxMetadata, error) {
	resp, err := ds.client.InspectContainer(podSandboxID)
	if err != nil {
		return nil, nil, err
	}

	metadata, err := parseSandboxName(resp.Name)
	if err != nil {
		return nil, nil, err
	}

	return resp, metadata, nil
}

// PodSandboxStatus returns the status of the PodSandbox.
func (ds *dockerService) PodSandboxStatus(ctx context.Context, req *runtimeapi.PodSandboxStatusRequest) (*runtimeapi.PodSandboxStatusResponse, error) {
	podSandboxID := req.PodSandboxId

	r, metadata, err := ds.getPodSandboxDetails(podSandboxID)
	if err != nil {
		return nil, err
	}

	// Parse the timestamps.
	createdAt, _, _, err := getContainerTimestamps(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp for container %q: %v", podSandboxID, err)
	}
	ct := createdAt.UnixNano()

	// Translate container to sandbox state.
	state := runtimeapi.PodSandboxState_SANDBOX_NOTREADY
	if r.State.Running {
		state = runtimeapi.PodSandboxState_SANDBOX_READY
	}

	var ips []string
	// TODO: Remove this when sandbox is available on windows
	// This is a workaround for windows, where sandbox is not in use, and pod IP is determined through containers belonging to the Pod.
	if ips = ds.determinePodIPBySandboxID(podSandboxID); len(ips) == 0 {
		ips = ds.getIPs(podSandboxID, r)
	}

	// ip is primary ips
	// ips is all other ips
	ip := ""
	if len(ips) != 0 {
		ip = ips[0]
		ips = ips[1:]
	}

	labels, annotations := extractLabels(r.Config.Labels)
	status := &runtimeapi.PodSandboxStatus{
		Id:          r.ID,
		State:       state,
		CreatedAt:   ct,
		Metadata:    metadata,
		Labels:      labels,
		Annotations: annotations,
		Network: &runtimeapi.PodSandboxNetworkStatus{
			Ip: ip,
		},
		Linux: &runtimeapi.LinuxPodSandboxStatus{
			Namespaces: &runtimeapi.Namespace{
				Options: &runtimeapi.NamespaceOption{
					Network: networkNamespaceMode(r),
					Pid:     pidNamespaceMode(r),
					Ipc:     ipcNamespaceMode(r),
				},
			},
		},
	}
	// add additional IPs
	additionalPodIPs := make([]*runtimeapi.PodIP, 0, len(ips))
	for _, ip := range ips {
		additionalPodIPs = append(additionalPodIPs, &runtimeapi.PodIP{
			Ip: ip,
		})
	}
	status.Network.AdditionalIps = additionalPodIPs
	return &runtimeapi.PodSandboxStatusResponse{Status: status}, nil
}

// ListPodSandbox returns a list of Sandbox.
func (ds *dockerService) ListPodSandbox(_ context.Context, r *runtimeapi.ListPodSandboxRequest) (*runtimeapi.ListPodSandboxResponse, error) {
	filter := r.GetFilter()

	// By default, list all containers whether they are running or not.
	opts := dockertypes.ContainerListOptions{All: true}
	filterOutReadySandboxes := false

	opts.Filters = dockerfilters.NewArgs()
	f := newDockerFilter(&opts.Filters)
	// Add filter to select only sandbox containers.
	f.AddLabel(containerTypeLabelKey, containerTypeLabelSandbox)

	if filter != nil {
		if filter.Id != "" {
			f.Add("id", filter.Id)
		}
		if filter.State != nil {
			if filter.GetState().State == runtimeapi.PodSandboxState_SANDBOX_READY {
				// Only list running containers.
				opts.All = false
			} else {
				// runtimeapi.PodSandboxState_SANDBOX_NOTREADY can mean the
				// container is in any of the non-running state (e.g., created,
				// exited). We can't tell docker to filter out running
				// containers directly, so we'll need to filter them out
				// ourselves after getting the results.
				filterOutReadySandboxes = true
			}
		}

		if filter.LabelSelector != nil {
			for k, v := range filter.LabelSelector {
				f.AddLabel(k, v)
			}
		}
	}

	// Make sure we get the list of checkpoints first so that we don't include
	// new PodSandboxes that are being created right now.
	var err error
	checkpoints := []string{}
	if filter == nil {
		checkpoints, err = ds.checkpointManager.ListCheckpoints()
		if err != nil {
			klog.ErrorS(err, "Failed to list checkpoints")
		}
	}

	containers, err := ds.client.ListContainers(opts)
	if err != nil {
		return nil, err
	}

	// Convert docker containers to runtime api sandboxes.
	result := []*runtimeapi.PodSandbox{}
	// using map as set
	sandboxIDs := make(map[string]bool)
	for i := range containers {
		c := containers[i]
		converted, err := containerToRuntimeAPISandbox(&c)
		if err != nil {
			klog.V(4).InfoS("Unable to convert docker to runtime API sandbox", "containerName", c.Names, "err", err)
			continue
		}
		if filterOutReadySandboxes && converted.State == runtimeapi.PodSandboxState_SANDBOX_READY {
			continue
		}
		sandboxIDs[converted.Id] = true
		result = append(result, converted)
	}

	// Include sandbox that could only be found with its checkpoint if no filter is applied
	// These PodSandbox will only include PodSandboxID, Name, Namespace.
	// These PodSandbox will be in PodSandboxState_SANDBOX_NOTREADY state.
	for _, id := range checkpoints {
		if _, ok := sandboxIDs[id]; ok {
			continue
		}
		checkpoint := NewPodSandboxCheckpoint("", "", &CheckpointData{})
		err := ds.checkpointManager.GetCheckpoint(id, checkpoint)
		if err != nil {
			klog.ErrorS(err, "Failed to retrieve checkpoint for sandbox", "sandboxID", id)
			if err == errors.ErrCorruptCheckpoint {
				err = ds.checkpointManager.RemoveCheckpoint(id)
				if err != nil {
					klog.ErrorS(err, "Failed to delete corrupt checkpoint for sandbox", "sandboxID", id)
				}
			}
			continue
		}
		result = append(result, checkpointToRuntimeAPISandbox(id, checkpoint))
	}

	return &runtimeapi.ListPodSandboxResponse{Items: result}, nil
}

// applySandboxLinuxOptions applies LinuxPodSandboxConfig to dockercontainer.HostConfig and dockercontainer.ContainerCreateConfig.
func (ds *dockerService) applySandboxLinuxOptions(hc *dockercontainer.HostConfig, lc *runtimeapi.LinuxPodSandboxConfig, createConfig *dockertypes.ContainerCreateConfig, image string, separator rune) error {
	if lc == nil {
		return nil
	}
	// Apply security context.
	if err := applySandboxSecurityContext(lc, createConfig.Config, hc, ds.network, separator); err != nil {
		return err
	}

	// Set sysctls.
	hc.Sysctls = lc.Sysctls
	return nil
}

func (ds *dockerService) applySandboxResources(hc *dockercontainer.HostConfig, lc *runtimeapi.LinuxPodSandboxConfig) error {
	hc.Resources = dockercontainer.Resources{
		MemorySwap: DefaultMemorySwap(),
		CPUShares:  defaultSandboxCPUshares,
		// Use docker's default cpu quota/period.
	}

	if lc != nil {
		// Apply Cgroup options.
		cgroupParent, err := ds.GenerateExpectedCgroupParent(lc.CgroupParent)
		if err != nil {
			return err
		}
		hc.CgroupParent = cgroupParent
	}
	return nil
}

// makeSandboxDockerConfig returns dockertypes.ContainerCreateConfig based on runtimeapi.PodSandboxConfig.
func (ds *dockerService) makeSandboxDockerConfig(c *runtimeapi.PodSandboxConfig, image string) (*dockertypes.ContainerCreateConfig, error) {
	// Merge annotations and labels because docker supports only labels.
	labels := makeLabels(c.GetLabels(), c.GetAnnotations())
	// Apply a label to distinguish sandboxes from regular containers.
	labels[containerTypeLabelKey] = containerTypeLabelSandbox
	// Apply a container name label for infra container. This is used in summary v1.
	// TODO(random-liu): Deprecate this label once container metrics is directly got from CRI.
	labels[types.KubernetesContainerNameLabel] = sandboxContainerName

	hc := &dockercontainer.HostConfig{
		IpcMode: dockercontainer.IpcMode("shareable"),
	}
	createConfig := &dockertypes.ContainerCreateConfig{
		Name: makeSandboxName(c),
		Config: &dockercontainer.Config{
			Hostname: c.Hostname,
			// TODO: Handle environment variables.
			Image:  image,
			Labels: labels,
		},
		HostConfig: hc,
	}

	// Apply linux-specific options.
	if err := ds.applySandboxLinuxOptions(hc, c.GetLinux(), createConfig, image, securityOptSeparator); err != nil {
		return nil, err
	}

	// Set port mappings.
	exposedPorts, portBindings := makePortsAndBindings(c.GetPortMappings())
	createConfig.Config.ExposedPorts = exposedPorts
	hc.PortBindings = portBindings

	hc.OomScoreAdj = defaultSandboxOOMAdj

	// Apply resource options.
	if err := ds.applySandboxResources(hc, c.GetLinux()); err != nil {
		return nil, err
	}

	// Set security options.
	securityOpts := ds.getSandBoxSecurityOpts(securityOptSeparator)
	hc.SecurityOpt = append(hc.SecurityOpt, securityOpts...)

	return createConfig, nil
}

// networkNamespaceMode returns the network runtimeapi.NamespaceMode for this container.
// Supports: POD, NODE
func networkNamespaceMode(container *dockertypes.ContainerJSON) runtimeapi.NamespaceMode {
	if container != nil && container.HostConfig != nil && string(container.HostConfig.NetworkMode) == namespaceModeHost {
		return runtimeapi.NamespaceMode_NODE
	}
	return runtimeapi.NamespaceMode_POD
}

// pidNamespaceMode returns the PID runtimeapi.NamespaceMode for this container.
// Supports: CONTAINER, NODE
// TODO(verb): add support for POD PID namespace sharing
func pidNamespaceMode(container *dockertypes.ContainerJSON) runtimeapi.NamespaceMode {
	if container != nil && container.HostConfig != nil && string(container.HostConfig.PidMode) == namespaceModeHost {
		return runtimeapi.NamespaceMode_NODE
	}
	return runtimeapi.NamespaceMode_CONTAINER
}

// ipcNamespaceMode returns the IPC runtimeapi.NamespaceMode for this container.
// Supports: POD, NODE
func ipcNamespaceMode(container *dockertypes.ContainerJSON) runtimeapi.NamespaceMode {
	if container != nil && container.HostConfig != nil && string(container.HostConfig.IpcMode) == namespaceModeHost {
		return runtimeapi.NamespaceMode_NODE
	}
	return runtimeapi.NamespaceMode_POD
}

func constructPodSandboxCheckpoint(config *runtimeapi.PodSandboxConfig) checkpointmanager.Checkpoint {
	data := CheckpointData{}
	for _, pm := range config.GetPortMappings() {
		proto := toCheckpointProtocol(pm.Protocol)
		data.PortMappings = append(data.PortMappings, &PortMapping{
			HostPort:      &pm.HostPort,
			ContainerPort: &pm.ContainerPort,
			Protocol:      &proto,
			HostIP:        pm.HostIp,
		})
	}
	if config.GetLinux().GetSecurityContext().GetNamespaceOptions().GetNetwork() == runtimeapi.NamespaceMode_NODE {
		data.HostNetwork = true
	}
	return NewPodSandboxCheckpoint(config.Metadata.Namespace, config.Metadata.Name, &data)
}

func toCheckpointProtocol(protocol runtimeapi.Protocol) Protocol {
	switch protocol {
	case runtimeapi.Protocol_TCP:
		return protocolTCP
	case runtimeapi.Protocol_UDP:
		return protocolUDP
	case runtimeapi.Protocol_SCTP:
		return protocolSCTP
	}
	klog.InfoS("Unknown protocol, defaulting to TCP", "protocol", protocol)
	return protocolTCP
}

// rewriteResolvFile rewrites resolv.conf file generated by docker.
func rewriteResolvFile(resolvFilePath string, dns []string, dnsSearch []string, dnsOptions []string) error {
	if len(resolvFilePath) == 0 {
		klog.ErrorS(nil, "ResolvConfPath is empty.")
		return nil
	}

	if _, err := os.Stat(resolvFilePath); os.IsNotExist(err) {
		return fmt.Errorf("ResolvConfPath %q does not exist", resolvFilePath)
	}

	var resolvFileContent []string
	for _, srv := range dns {
		resolvFileContent = append(resolvFileContent, "nameserver "+srv)
	}

	if len(dnsSearch) > 0 {
		resolvFileContent = append(resolvFileContent, "search "+strings.Join(dnsSearch, " "))
	}

	if len(dnsOptions) > 0 {
		resolvFileContent = append(resolvFileContent, "options "+strings.Join(dnsOptions, " "))
	}

	if len(resolvFileContent) > 0 {
		resolvFileContentStr := strings.Join(resolvFileContent, "\n")
		resolvFileContentStr += "\n"

		klog.V(4).InfoS("Will attempt to re-write config file", "path", resolvFilePath, "fileContent", resolvFileContent)
		if err := rewriteFile(resolvFilePath, resolvFileContentStr); err != nil {
			klog.ErrorS(err, "Resolv.conf could not be updated")
			return err
		}
	}

	return nil
}

func rewriteFile(filePath, stringToWrite string) error {
	f, err := os.OpenFile(filePath, os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(stringToWrite)
	return err
}
