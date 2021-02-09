/*
   Copyright The containerd Authors.

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

package server

import (
	"encoding/json"
	"math"
	goruntime "runtime"
	"strings"

	"github.com/containerd/containerd"
	containerdio "github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	cni "github.com/containerd/go-cni"
	"github.com/containerd/typeurl"
	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	"github.com/containerd/cri/pkg/annotations"
	criconfig "github.com/containerd/cri/pkg/config"
	customopts "github.com/containerd/cri/pkg/containerd/opts"
	ctrdutil "github.com/containerd/cri/pkg/containerd/util"
	"github.com/containerd/cri/pkg/netns"
	"github.com/containerd/cri/pkg/server/bandwidth"
	sandboxstore "github.com/containerd/cri/pkg/store/sandbox"
	"github.com/containerd/cri/pkg/util"
	selinux "github.com/opencontainers/selinux/go-selinux"
)

func init() {
	typeurl.Register(&sandboxstore.Metadata{},
		"github.com/containerd/cri/pkg/store/sandbox", "Metadata")
}

// RunPodSandbox creates and starts a pod-level sandbox. Runtimes should ensure
// the sandbox is in ready state.
func (c *criService) RunPodSandbox(ctx context.Context, r *runtime.RunPodSandboxRequest) (_ *runtime.RunPodSandboxResponse, retErr error) {
	config := r.GetConfig()
	log.G(ctx).Debugf("Sandbox config %+v", config)

	// Generate unique id and name for the sandbox and reserve the name.
	id := util.GenerateID()
	metadata := config.GetMetadata()
	if metadata == nil {
		return nil, errors.New("sandbox config must include metadata")
	}
	name := makeSandboxName(metadata)
	log.G(ctx).Debugf("Generated id %q for sandbox %q", id, name)
	// Reserve the sandbox name to avoid concurrent `RunPodSandbox` request starting the
	// same sandbox.
	if err := c.sandboxNameIndex.Reserve(name, id); err != nil {
		return nil, errors.Wrapf(err, "failed to reserve sandbox name %q", name)
	}
	defer func() {
		// Release the name if the function returns with an error.
		if retErr != nil {
			c.sandboxNameIndex.ReleaseByName(name)
		}
	}()

	// Create initial internal sandbox object.
	sandbox := sandboxstore.NewSandbox(
		sandboxstore.Metadata{
			ID:             id,
			Name:           name,
			Config:         config,
			RuntimeHandler: r.GetRuntimeHandler(),
		},
		sandboxstore.Status{
			State: sandboxstore.StateUnknown,
		},
	)

	// Ensure sandbox container image snapshot.
	image, err := c.ensureImageExists(ctx, c.config.SandboxImage, config)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get sandbox image %q", c.config.SandboxImage)
	}
	containerdImage, err := c.toContainerdImage(ctx, *image)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get image from containerd %q", image.ID)
	}

	ociRuntime, err := c.getSandboxRuntime(config, r.GetRuntimeHandler())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get sandbox runtime")
	}
	log.G(ctx).Debugf("Use OCI %+v for sandbox %q", ociRuntime, id)

	podNetwork := true
	// Pod network is always needed on windows.
	if goruntime.GOOS != "windows" &&
		config.GetLinux().GetSecurityContext().GetNamespaceOptions().GetNetwork() == runtime.NamespaceMode_NODE {
		// Pod network is not needed on linux with host network.
		podNetwork = false
	}
	if podNetwork {
		// If it is not in host network namespace then create a namespace and set the sandbox
		// handle. NetNSPath in sandbox metadata and NetNS is non empty only for non host network
		// namespaces. If the pod is in host network namespace then both are empty and should not
		// be used.
		sandbox.NetNS, err = netns.NewNetNS()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create network namespace for sandbox %q", id)
		}
		sandbox.NetNSPath = sandbox.NetNS.GetPath()
		defer func() {
			if retErr != nil {
				// Teardown network if an error is returned.
				if err := c.teardownPodNetwork(ctx, sandbox); err != nil {
					log.G(ctx).WithError(err).Errorf("Failed to destroy network for sandbox %q", id)
				}

				if err := sandbox.NetNS.Remove(); err != nil {
					log.G(ctx).WithError(err).Errorf("Failed to remove network namespace %s for sandbox %q", sandbox.NetNSPath, id)
				}
				sandbox.NetNSPath = ""
			}
		}()

		// Setup network for sandbox.
		// Certain VM based solutions like clear containers (Issue containerd/cri-containerd#524)
		// rely on the assumption that CRI shim will not be querying the network namespace to check the
		// network states such as IP.
		// In future runtime implementation should avoid relying on CRI shim implementation details.
		// In this case however caching the IP will add a subtle performance enhancement by avoiding
		// calls to network namespace of the pod to query the IP of the veth interface on every
		// SandboxStatus request.
		if err := c.setupPodNetwork(ctx, &sandbox); err != nil {
			return nil, errors.Wrapf(err, "failed to setup network for sandbox %q", id)
		}
	}

	// Create sandbox container.
	// NOTE: sandboxContainerSpec SHOULD NOT have side
	// effect, e.g. accessing/creating files, so that we can test
	// it safely.
	spec, err := c.sandboxContainerSpec(id, config, &image.ImageSpec.Config, sandbox.NetNSPath, ociRuntime.PodAnnotations)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate sandbox container spec")
	}
	log.G(ctx).Debugf("Sandbox container %q spec: %#+v", id, spew.NewFormatter(spec))
	sandbox.ProcessLabel = spec.Process.SelinuxLabel
	defer func() {
		if retErr != nil {
			selinux.ReleaseLabel(sandbox.ProcessLabel)
		}
	}()

	if config.GetLinux().GetSecurityContext().GetPrivileged() {
		// If privileged don't set selinux label, but we still record the MCS label so that
		// the unused label can be freed later.
		spec.Process.SelinuxLabel = ""
	}

	// Generate spec options that will be applied to the spec later.
	specOpts, err := c.sandboxContainerSpecOpts(config, &image.ImageSpec.Config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate sanbdox container spec options")
	}

	sandboxLabels := buildLabels(config.Labels, containerKindSandbox)

	runtimeOpts, err := generateRuntimeOptions(ociRuntime, c.config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate runtime options")
	}
	opts := []containerd.NewContainerOpts{
		containerd.WithSnapshotter(c.config.ContainerdConfig.Snapshotter),
		customopts.WithNewSnapshot(id, containerdImage),
		containerd.WithSpec(spec, specOpts...),
		containerd.WithContainerLabels(sandboxLabels),
		containerd.WithContainerExtension(sandboxMetadataExtension, &sandbox.Metadata),
		containerd.WithRuntime(ociRuntime.Type, runtimeOpts)}

	container, err := c.client.NewContainer(ctx, id, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create containerd container")
	}
	defer func() {
		if retErr != nil {
			deferCtx, deferCancel := ctrdutil.DeferContext()
			defer deferCancel()
			if err := container.Delete(deferCtx, containerd.WithSnapshotCleanup); err != nil {
				log.G(ctx).WithError(err).Errorf("Failed to delete containerd container %q", id)
			}
		}
	}()

	// Create sandbox container root directories.
	sandboxRootDir := c.getSandboxRootDir(id)
	if err := c.os.MkdirAll(sandboxRootDir, 0755); err != nil {
		return nil, errors.Wrapf(err, "failed to create sandbox root directory %q",
			sandboxRootDir)
	}
	defer func() {
		if retErr != nil {
			// Cleanup the sandbox root directory.
			if err := c.os.RemoveAll(sandboxRootDir); err != nil {
				log.G(ctx).WithError(err).Errorf("Failed to remove sandbox root directory %q",
					sandboxRootDir)
			}
		}
	}()
	volatileSandboxRootDir := c.getVolatileSandboxRootDir(id)
	if err := c.os.MkdirAll(volatileSandboxRootDir, 0755); err != nil {
		return nil, errors.Wrapf(err, "failed to create volatile sandbox root directory %q",
			volatileSandboxRootDir)
	}
	defer func() {
		if retErr != nil {
			// Cleanup the volatile sandbox root directory.
			if err := c.os.RemoveAll(volatileSandboxRootDir); err != nil {
				log.G(ctx).WithError(err).Errorf("Failed to remove volatile sandbox root directory %q",
					volatileSandboxRootDir)
			}
		}
	}()

	// Setup files required for the sandbox.
	if err = c.setupSandboxFiles(id, config); err != nil {
		return nil, errors.Wrapf(err, "failed to setup sandbox files")
	}
	defer func() {
		if retErr != nil {
			if err = c.cleanupSandboxFiles(id, config); err != nil {
				log.G(ctx).WithError(err).Errorf("Failed to cleanup sandbox files in %q",
					sandboxRootDir)
			}
		}
	}()

	// Update sandbox created timestamp.
	info, err := container.Info(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get sandbox container info")
	}

	// Create sandbox task in containerd.
	log.G(ctx).Tracef("Create sandbox container (id=%q, name=%q).",
		id, name)

	taskOpts := c.taskOpts(ociRuntime.Type)
	// We don't need stdio for sandbox container.
	task, err := container.NewTask(ctx, containerdio.NullIO, taskOpts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create containerd task")
	}
	defer func() {
		if retErr != nil {
			deferCtx, deferCancel := ctrdutil.DeferContext()
			defer deferCancel()
			// Cleanup the sandbox container if an error is returned.
			if _, err := task.Delete(deferCtx, containerd.WithProcessKill); err != nil && !errdefs.IsNotFound(err) {
				log.G(ctx).WithError(err).Errorf("Failed to delete sandbox container %q", id)
			}
		}
	}()

	// wait is a long running background request, no timeout needed.
	exitCh, err := task.Wait(ctrdutil.NamespacedContext())
	if err != nil {
		return nil, errors.Wrap(err, "failed to wait for sandbox container task")
	}

	if err := task.Start(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to start sandbox container task %q", id)
	}

	if err := sandbox.Status.Update(func(status sandboxstore.Status) (sandboxstore.Status, error) {
		// Set the pod sandbox as ready after successfully start sandbox container.
		status.Pid = task.Pid()
		status.State = sandboxstore.StateReady
		status.CreatedAt = info.CreatedAt
		return status, nil
	}); err != nil {
		return nil, errors.Wrap(err, "failed to update sandbox status")
	}

	// Add sandbox into sandbox store in INIT state.
	sandbox.Container = container

	if err := c.sandboxStore.Add(sandbox); err != nil {
		return nil, errors.Wrapf(err, "failed to add sandbox %+v into store", sandbox)
	}

	// start the monitor after adding sandbox into the store, this ensures
	// that sandbox is in the store, when event monitor receives the TaskExit event.
	//
	// TaskOOM from containerd may come before sandbox is added to store,
	// but we don't care about sandbox TaskOOM right now, so it is fine.
	c.eventMonitor.startExitMonitor(context.Background(), id, task.Pid(), exitCh)

	return &runtime.RunPodSandboxResponse{PodSandboxId: id}, nil
}

// setupPodNetwork setups up the network for a pod
func (c *criService) setupPodNetwork(ctx context.Context, sandbox *sandboxstore.Sandbox) error {
	var (
		id     = sandbox.ID
		config = sandbox.Config
		path   = sandbox.NetNSPath
	)
	if c.netPlugin == nil {
		return errors.New("cni config not initialized")
	}

	opts, err := cniNamespaceOpts(id, config)
	if err != nil {
		return errors.Wrap(err, "get cni namespace options")
	}

	result, err := c.netPlugin.Setup(ctx, id, path, opts...)
	if err != nil {
		return err
	}
	logDebugCNIResult(ctx, id, result)
	// Check if the default interface has IP config
	if configs, ok := result.Interfaces[defaultIfName]; ok && len(configs.IPConfigs) > 0 {
		sandbox.IP, sandbox.AdditionalIPs = selectPodIPs(configs.IPConfigs)
		sandbox.CNIResult = result
		return nil
	}
	return errors.Errorf("failed to find network info for sandbox %q", id)
}

// cniNamespaceOpts get CNI namespace options from sandbox config.
func cniNamespaceOpts(id string, config *runtime.PodSandboxConfig) ([]cni.NamespaceOpts, error) {
	opts := []cni.NamespaceOpts{
		cni.WithLabels(toCNILabels(id, config)),
	}

	portMappings := toCNIPortMappings(config.GetPortMappings())
	if len(portMappings) > 0 {
		opts = append(opts, cni.WithCapabilityPortMap(portMappings))
	}

	// Will return an error if the bandwidth limitation has the wrong unit
	// or an unreasonable value see validateBandwidthIsReasonable()
	bandWidth, err := toCNIBandWidth(config.Annotations)
	if err != nil {
		return nil, err
	}
	if bandWidth != nil {
		opts = append(opts, cni.WithCapabilityBandWidth(*bandWidth))
	}

	dns := toCNIDNS(config.GetDnsConfig())
	if dns != nil {
		opts = append(opts, cni.WithCapabilityDNS(*dns))
	}

	return opts, nil
}

// toCNILabels adds pod metadata into CNI labels.
func toCNILabels(id string, config *runtime.PodSandboxConfig) map[string]string {
	return map[string]string{
		"K8S_POD_NAMESPACE":          config.GetMetadata().GetNamespace(),
		"K8S_POD_NAME":               config.GetMetadata().GetName(),
		"K8S_POD_INFRA_CONTAINER_ID": id,
		"IgnoreUnknown":              "1",
	}
}

// toCNIBandWidth converts CRI annotations to CNI bandwidth.
func toCNIBandWidth(annotations map[string]string) (*cni.BandWidth, error) {
	ingress, egress, err := bandwidth.ExtractPodBandwidthResources(annotations)
	if err != nil {
		return nil, errors.Wrap(err, "reading pod bandwidth annotations")
	}

	if ingress == nil && egress == nil {
		return nil, nil
	}

	bandWidth := &cni.BandWidth{}

	if ingress != nil {
		bandWidth.IngressRate = uint64(ingress.Value())
		bandWidth.IngressBurst = math.MaxUint32
	}

	if egress != nil {
		bandWidth.EgressRate = uint64(egress.Value())
		bandWidth.EgressBurst = math.MaxUint32
	}

	return bandWidth, nil
}

// toCNIPortMappings converts CRI port mappings to CNI.
func toCNIPortMappings(criPortMappings []*runtime.PortMapping) []cni.PortMapping {
	var portMappings []cni.PortMapping
	for _, mapping := range criPortMappings {
		if mapping.HostPort <= 0 {
			continue
		}
		portMappings = append(portMappings, cni.PortMapping{
			HostPort:      mapping.HostPort,
			ContainerPort: mapping.ContainerPort,
			Protocol:      strings.ToLower(mapping.Protocol.String()),
			HostIP:        mapping.HostIp,
		})
	}
	return portMappings
}

// toCNIDNS converts CRI DNSConfig to CNI.
func toCNIDNS(dns *runtime.DNSConfig) *cni.DNS {
	if dns == nil {
		return nil
	}
	return &cni.DNS{
		Servers:  dns.GetServers(),
		Searches: dns.GetSearches(),
		Options:  dns.GetOptions(),
	}
}

// selectPodIPs select an ip from the ip list. It prefers ipv4 more than ipv6
// and returns the additional ips
// TODO(random-liu): Revisit the ip order in the ipv6 beta stage. (cri#1278)
func selectPodIPs(ipConfigs []*cni.IPConfig) (string, []string) {
	var (
		additionalIPs []string
		ip            string
	)
	for _, c := range ipConfigs {
		if c.IP.To4() != nil && ip == "" {
			ip = c.IP.String()
		} else {
			additionalIPs = append(additionalIPs, c.IP.String())
		}
	}
	if ip != "" {
		return ip, additionalIPs
	}
	if len(ipConfigs) == 1 {
		return additionalIPs[0], nil
	}
	return additionalIPs[0], additionalIPs[1:]
}

// untrustedWorkload returns true if the sandbox contains untrusted workload.
func untrustedWorkload(config *runtime.PodSandboxConfig) bool {
	return config.GetAnnotations()[annotations.UntrustedWorkload] == "true"
}

// hostAccessingSandbox returns true if the sandbox configuration
// requires additional host access for the sandbox.
func hostAccessingSandbox(config *runtime.PodSandboxConfig) bool {
	securityContext := config.GetLinux().GetSecurityContext()

	namespaceOptions := securityContext.GetNamespaceOptions()
	if namespaceOptions.GetNetwork() == runtime.NamespaceMode_NODE ||
		namespaceOptions.GetPid() == runtime.NamespaceMode_NODE ||
		namespaceOptions.GetIpc() == runtime.NamespaceMode_NODE {
		return true
	}

	return false
}

// getSandboxRuntime returns the runtime configuration for sandbox.
// If the sandbox contains untrusted workload, runtime for untrusted workload will be returned,
// or else default runtime will be returned.
func (c *criService) getSandboxRuntime(config *runtime.PodSandboxConfig, runtimeHandler string) (criconfig.Runtime, error) {
	if untrustedWorkload(config) {
		// If the untrusted annotation is provided, runtimeHandler MUST be empty.
		if runtimeHandler != "" && runtimeHandler != criconfig.RuntimeUntrusted {
			return criconfig.Runtime{}, errors.New("untrusted workload with explicit runtime handler is not allowed")
		}

		//  If the untrusted workload is requesting access to the host/node, this request will fail.
		//
		//  Note: If the workload is marked untrusted but requests privileged, this can be granted, as the
		// runtime may support this.  For example, in a virtual-machine isolated runtime, privileged
		// is a supported option, granting the workload to access the entire guest VM instead of host.
		// TODO(windows): Deprecate this so that we don't need to handle it for windows.
		if hostAccessingSandbox(config) {
			return criconfig.Runtime{}, errors.New("untrusted workload with host access is not allowed")
		}

		runtimeHandler = criconfig.RuntimeUntrusted
	}

	if runtimeHandler == "" {
		runtimeHandler = c.config.ContainerdConfig.DefaultRuntimeName
	}

	handler, ok := c.config.ContainerdConfig.Runtimes[runtimeHandler]
	if !ok {
		return criconfig.Runtime{}, errors.Errorf("no runtime for %q is configured", runtimeHandler)
	}
	return handler, nil
}

func logDebugCNIResult(ctx context.Context, sandboxID string, result *cni.CNIResult) {
	if logrus.GetLevel() < logrus.DebugLevel {
		return
	}
	cniResult, err := json.Marshal(result)
	if err != nil {
		log.G(ctx).WithError(err).Errorf("Failed to marshal CNI result for sandbox %q: %v", sandboxID, err)
		return
	}
	log.G(ctx).Debugf("cni result for sandbox %q: %s", sandboxID, string(cniResult))
}
