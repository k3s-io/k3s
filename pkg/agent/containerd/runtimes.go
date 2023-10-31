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

// findContainerRuntimes returns a list of container runtimes that
// are available on the system. It checks install locations provided via
// the potentialRuntimes variable.
// The binaries are searched at the locations specivied by locationsToCheck.
// Note: check the given locations in order.
// The given fs.FS should represent the filesystem root directory to search in.
func findContainerRuntimes(root fs.FS, potentialRuntimes runtimeConfigs, locationsToCheck []string, foundRuntimes runtimeConfigs) {
	// Check these locations in order. The GPU operator's installation should
	// take precedence over the package manager's installation.

	// Fill in the binary location with just the name of the binary,
	// and check against each of the possible locations. If a match is found,
	// set the location to the full path.
RUNTIME:
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
				// Skip to the next runtime to enforce precedence.
				continue RUNTIME
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
