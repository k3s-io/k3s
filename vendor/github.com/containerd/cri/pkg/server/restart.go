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

package server

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/containerd/containerd"
	containerdio "github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	containerdimages "github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/typeurl"
	"github.com/docker/docker/pkg/system"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	runtime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

	"github.com/containerd/cri/pkg/netns"
	cio "github.com/containerd/cri/pkg/server/io"
	containerstore "github.com/containerd/cri/pkg/store/container"
	sandboxstore "github.com/containerd/cri/pkg/store/sandbox"
)

// NOTE: The recovery logic has following assumption: when the cri plugin is down:
// 1) Files (e.g. root directory, netns) and checkpoint maintained by the plugin MUST NOT be
// touched. Or else, recovery logic for those containers/sandboxes may return error.
// 2) Containerd containers may be deleted, but SHOULD NOT be added. Or else, recovery logic
// for the newly added container/sandbox will return error, because there is no corresponding root
// directory created.
// 3) Containerd container tasks may exit or be stoppped, deleted. Even though current logic could
// tolerant tasks being created or started, we prefer that not to happen.

// recover recovers system state from containerd and status checkpoint.
func (c *criService) recover(ctx context.Context) error {
	// Recover all sandboxes.
	sandboxes, err := c.client.Containers(ctx, filterLabel(containerKindLabel, containerKindSandbox))
	if err != nil {
		return errors.Wrap(err, "failed to list sandbox containers")
	}
	for _, sandbox := range sandboxes {
		sb, err := loadSandbox(ctx, sandbox)
		if err != nil {
			logrus.WithError(err).Errorf("Failed to load sandbox %q", sandbox.ID())
			continue
		}
		logrus.Debugf("Loaded sandbox %+v", sb)
		if err := c.sandboxStore.Add(sb); err != nil {
			return errors.Wrapf(err, "failed to add sandbox %q to store", sandbox.ID())
		}
		if err := c.sandboxNameIndex.Reserve(sb.Name, sb.ID); err != nil {
			return errors.Wrapf(err, "failed to reserve sandbox name %q", sb.Name)
		}
	}

	// Recover all containers.
	containers, err := c.client.Containers(ctx, filterLabel(containerKindLabel, containerKindContainer))
	if err != nil {
		return errors.Wrap(err, "failed to list containers")
	}
	for _, container := range containers {
		cntr, err := c.loadContainer(ctx, container)
		if err != nil {
			logrus.WithError(err).Errorf("Failed to load container %q", container.ID())
			continue
		}
		logrus.Debugf("Loaded container %+v", cntr)
		if err := c.containerStore.Add(cntr); err != nil {
			return errors.Wrapf(err, "failed to add container %q to store", container.ID())
		}
		if err := c.containerNameIndex.Reserve(cntr.Name, cntr.ID); err != nil {
			return errors.Wrapf(err, "failed to reserve container name %q", cntr.Name)
		}
	}

	// Recover all images.
	cImages, err := c.client.ListImages(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list images")
	}
	c.loadImages(ctx, cImages)

	// It's possible that containerd containers are deleted unexpectedly. In that case,
	// we can't even get metadata, we should cleanup orphaned sandbox/container directories
	// with best effort.

	// Cleanup orphaned sandbox and container directories without corresponding containerd container.
	for _, cleanup := range []struct {
		cntrs  []containerd.Container
		base   string
		errMsg string
	}{
		{
			cntrs:  sandboxes,
			base:   filepath.Join(c.config.RootDir, sandboxesDir),
			errMsg: "failed to cleanup orphaned sandbox directories",
		},
		{
			cntrs:  sandboxes,
			base:   filepath.Join(c.config.StateDir, sandboxesDir),
			errMsg: "failed to cleanup orphaned volatile sandbox directories",
		},
		{
			cntrs:  containers,
			base:   filepath.Join(c.config.RootDir, containersDir),
			errMsg: "failed to cleanup orphaned container directories",
		},
		{
			cntrs:  containers,
			base:   filepath.Join(c.config.StateDir, containersDir),
			errMsg: "failed to cleanup orphaned volatile container directories",
		},
	} {
		if err := cleanupOrphanedIDDirs(cleanup.cntrs, cleanup.base); err != nil {
			return errors.Wrap(err, cleanup.errMsg)
		}
	}
	return nil
}

