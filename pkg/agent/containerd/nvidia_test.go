package containerd

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/rancher/k3s/pkg/agent/templates"
	"github.com/stretchr/testify/assert"
)

func TestFindNvidiaContainerRuntimes(t *testing.T) {
	// Set up sample filesystems for testing
	executable := &fstest.MapFile{Mode: 0755}
	tests := []struct {
		FS      fs.StatFS
		Results []templates.ContainerdRuntimeConfig
	}{
		{
			FS:      fstest.MapFS{},
			Results: []templates.ContainerdRuntimeConfig{},
		},
		{
			FS: fstest.MapFS{
				"usr/bin/nvidia-container-runtime": executable,
			},
			Results: []templates.ContainerdRuntimeConfig{
				{
					Name:        "nvidia",
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/bin/nvidia-container-runtime",
				},
			},
		},
		{
			FS: fstest.MapFS{
				"usr/local/nvidia/toolkit/nvidia-container-runtime": executable,
			},
			Results: []templates.ContainerdRuntimeConfig{
				{
					Name:        "nvidia",
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/local/nvidia/toolkit/nvidia-container-runtime",
				},
			},
		},
		{
			FS: fstest.MapFS{
				"usr/bin/nvidia-container-runtime":                  executable,
				"usr/local/nvidia/toolkit/nvidia-container-runtime": executable,
			},
			Results: []templates.ContainerdRuntimeConfig{
				{
					Name:        "nvidia",
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/local/nvidia/toolkit/nvidia-container-runtime",
				},
			},
		},
		{
			FS: fstest.MapFS{
				"usr/bin/nvidia-container-runtime-experimental": executable,
			},
			Results: []templates.ContainerdRuntimeConfig{
				{
					Name:        "nvidia-experimental",
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/bin/nvidia-container-runtime-experimental",
				},
			},
		},
		{
			FS: fstest.MapFS{
				"usr/bin/nvidia-container-runtime-experimental":                  executable,
				"usr/local/nvidia/toolkit/nvidia-container-runtime-experimental": executable,
			},
			Results: []templates.ContainerdRuntimeConfig{
				{
					Name:        "nvidia-experimental",
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/local/nvidia/toolkit/nvidia-container-runtime-experimental",
				},
			},
		},
		{
			FS: fstest.MapFS{
				"usr/bin/nvidia-container-runtime-experimental": executable,
				"usr/bin/nvidia-container-runtime":              executable,
			},
			Results: []templates.ContainerdRuntimeConfig{
				{
					Name:        "nvidia",
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/bin/nvidia-container-runtime",
				},
				{
					Name:        "nvidia-experimental",
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/bin/nvidia-container-runtime-experimental",
				},
			},
		},
		{
			FS: fstest.MapFS{
				"usr/local/nvidia/toolkit/nvidia-container-runtime":              executable,
				"usr/local/nvidia/toolkit/nvidia-container-runtime-experimental": executable,
				"usr/bin/nvidia-container-runtime":                               executable,
				"usr/bin/nvidia-container-runtime-experimental":                  executable,
			},
			Results: []templates.ContainerdRuntimeConfig{
				{
					Name:        "nvidia",
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/local/nvidia/toolkit/nvidia-container-runtime",
				},
				{
					Name:        "nvidia-experimental",
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/local/nvidia/toolkit/nvidia-container-runtime-experimental",
				},
			},
		},
		{
			FS: fstest.MapFS{
				"usr/local/nvidia/toolkit/nvidia-container-runtime":              executable,
				"usr/local/nvidia/toolkit/nvidia-container-runtime-experimental": executable,
			},
			Results: []templates.ContainerdRuntimeConfig{
				{
					Name:        "nvidia",
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/local/nvidia/toolkit/nvidia-container-runtime",
				},
				{
					Name:        "nvidia-experimental",
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/local/nvidia/toolkit/nvidia-container-runtime-experimental",
				},
			},
		},
		{
			FS: fstest.MapFS{
				"usr/bin/nvidia-container-runtime":                               executable,
				"usr/bin/nvidia-container-runtime-experimental":                  executable,
				"usr/local/nvidia/toolkit/nvidia-container-runtime-experimental": executable,
			},
			Results: []templates.ContainerdRuntimeConfig{
				{
					Name:        "nvidia",
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/bin/nvidia-container-runtime",
				},
				{
					Name:        "nvidia-experimental",
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/local/nvidia/toolkit/nvidia-container-runtime-experimental",
				},
			},
		},
	}
	for i, test := range tests {
		assert.Equal(t, findNvidiaContainerRuntimes(test.FS), test.Results, "test %d", i+1)
	}
}
