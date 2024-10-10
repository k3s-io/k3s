//go:build linux
// +build linux

package containerd

import (
	"errors"
	"io/fs"
	"os/exec"

	"github.com/k3s-io/k3s/pkg/agent/templates"
	"github.com/sirupsen/logrus"
)

// A map with string as value and `templates.ContainerdRuntimeConfig` as values.
// The key holds the name of the runtime
type runtimeConfigs map[string]templates.ContainerdRuntimeConfig

// searchForRuntimes searches for runtimes and add into foundRuntimes
// It checks the PATH for the executables
func searchForRuntimes(potentialRuntimes runtimeConfigs, foundRuntimes runtimeConfigs) {
	// Fill in the binary location with just the name of the binary,
	// and check against each of the possible locations. If a match is found,
	// set the location to the full path.
	for runtimeName, runtimeConfig := range potentialRuntimes {
		logrus.Debugf("Searching for %s container runtime", runtimeName)
		path, err := exec.LookPath(runtimeConfig.BinaryName)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				logrus.Debugf("%s container runtime not found in $PATH: %v", runtimeName, err)
			} else {
				logrus.Debugf("Error searching for %s in $PATH: %v", runtimeName, err)
			}
			continue
		}

		logrus.Infof("Found %s container runtime at %s", runtimeName, path)
		runtimeConfig.BinaryName = path
		foundRuntimes[runtimeName] = runtimeConfig
	}
}

// findContainerRuntimes is a function that searches for all the runtimes and
// return a list with all the runtimes that have been found
func findContainerRuntimes() runtimeConfigs {
	foundRuntimes := runtimeConfigs{}
	findCRunContainerRuntime(foundRuntimes)
	findNvidiaContainerRuntimes(foundRuntimes)
	findWasiRuntimes(foundRuntimes)
	return foundRuntimes
}

func findCRunContainerRuntime(foundRuntimes runtimeConfigs) {
	potentialRuntimes := runtimeConfigs{
		"crun": {
			RuntimeType: "io.containerd.runc.v2",
			BinaryName:  "crun",
		},
	}

	searchForRuntimes(potentialRuntimes, foundRuntimes)
}

func findNvidiaContainerRuntimes(foundRuntimes runtimeConfigs) {
	potentialRuntimes := runtimeConfigs{
		"nvidia": {
			RuntimeType: "io.containerd.runc.v2",
			BinaryName:  "nvidia-container-runtime",
		},
		"nvidia-experimental": {
			RuntimeType: "io.containerd.runc.v2",
			BinaryName:  "nvidia-container-runtime-experimental",
		},
                "nvidia-cdi": {
                        RuntimeType: "io.containerd.runc.v2",
                        BinaryName:  "nvidia-container-runtime.cdi",
                },
	}

	searchForRuntimes(potentialRuntimes, foundRuntimes)
}

func findWasiRuntimes(foundRuntimes runtimeConfigs) {
	potentialRuntimes := runtimeConfigs{
		"lunatic": {
			RuntimeType: "io.containerd.lunatic.v1",
			BinaryName:  "containerd-shim-lunatic-v1",
		},
		"slight": {
			RuntimeType: "io.containerd.slight.v1",
			BinaryName:  "containerd-shim-slight-v1",
		},
		"spin": {
			RuntimeType: "io.containerd.spin.v2",
			BinaryName:  "containerd-shim-spin-v2",
		},
		"wws": {
			RuntimeType: "io.containerd.wws.v1",
			BinaryName:  "containerd-shim-wws-v1",
		},
		"wasmedge": {
			RuntimeType: "io.containerd.wasmedge.v1",
			BinaryName:  "containerd-shim-wasmedge-v1",
		},
		"wasmer": {
			RuntimeType: "io.containerd.wasmer.v1",
			BinaryName:  "containerd-shim-wasmer-v1",
		},
		"wasmtime": {
			RuntimeType: "io.containerd.wasmtime.v1",
			BinaryName:  "containerd-shim-wasmtime-v1",
		},
	}

	searchForRuntimes(potentialRuntimes, foundRuntimes)
}
