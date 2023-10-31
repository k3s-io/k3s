//go:build linux
// +build linux

package containerd

import (
	"io/fs"

	"github.com/k3s-io/k3s/pkg/agent/templates"
)

// findNvidiaContainerRuntimes returns a list of nvidia container runtimes that
// are available on the system. It checks install locations used by the nvidia
// gpu operator and by system package managers. The gpu operator installation
// takes precedence over the system package manager installation.
// The given fs.FS should represent the filesystem root directory to search in.
func findNvidiaContainerRuntimes(root fs.FS) map[string]templates.ContainerdRuntimeConfig {
	// Check these locations in order. The GPU operator's installation should
	// take precedence over the package manager's installation.
	locationsToCheck := []string{
		"usr/local/nvidia/toolkit", // Path when installing via GPU Operator
		"usr/bin",                  // Path when installing via package manager
	}

	// Fill in the binary location with just the name of the binary,
	// and check against each of the possible locations. If a match is found,
	// set the location to the full path.
	potentialRuntimes := map[string]templates.ContainerdRuntimeConfig{
		"nvidia": {
			RuntimeType: "io.containerd.runc.v2",
			BinaryName:  "nvidia-container-runtime",
		},
		"nvidia-experimental": {
			RuntimeType: "io.containerd.runc.v2",
			BinaryName:  "nvidia-container-runtime-experimental",
		},
	}
	return findContainerRuntimes(root, potentialRuntimes, locationsToCheck)
}