// loadContainerTimeout is the default timeout for loading a container/sandbox.
// One container/sandbox hangs (e.g. containerd#2438) should not affect other
// containers/sandboxes.
// Most CRI container/sandbox related operations are per container, the ones
// which handle multiple containers at a time are:
// * ListPodSandboxes: Don't talk with containerd services.
// * ListContainers: Don't talk with containerd services.
// * ListContainerStats: Not in critical code path, a default timeout will
// be applied at CRI level.
// * Recovery logic: We should set a time for each container/sandbox recovery.
// * Event montior: We should set a timeout for each container/sandbox event handling.
const loadContainerTimeout = 10 * time.Second

// loadContainer loads container from containerd and status checkpoint.
func (c *criService) loadContainer(ctx context.Context, cntr containerd.Container) (containerstore.Container, error) {
	ctx, cancel := context.WithTimeout(ctx, loadContainerTimeout)
	defer cancel()
	id := cntr.ID()
	containerDir := c.getContainerRootDir(id)
	volatileContainerDir := c.getVolatileContainerRootDir(id)
	var container containerstore.Container
	// Load container metadata.
	exts, err := cntr.Extensions(ctx)
	if err != nil {
		return container, errors.Wrap(err, "failed to get container extensions")
	}
	ext, ok := exts[containerMetadataExtension]
	if !ok {
		return container, errors.Errorf("metadata extension %q not found", containerMetadataExtension)
	}
	data, err := typeurl.UnmarshalAny(&ext)
	if err != nil {
		return container, errors.Wrapf(err, "failed to unmarshal metadata extension %q", ext)
	}
	meta := data.(*containerstore.Metadata)

	// Load status from checkpoint.
	status, err := containerstore.LoadStatus(containerDir, id)
	if err != nil {
		logrus.WithError(err).Warnf("Failed to load container status for %q", id)
		status = unknownContainerStatus()
	}

	var containerIO *cio.ContainerIO
	err = func() error {
		// Load up-to-date status from containerd.
		t, err := cntr.Task(ctx, func(fifos *containerdio.FIFOSet) (_ containerdio.IO, err error) {
			stdoutWC, stderrWC, err := c.createContainerLoggers(meta.LogPath, meta.Config.GetTty())
			if err != nil {
				return nil, err
			}
			defer func() {
				if err != nil {
					if stdoutWC != nil {
						stdoutWC.Close()
					}
					if stderrWC != nil {
						stderrWC.Close()
					}
				}
			}()
			containerIO, err = cio.NewContainerIO(id,
				cio.WithFIFOs(fifos),
			)
			if err != nil {
				return nil, err
			}
			containerIO.AddOutput("log", stdoutWC, stderrWC)
			containerIO.Pipe()
			return containerIO, nil
		})
		if err != nil && !errdefs.IsNotFound(err) {
			return errors.Wrap(err, "failed to load task")
		}
		var s containerd.Status
		var notFound bool
		if errdefs.IsNotFound(err) {
			// Task is not found.
			notFound = true
		} else {
			// Task is found. Get task status.
			s, err = t.Status(ctx)
			if err != nil {
				// It's still possible that task is deleted during this window.
				if !errdefs.IsNotFound(err) {
					return errors.Wrap(err, "failed to get task status")
				}
				notFound = true
			}
		}
		if notFound {
			// Task is not created or has been deleted, use the checkpointed status
			// to generate container status.
			switch status.State() {
			case runtime.ContainerState_CONTAINER_CREATED:
				// NOTE: Another possibility is that we've tried to start the container, but
				// containerd got restarted during that. In that case, we still
				// treat the container as `CREATED`.
				containerIO, err = cio.NewContainerIO(id,
					cio.WithNewFIFOs(volatileContainerDir, meta.Config.GetTty(), meta.Config.GetStdin()),
				)
				if err != nil {
					return errors.Wrap(err, "failed to create container io")
				}
			case runtime.ContainerState_CONTAINER_RUNNING:
				// Container was in running state, but its task has been deleted,
				// set unknown exited state. Container io is not needed in this case.
				status.FinishedAt = time.Now().UnixNano()
				status.ExitCode = unknownExitCode
				status.Reason = unknownExitReason
			default:
				// Container is in exited/unknown state, return the status as it is.
			}
		} else {
			// Task status is found. Update container status based on the up-to-date task status.
			switch s.Status {
			case containerd.Created:
				// Task has been created, but not started yet. This could only happen if containerd
				// gets restarted during container start.
				// Container must be in `CREATED` state.
				if _, err := t.Delete(ctx, containerd.WithProcessKill); err != nil && !errdefs.IsNotFound(err) {
					return errors.Wrap(err, "failed to delete task")
				}
				if status.State() != runtime.ContainerState_CONTAINER_CREATED {
					return errors.Errorf("unexpected container state for created task: %q", status.State())
				}
			case containerd.Running:
				// Task is running. Container must be in `RUNNING` state, based on our assuption that
				// "task should not be started when containerd is down".
				switch status.State() {
				case runtime.ContainerState_CONTAINER_EXITED:
					return errors.Errorf("unexpected container state for running task: %q", status.State())
				case runtime.ContainerState_CONTAINER_RUNNING:
				default:
					// This may happen if containerd gets restarted after task is started, but
					// before status is checkpointed.
					status.StartedAt = time.Now().UnixNano()
					status.Pid = t.Pid()
				}
			case containerd.Stopped:
				// Task is stopped. Updata status and delete the task.
				if _, err := t.Delete(ctx, containerd.WithProcessKill); err != nil && !errdefs.IsNotFound(err) {
					return errors.Wrap(err, "failed to delete task")
				}
				status.FinishedAt = s.ExitTime.UnixNano()
				status.ExitCode = int32(s.ExitStatus)
			default:
				return errors.Errorf("unexpected task status %q", s.Status)
			}
		}
		return nil
	}()
	if err != nil {
		logrus.WithError(err).Errorf("Failed to load container status for %q", id)
		status = unknownContainerStatus()
	}
	opts := []containerstore.Opts{
		containerstore.WithStatus(status, containerDir),
		containerstore.WithContainer(cntr),
	}
	// containerIO could be nil for container in unknown state.
	if containerIO != nil {
		opts = append(opts, containerstore.WithContainerIO(containerIO))
	}
	return containerstore.NewContainer(*meta, opts...)
}

