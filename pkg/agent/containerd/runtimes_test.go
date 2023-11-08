//go:build linux
// +build linux

package containerd

import (
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"
)

func Test_UnitFindContainerRuntimes(t *testing.T) {
	executable := &fstest.MapFile{Mode: 0755}
	locationsToCheck := []string{
		"usr/local/nvidia/toolkit", // Path for nvidia shim when installing via GPU Operator
		"opt/kwasm/bin",            // Path for wasm shim when installing via the kwasm operator
		"usr/bin",                  // Path when installing via package manager
		"usr/sbin",                 // Path when installing via package manager
	}

	potentialRuntimes := runtimeConfigs{
		"nvidia": {
			RuntimeType: "io.containerd.runc.v2",
			BinaryName:  "nvidia-container-runtime",
		},
		"spin": {
			RuntimeType: "io.containerd.spin.v2",
			BinaryName:  "containerd-shim-spin-v1",
		},
	}

	type args struct {
		root              fs.FS
		potentialRuntimes runtimeConfigs
		locationsToCheck  []string
	}
	tests := []struct {
		name string
		args args
		want runtimeConfigs
	}{
		{
			name: "No runtimes",
			args: args{
				root:              fstest.MapFS{},
				locationsToCheck:  locationsToCheck,
				potentialRuntimes: potentialRuntimes,
			},
			want: runtimeConfigs{},
		},
		{
			name: "Nvidia runtime in /usr/bin",
			args: args{
				root: fstest.MapFS{
					"usr/bin/nvidia-container-runtime": executable,
				},
				locationsToCheck:  locationsToCheck,
				potentialRuntimes: potentialRuntimes,
			},
			want: runtimeConfigs{
				"nvidia": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/bin/nvidia-container-runtime",
				},
			},
		},
		{
			name: "Two runtimes in separate directories",
			args: args{
				root: fstest.MapFS{
					"usr/bin/nvidia-container-runtime":      executable,
					"opt/kwasm/bin/containerd-shim-spin-v1": executable,
				},
				locationsToCheck:  locationsToCheck,
				potentialRuntimes: potentialRuntimes,
			},
			want: runtimeConfigs{
				"nvidia": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/bin/nvidia-container-runtime",
				},
				"spin": {
					RuntimeType: "io.containerd.spin.v2",
					BinaryName:  "/opt/kwasm/bin/containerd-shim-spin-v1",
				},
			},
		},
		{
			name: "Same runtime in two directories",
			args: args{
				root: fstest.MapFS{
					"usr/bin/containerd-shim-spin-v1":       executable,
					"opt/kwasm/bin/containerd-shim-spin-v1": executable,
				},
				locationsToCheck:  locationsToCheck,
				potentialRuntimes: potentialRuntimes,
			},
			want: runtimeConfigs{
				"spin": {
					RuntimeType: "io.containerd.spin.v2",
					BinaryName:  "/opt/kwasm/bin/containerd-shim-spin-v1",
				},
			},
		},
		{
			name: "Both runtimes in /usr/bin",
			args: args{
				root: fstest.MapFS{
					"usr/bin/containerd-shim-spin-v1":  executable,
					"usr/bin/nvidia-container-runtime": executable,
				},
				locationsToCheck:  locationsToCheck,
				potentialRuntimes: potentialRuntimes,
			},
			want: runtimeConfigs{
				"nvidia": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/bin/nvidia-container-runtime",
				},
				"spin": {
					RuntimeType: "io.containerd.spin.v2",
					BinaryName:  "/usr/bin/containerd-shim-spin-v1",
				},
			},
		},
		{
			name: "Both runtimes in both directories",
			args: args{
				root: fstest.MapFS{
					"usr/local/nvidia/toolkit/nvidia-container-runtime": executable,
					"usr/bin/nvidia-container-runtime":                  executable,
					"usr/bin/containerd-shim-spin-v1":                   executable,
					"opt/kwasm/bin/containerd-shim-spin-v1":             executable,
				},
				locationsToCheck:  locationsToCheck,
				potentialRuntimes: potentialRuntimes,
			},
			want: runtimeConfigs{
				"nvidia": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/local/nvidia/toolkit/nvidia-container-runtime",
				},
				"spin": {
					RuntimeType: "io.containerd.spin.v2",
					BinaryName:  "/opt/kwasm/bin/containerd-shim-spin-v1",
				},
			},
		},
		{
			name: "Both runtimes in /usr/bin and one duplicate in /usr/local/nvidia/toolkit",
			args: args{
				root: fstest.MapFS{
					"usr/bin/nvidia-container-runtime":                  executable,
					"usr/bin/containerd-shim-spin-v1":                   executable,
					"usr/local/nvidia/toolkit/nvidia-container-runtime": executable,
				},
				locationsToCheck:  locationsToCheck,
				potentialRuntimes: potentialRuntimes,
			},
			want: runtimeConfigs{
				"spin": {
					RuntimeType: "io.containerd.spin.v2",
					BinaryName:  "/usr/bin/containerd-shim-spin-v1",
				},
				"nvidia": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/local/nvidia/toolkit/nvidia-container-runtime",
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
				locationsToCheck:  locationsToCheck,
				potentialRuntimes: potentialRuntimes,
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
				locationsToCheck:  locationsToCheck,
				potentialRuntimes: potentialRuntimes,
			},
			want: runtimeConfigs{
				"nvidia": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/bin/nvidia-container-runtime",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			foundRuntimes := runtimeConfigs{}
			findContainerRuntimes(tt.args.root, tt.args.potentialRuntimes, tt.args.locationsToCheck, foundRuntimes)
			if !reflect.DeepEqual(foundRuntimes, tt.want) {
				t.Errorf("findContainerRuntimes() = %+v\nWant = %+v", foundRuntimes, tt.want)
			}
		})
	}
}
