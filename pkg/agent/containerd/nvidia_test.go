//go:build linux
// +build linux

package containerd

import (
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/k3s-io/k3s/pkg/agent/templates"
)

func Test_UnitFindNvidiaContainerRuntimes(t *testing.T) {
	executable := &fstest.MapFile{Mode: 0755}
	type args struct {
		root fs.FS
	}
	tests := []struct {
		name string
		args args
		want map[string]templates.ContainerdRuntimeConfig
	}{
		{
			name: "No runtimes",
			args: args{
				root: fstest.MapFS{},
			},
			want: map[string]templates.ContainerdRuntimeConfig{},
		},
		{
			name: "Nvidia runtime in /usr/bin",
			args: args{
				root: fstest.MapFS{
					"usr/bin/nvidia-container-runtime": executable,
				},
			},
			want: map[string]templates.ContainerdRuntimeConfig{
				"nvidia": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/bin/nvidia-container-runtime",
				},
			},
		},
		{
			name: "Experimental runtime in /usr/local/nvidia/toolkit",
			args: args{
				root: fstest.MapFS{
					"usr/local/nvidia/toolkit/nvidia-container-runtime": executable,
				},
			},
			want: map[string]templates.ContainerdRuntimeConfig{
				"nvidia": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/local/nvidia/toolkit/nvidia-container-runtime",
				},
			},
		},
		{
			name: "Two runtimes in separate directories",
			args: args{
				root: fstest.MapFS{
					"usr/bin/nvidia-container-runtime":                  executable,
					"usr/local/nvidia/toolkit/nvidia-container-runtime": executable,
				},
			},
			want: map[string]templates.ContainerdRuntimeConfig{
				"nvidia": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/local/nvidia/toolkit/nvidia-container-runtime",
				},
			},
		},
		{
			name: "Experimental runtime in /usr/bin",
			args: args{
				root: fstest.MapFS{
					"usr/bin/nvidia-container-runtime-experimental": executable,
				},
			},
			want: map[string]templates.ContainerdRuntimeConfig{
				"nvidia-experimental": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/bin/nvidia-container-runtime-experimental",
				},
			},
		},
		{
			name: "Same runtime in two directories",
			args: args{
				root: fstest.MapFS{
					"usr/bin/nvidia-container-runtime-experimental":                  executable,
					"usr/local/nvidia/toolkit/nvidia-container-runtime-experimental": executable,
				},
			},
			want: map[string]templates.ContainerdRuntimeConfig{
				"nvidia-experimental": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/local/nvidia/toolkit/nvidia-container-runtime-experimental",
				},
			},
		},
		{
			name: "Both runtimes in /usr/bin",
			args: args{
				root: fstest.MapFS{
					"usr/bin/nvidia-container-runtime-experimental": executable,
					"usr/bin/nvidia-container-runtime":              executable,
				},
			},
			want: map[string]templates.ContainerdRuntimeConfig{
				"nvidia": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/bin/nvidia-container-runtime",
				},
				"nvidia-experimental": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/bin/nvidia-container-runtime-experimental",
				},
			},
		},
		{
			name: "Both runtimes in both directories",
			args: args{
				root: fstest.MapFS{
					"usr/local/nvidia/toolkit/nvidia-container-runtime":              executable,
					"usr/local/nvidia/toolkit/nvidia-container-runtime-experimental": executable,
					"usr/bin/nvidia-container-runtime":                               executable,
					"usr/bin/nvidia-container-runtime-experimental":                  executable,
				},
			},
			want: map[string]templates.ContainerdRuntimeConfig{
				"nvidia": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/local/nvidia/toolkit/nvidia-container-runtime",
				},
				"nvidia-experimental": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/local/nvidia/toolkit/nvidia-container-runtime-experimental",
				},
			},
		},
		{
			name: "Both runtimes in /usr/local/nvidia/toolkit",
			args: args{
				root: fstest.MapFS{
					"usr/local/nvidia/toolkit/nvidia-container-runtime":              executable,
					"usr/local/nvidia/toolkit/nvidia-container-runtime-experimental": executable,
				},
			},
			want: map[string]templates.ContainerdRuntimeConfig{
				"nvidia": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/local/nvidia/toolkit/nvidia-container-runtime",
				},
				"nvidia-experimental": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/local/nvidia/toolkit/nvidia-container-runtime-experimental",
				},
			},
		},
		{
			name: "Both runtimes in /usr/bin and one duplicate in /usr/local/nvidia/toolkit",
			args: args{
				root: fstest.MapFS{
					"usr/bin/nvidia-container-runtime":                               executable,
					"usr/bin/nvidia-container-runtime-experimental":                  executable,
					"usr/local/nvidia/toolkit/nvidia-container-runtime-experimental": executable,
				},
			},
			want: map[string]templates.ContainerdRuntimeConfig{
				"nvidia": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/bin/nvidia-container-runtime",
				},
				"nvidia-experimental": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/local/nvidia/toolkit/nvidia-container-runtime-experimental",
				},
			},
		},
		{
			name: "Runtime is a directory",
			args: args{
				root: fstest.MapFS{
					"usr/bin/nvidia-container-runtime": &fstest.MapFile{
						Mode: fs.ModeDir,
					},
				},
			},
			want: map[string]templates.ContainerdRuntimeConfig{},
		},
		{
			name: "Runtime in both directories, but one is a directory",
			args: args{
				root: fstest.MapFS{
					"usr/bin/nvidia-container-runtime": executable,
					"usr/local/nvidia/toolkit/nvidia-container-runtime": &fstest.MapFile{
						Mode: fs.ModeDir,
					},
				},
			},
			want: map[string]templates.ContainerdRuntimeConfig{
				"nvidia": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/bin/nvidia-container-runtime",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findNvidiaContainerRuntimes(tt.args.root); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findNvidiaContainerRuntimes() = %+v\nWant = %+v", got, tt.want)
			}
		})
	}
}
