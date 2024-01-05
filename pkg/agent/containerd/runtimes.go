//go:build linux
// +build linux

package containerd

import (
	"errors"
	"io/fs"
	"path/filepath"

	"github.com/k3s-io/k3s/pkg/agent/templates"
	"github.com/sirupsen/logrus"
)

// A map with string as value and `templates.ContainerdRuntimeConfig` as values.
// The key holds the name of the runtime
type runtimeConfigs map[string]templates.ContainerdRuntimeConfig

// searchForRuntimes searches for runtimes and add into foundRuntimes
// It checks install locations provided via potentitalRuntimes variable.
// The binaries are searched at the locations specivied by locationsToCheck.
// The given fs.FS should represent the filesystem root directory to search in.
func searchForRuntimes(root fs.FS, potentialRuntimes runtimeConfigs, locationsToCheck []string, foundRuntimes runtimeConfigs) {
	// Check these locations in order. The GPU operator's installation should
	// take precedence over the package manager's installation.

	// Fill in the binary location with just the name of the binary,
	// and check against each of the possible locations. If a match is found,
	// set the location to the full path.
	for runtimeName, runtimeConfig := range potentialRuntimes {
		for _, location := range locationsToCheck {
			binaryPath := filepath.Join(location, runtimeConfig.BinaryName)
			logrus.Debugf("Searching for %s container runtime at /%s", runtimeName, binaryPath)
			if info, err := fs.Stat(root, binaryPath); err == nil {
				if info.IsDir() {
					logrus.Debugf("Found %s container runtime at /%s, but it is a directory. Skipping.", runtimeName, binaryPath)
					continue
				}
				runtimeConfig.BinaryName = filepath.Join("/", binaryPath)
				logrus.Infof("Found %s container runtime at %s", runtimeName, runtimeConfig.BinaryName)
				foundRuntimes[runtimeName] = runtimeConfig
				break
			} else {
				if errors.Is(err, fs.ErrNotExist) {
					logrus.Debugf("%s container runtime not found at /%s", runtimeName, binaryPath)
				} else {
					logrus.Errorf("Error searching for %s container runtime at /%s: %v", runtimeName, binaryPath, err)
				}
			}
		}
	}
}

// findContainerRuntimes is a function that searches for all the runtimes and
// return a list with all the runtimes that have been found
func findContainerRuntimes(root fs.FS) runtimeConfigs {
	foundRuntimes := runtimeConfigs{}
	findCRunContainerRuntime(root, foundRuntimes)
	findNvidiaContainerRuntimes(root, foundRuntimes)
	findWasiRuntimes(root, foundRuntimes)
	return foundRuntimes
}

// findCRunContainerRuntime finds if crun is available in the system and adds to foundRuntimes
func findCRunContainerRuntime(root fs.FS, foundRuntimes runtimeConfigs) {
	// Check these locations in order.
	locationsToCheck := []string{
		"usr/sbin",       // Path when installing via package manager
		"usr/bin",        // Path when installing via package manager
		"usr/local/bin",  // Path when installing manually
		"usr/local/sbin", // Path when installing manually
	}

	// Fill in the binary location with just the name of the binary,
	// and check against each of the possible locations. If a match is found,
	// set the location to the full path.
	potentialRuntimes := runtimeConfigs{
		"crun": {
			RuntimeType: "io.containerd.runc.v2",
			BinaryName:  "crun",
		},
	}

	searchForRuntimes(root, potentialRuntimes, locationsToCheck, foundRuntimes)
}

// findNvidiaContainerRuntimes finds the nvidia runtimes that are are available on the system
// and adds to foundRuntimes. It checks install locations used by the nvidia
// gpu operator and by system package managers. The gpu operator installation
// takes precedence over the system package manager installation.
// The given fs.FS should represent the filesystem root directory to search in.
func findNvidiaContainerRuntimes(root fs.FS, foundRuntimes runtimeConfigs) {
	// Check these locations in order. The GPU operator's installation should
	// take precedence over the package manager's installation.
	locationsToCheck := []string{
		"usr/local/nvidia/toolkit", // Path when installing via GPU Operator
		"usr/bin",                  // Path when installing via package manager
	}

	// Fill in the binary location with just the name of the binary,
	// and check against each of the possible locations. If a match is found,
	// set the location to the full path.
	potentialRuntimes := runtimeConfigs{
		"nvidia": {
			RuntimeType: "io.containerd.runc.v2",
			BinaryName:  "nvidia-container-runtime",
		},
		"nvidia-experimental": {
			RuntimeType: "io.containerd.runc.v2",
			BinaryName:  "nvidia-container-runtime-experimental",
		},
	}
	searchForRuntimes(root, potentialRuntimes, locationsToCheck, foundRuntimes)
}

// findWasiRuntimes finds the WebAssembly (WASI) container runtimes that
// are available on the system and adds to foundRuntimes. It checks install locations used by the kwasm
// operator and by system package managers. The kwasm operator installation
// takes precedence over the system package manager installation.
// The given fs.FS should represent the filesystem root directory to search in.
func findWasiRuntimes(root fs.FS, foundRuntimes runtimeConfigs) {
	// Check these locations in order.
	locationsToCheck := []string{
		"opt/kwasm/bin", // Path when installing via kwasm Operator
		"usr/bin",       // Path when installing via package manager
		"usr/sbin",      // Path when installing via package manager
	}

	// Fill in the binary location with just the name of the binary,
	// and check against each of the possible locations. If a match is found,
	// set the location to the full path.
	potentialRuntimes := runtimeConfigs{
		"lunatic": {
			RuntimeType: "io.containerd.lunatic.v2",
			BinaryName:  "containerd-shim-lunatic-v1",
		},
		"slight": {
			RuntimeType: "io.containerd.slight.v2",
			BinaryName:  "containerd-shim-slight-v1",
		},
		"spin": {
			RuntimeType: "io.containerd.spin.v2",
			BinaryName:  "containerd-shim-spin-v1",
		},
		"wws": {
			RuntimeType: "io.containerd.wws.v2",
			BinaryName:  "containerd-shim-wws-v1",
		},
		"wasmedge": {
			RuntimeType: "io.containerd.wasmedge.v2",
			BinaryName:  "containerd-shim-wasmedge-v1",
		},
		"wasmer": {
			RuntimeType: "io.containerd.wasmer.v2",
			BinaryName:  "containerd-shim-wasmer-v1",
		},
		"wasmtime": {
			RuntimeType: "io.containerd.wasmtime.v2",
			BinaryName:  "containerd-shim-wasmtime-v1",
		},
	}
	searchForRuntimes(root, potentialRuntimes, locationsToCheck, foundRuntimes)
}
