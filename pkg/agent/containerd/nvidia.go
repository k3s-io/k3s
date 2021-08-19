// +build linux

package containerd

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/rancher/k3s/pkg/agent/templates"
)

// findNvidiaContainerRuntimes returns a list of nvidia container runtimes that
// are available on the system. It checks install locations used by the nvidia
// gpu operator and by system package managers. The gpu operator installation
// takes precedence over the system package manager installation.
// The given fs.FS should represent the filesystem root directory. If nil,
// os.DirFS("/") is used.
func findNvidiaContainerRuntimes(fsys fs.FS) []templates.ContainerdRuntimeConfig {
	rootFS := fsys
	if rootFS == nil {
		rootFS = os.DirFS("/")
	}

	// Check these locations in order. The GPU operator's installation should
	// take precedence over the package manager's installation.
	locationsToCheck := []string{
		"usr/local/nvidia/toolkit", // Path when installing via GPU Operator
		"usr/bin",                  // Path when installing via package manager
	}

	// Fill in the binary location with just the name of the binary,
	// and check against each of the possible locations. If a match is found,
	// set the location to the full path.
	potentialRuntimes := []templates.ContainerdRuntimeConfig{
		{
			Name:        "nvidia",
			RuntimeType: "io.containerd.runc.v2",
			BinaryName:  "nvidia-container-runtime",
		},
		{
			Name:        "nvidia-experimental",
			RuntimeType: "io.containerd.runc.v2",
			BinaryName:  "nvidia-container-runtime-experimental",
		},
	}
	foundRuntimes := []templates.ContainerdRuntimeConfig{}
RuntimeLoop:
	for _, runtime := range potentialRuntimes {
		for _, location := range locationsToCheck {
			binaryPath := filepath.Join(location, runtime.BinaryName)
			if info, err := fs.Stat(rootFS, binaryPath); err == nil && !info.IsDir() {
				runtime.BinaryName = filepath.Join("/", binaryPath)
				foundRuntimes = append(foundRuntimes, runtime)
				// Skip to the next runtime to avoid duplicates.
				continue RuntimeLoop
			}
		}
	}
	return foundRuntimes
}
