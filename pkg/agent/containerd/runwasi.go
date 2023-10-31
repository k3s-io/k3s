//go:build linux
// +build linux

package containerd

import (
	"io/fs"

	"github.com/k3s-io/k3s/pkg/agent/templates"
)

// findWasiRuntimes returns a list of WebAssembly (WASI) container runtimes that
// are available on the system. It checks install locations used by the kwasm
// operator and by system package managers. The kwasm operator installation
// takes precedence over the system package manager installation.
// The given fs.FS should represent the filesystem root directory to search in.
func findWasiRuntimes(root fs.FS) map[string]templates.ContainerdRuntimeConfig {
	// Check these locations in order. The GPU operator's installation should
	// take precedence over the package manager's installation.
	locationsToCheck := []string{
		"opt/kwasm/bin", // Path when installing via kwasm Operator
		"usr/bin",       // Path when installing via package manager
		"usr/sbin",      // Path when installing via package manager
	}

	// Fill in the binary location with just the name of the binary,
	// and check against each of the possible locations. If a match is found,
	// set the location to the full path.
	potentialRuntimes := map[string]templates.ContainerdRuntimeConfig{
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
	return findContainerRuntimes(root, potentialRuntimes, locationsToCheck)
}
