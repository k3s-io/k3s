//go:build linux
// +build linux

package containerd

import (
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"
)

func Test_UnitFindNvidiaContainerRuntimes(t *testing.T) {
	executable := &fstest.MapFile{Mode: 0755}
	type args struct {
		root                 fs.FS
		alreadyFoundRuntimes runtimeConfigs
	}
	tests := []struct {
		name string
		args args
		want runtimeConfigs
	}{
		{
			name: "No runtimes",
			args: args{
				root:                 fstest.MapFS{},
				alreadyFoundRuntimes: runtimeConfigs{},
			},
			want: runtimeConfigs{},
		},
		{
			name: "Nvidia runtime in /usr/bin",
			args: args{
				root: fstest.MapFS{
					"usr/bin/nvidia-container-runtime": executable,
				},
				alreadyFoundRuntimes: runtimeConfigs{},
			},
			want: runtimeConfigs{
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
				alreadyFoundRuntimes: runtimeConfigs{},
			},
			want: runtimeConfigs{
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
				alreadyFoundRuntimes: runtimeConfigs{},
			},
			want: runtimeConfigs{
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
				alreadyFoundRuntimes: runtimeConfigs{},
			},
			want: runtimeConfigs{
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
				alreadyFoundRuntimes: runtimeConfigs{},
			},
			want: runtimeConfigs{
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
				alreadyFoundRuntimes: runtimeConfigs{},
			},
			want: runtimeConfigs{
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
				alreadyFoundRuntimes: runtimeConfigs{},
			},
			want: runtimeConfigs{
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
				alreadyFoundRuntimes: runtimeConfigs{},
			},
			want: runtimeConfigs{
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
				alreadyFoundRuntimes: runtimeConfigs{},
			},
			want: runtimeConfigs{
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
				alreadyFoundRuntimes: runtimeConfigs{},
			},
			want: runtimeConfigs{},
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
				alreadyFoundRuntimes: runtimeConfigs{},
			},
			want: runtimeConfigs{
				"nvidia": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/bin/nvidia-container-runtime",
				},
			},
		},
		{
			name: "Preserve already found runtimes",
			args: args{
				root: fstest.MapFS{
					"usr/bin/nvidia-container-runtime": executable,
					"usr/local/nvidia/toolkit/nvidia-container-runtime": &fstest.MapFile{
						Mode: fs.ModeDir,
					},
				},
				alreadyFoundRuntimes: runtimeConfigs{
					"slight": {
						RuntimeType: "io.containerd.slight.v2",
						BinaryName:  "/opt/kwasm/bin/containerd-shim-slight-v1",
					},
					"wasmtime": {
						RuntimeType: "io.containerd.wasmtime.v2",
						BinaryName:  "/opt/kwasm/bin/containerd-shim-wasmtime-v1",
					},
				},
			},
			want: runtimeConfigs{
				"slight": {
					RuntimeType: "io.containerd.slight.v2",
					BinaryName:  "/opt/kwasm/bin/containerd-shim-slight-v1",
				},
				"wasmtime": {
					RuntimeType: "io.containerd.wasmtime.v2",
					BinaryName:  "/opt/kwasm/bin/containerd-shim-wasmtime-v1",
				},
				"nvidia": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/bin/nvidia-container-runtime",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			foundRuntimes := tt.args.alreadyFoundRuntimes
			findNvidiaContainerRuntimes(tt.args.root, foundRuntimes)
			if !reflect.DeepEqual(foundRuntimes, tt.want) {
				t.Errorf("findNvidiaContainerRuntimes() = %+v\nWant = %+v", foundRuntimes, tt.want)
			}
		})
	}
}