// loadSandbox loads sandbox from containerd.
func loadSandbox(ctx context.Context, cntr containerd.Container) (sandboxstore.Sandbox, error) {
	ctx, cancel := context.WithTimeout(ctx, loadContainerTimeout)
	defer cancel()
	var sandbox sandboxstore.Sandbox
	// Load sandbox metadata.
	exts, err := cntr.Extensions(ctx)
	if err != nil {
		return sandbox, errors.Wrap(err, "failed to get sandbox container extensions")
	}
	ext, ok := exts[sandboxMetadataExtension]
	if !ok {
		return sandbox, errors.Errorf("metadata extension %q not found", sandboxMetadataExtension)
	}
	data, err := typeurl.UnmarshalAny(&ext)
	if err != nil {
		return sandbox, errors.Wrapf(err, "failed to unmarshal metadata extension %q", ext)
	}
	meta := data.(*sandboxstore.Metadata)

	s, err := func() (sandboxstore.Status, error) {
		status := unknownSandboxStatus()
		// Load sandbox created timestamp.
		info, err := cntr.Info(ctx)
		if err != nil {
			return status, errors.Wrap(err, "failed to get sandbox container info")
		}
		status.CreatedAt = info.CreatedAt

		// Load sandbox state.
		t, err := cntr.Task(ctx, nil)
		if err != nil && !errdefs.IsNotFound(err) {
			return status, errors.Wrap(err, "failed to load task")
		}
		var taskStatus containerd.Status
		var notFound bool
		if errdefs.IsNotFound(err) {
			// Task is not found.
			notFound = true
		} else {
			// Task is found. Get task status.
			taskStatus, err = t.Status(ctx)
			if err != nil {
				// It's still possible that task is deleted during this window.
				if !errdefs.IsNotFound(err) {
					return status, errors.Wrap(err, "failed to get task status")
				}
				notFound = true
			}
		}
		if notFound {
			// Task does not exist, set sandbox state as NOTREADY.
			status.State = sandboxstore.StateNotReady
		} else {
			if taskStatus.Status == containerd.Running {
				// Task is running, set sandbox state as READY.
				status.State = sandboxstore.StateReady
				status.Pid = t.Pid()
			} else {
				// Task is not running. Delete the task and set sandbox state as NOTREADY.
				if _, err := t.Delete(ctx, containerd.WithProcessKill); err != nil && !errdefs.IsNotFound(err) {
					return status, errors.Wrap(err, "failed to delete task")
				}
				status.State = sandboxstore.StateNotReady
			}
		}
		return status, nil
	}()
	if err != nil {
		logrus.WithError(err).Errorf("Failed to load sandbox status for %q", cntr.ID())
	}

	sandbox = sandboxstore.NewSandbox(*meta, s)
	sandbox.Container = cntr

	// Load network namespace.
	if meta.Config.GetLinux().GetSecurityContext().GetNamespaceOptions().GetNetwork() == runtime.NamespaceMode_NODE {
		// Don't need to load netns for host network sandbox.
		return sandbox, nil
	}
	sandbox.NetNS = netns.LoadNetNS(meta.NetNSPath)

	// It doesn't matter whether task is running or not. If it is running, sandbox
	// status will be `READY`; if it is not running, sandbox status will be `NOT_READY`,
	// kubelet will stop the sandbox which will properly cleanup everything.
	return sandbox, nil
}

// loadImages loads images from containerd.
func (c *criService) loadImages(ctx context.Context, cImages []containerd.Image) {
	snapshotter := c.config.ContainerdConfig.Snapshotter
	for _, i := range cImages {
		ok, _, _, _, err := containerdimages.Check(ctx, i.ContentStore(), i.Target(), platforms.Default())
		if err != nil {
			logrus.WithError(err).Errorf("Failed to check image content readiness for %q", i.Name())
			continue
		}
		if !ok {
			logrus.Warnf("The image content readiness for %q is not ok", i.Name())
			continue
		}
		// Checking existence of top-level snapshot for each image being recovered.
		unpacked, err := i.IsUnpacked(ctx, snapshotter)
		if err != nil {
			logrus.WithError(err).Warnf("Failed to check whether image is unpacked for image %s", i.Name())
			continue
		}
		if !unpacked {
			logrus.Warnf("The image %s is not unpacked.", i.Name())
			// TODO(random-liu): Consider whether we should try unpack here.
		}
		if err := c.updateImage(ctx, i.Name()); err != nil {
			logrus.WithError(err).Warnf("Failed to update reference for image %q", i.Name())
			continue
		}
		logrus.Debugf("Loaded image %q", i.Name())
	}
}

func cleanupOrphanedIDDirs(cntrs []containerd.Container, base string) error {
	// Cleanup orphaned id directories.
	dirs, err := ioutil.ReadDir(base)
	if err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to read base directory")
	}
	idsMap := make(map[string]containerd.Container)
	for _, cntr := range cntrs {
		idsMap[cntr.ID()] = cntr
	}
	for _, d := range dirs {
		if !d.IsDir() {
			logrus.Warnf("Invalid file %q found in base directory %q", d.Name(), base)
			continue
		}
		if _, ok := idsMap[d.Name()]; ok {
			// Do not remove id directory if corresponding container is found.
			continue
		}
		dir := filepath.Join(base, d.Name())
		if err := system.EnsureRemoveAll(dir); err != nil {
			logrus.WithError(err).Warnf("Failed to remove id directory %q", dir)
		} else {
			logrus.Debugf("Cleanup orphaned id directory %q", dir)
		}
	}
	return nil
}
