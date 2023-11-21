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

	type args struct {
		root fs.FS
	}

	tests := []struct {
		name string
		args args
		want runtimeConfigs
	}{
		{
			name: "No runtimes",
			args: args{
				root: fstest.MapFS{},
			},
			want: runtimeConfigs{},
		},
		{
			name: "Found crun, nvidia and wasm",
			args: args{
				root: fstest.MapFS{
					"usr/bin/nvidia-container-runtime":         executable,
					"usr/bin/crun":                             executable,
					"opt/kwasm/bin/containerd-shim-lunatic-v1": executable,
				},
			},
			want: runtimeConfigs{
				"nvidia": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/bin/nvidia-container-runtime",
				},
				"crun": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/bin/crun",
				},
				"lunatic": {
					RuntimeType: "io.containerd.lunatic.v2",
					BinaryName:  "/opt/kwasm/bin/containerd-shim-lunatic-v1",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			foundRuntimes := findContainerRuntimes(tt.args.root)
			if !reflect.DeepEqual(foundRuntimes, tt.want) {
				t.Errorf("findContainerRuntimes = %+v\nWant = %+v", foundRuntimes, tt.want)
			}
		})
	}
}

func Test_UnitSearchContainerRuntimes(t *testing.T) {
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
			searchForRuntimes(tt.args.root, tt.args.potentialRuntimes, tt.args.locationsToCheck, foundRuntimes)
			if !reflect.DeepEqual(foundRuntimes, tt.want) {
				t.Errorf("findContainerRuntimes() = %+v\nWant = %+v", foundRuntimes, tt.want)
			}
		})
	}
}

func Test_UnitSearchWasiRuntimes(t *testing.T) {
	executable := &fstest.MapFile{Mode: 0755}

	locationsToCheck := []string{
		"usr/local/nvidia/toolkit", // Path for nvidia shim when installing via GPU Operator
		"opt/kwasm/bin",            // Path for wasm shim when installing via the kwasm operator
		"usr/bin",                  // Path when installing via package manager
		"usr/sbin",                 // Path when installing via package manager
	}

	potentialRuntimes := runtimeConfigs{
		"wasmtime": {
			RuntimeType: "io.containerd.wasmtime.v2",
			BinaryName:  "containerd-shim-wasmtime-v1",
		},
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
		"nvidia": {
			RuntimeType: "io.containerd.runc.v2",
			BinaryName:  "nvidia-container-runtime",
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
			name: "wasmtime runtime in /usr/sbin",
			args: args{
				root: fstest.MapFS{
					"usr/sbin/containerd-shim-wasmtime-v1": executable,
				},
				locationsToCheck:  locationsToCheck,
				potentialRuntimes: potentialRuntimes,
			},
			want: runtimeConfigs{
				"wasmtime": {
					RuntimeType: "io.containerd.wasmtime.v2",
					BinaryName:  "/usr/sbin/containerd-shim-wasmtime-v1",
				},
			},
		},
		{
			name: "lunatic runtime in /opt/kwasm/bin/",
			args: args{
				root: fstest.MapFS{
					"opt/kwasm/bin/containerd-shim-lunatic-v1": executable,
				},
				locationsToCheck:  locationsToCheck,
				potentialRuntimes: potentialRuntimes,
			},
			want: runtimeConfigs{
				"lunatic": {
					RuntimeType: "io.containerd.lunatic.v2",
					BinaryName:  "/opt/kwasm/bin/containerd-shim-lunatic-v1",
				},
			},
		},
		{
			name: "Two runtimes in separate directories",
			args: args{
				root: fstest.MapFS{
					"usr/bin/containerd-shim-wasmer-v1":       executable,
					"opt/kwasm/bin/containerd-shim-slight-v1": executable,
				},
				locationsToCheck:  locationsToCheck,
				potentialRuntimes: potentialRuntimes,
			},
			want: runtimeConfigs{
				"slight": {
					RuntimeType: "io.containerd.slight.v2",
					BinaryName:  "/opt/kwasm/bin/containerd-shim-slight-v1",
				},
				"wasmer": {
					RuntimeType: "io.containerd.wasmer.v2",
					BinaryName:  "/usr/bin/containerd-shim-wasmer-v1",
				},
			},
		},
		{
			name: "Same runtime in two directories",
			args: args{
				root: fstest.MapFS{
					"usr/bin/containerd-shim-wasmedge-v1":       executable,
					"opt/kwasm/bin/containerd-shim-wasmedge-v1": executable,
				},
				locationsToCheck:  locationsToCheck,
				potentialRuntimes: potentialRuntimes,
			},
			want: runtimeConfigs{
				"wasmedge": {
					RuntimeType: "io.containerd.wasmedge.v2",
					BinaryName:  "/opt/kwasm/bin/containerd-shim-wasmedge-v1",
				},
			},
		},
		{
			name: "All runtimes in /usr/bin",
			args: args{
				root: fstest.MapFS{
					"usr/bin/containerd-shim-lunatic-v1":  executable,
					"usr/bin/containerd-shim-slight-v1":   executable,
					"usr/bin/containerd-shim-spin-v1":     executable,
					"usr/bin/containerd-shim-wws-v1":      executable,
					"usr/bin/containerd-shim-wasmedge-v1": executable,
					"usr/bin/containerd-shim-wasmer-v1":   executable,
					"usr/bin/containerd-shim-wasmtime-v1": executable,
				},
				locationsToCheck:  locationsToCheck,
				potentialRuntimes: potentialRuntimes,
			},
			want: runtimeConfigs{
				"lunatic": {
					RuntimeType: "io.containerd.lunatic.v2",
					BinaryName:  "/usr/bin/containerd-shim-lunatic-v1",
				},
				"slight": {
					RuntimeType: "io.containerd.slight.v2",
					BinaryName:  "/usr/bin/containerd-shim-slight-v1",
				},
				"spin": {
					RuntimeType: "io.containerd.spin.v2",
					BinaryName:  "/usr/bin/containerd-shim-spin-v1",
				},
				"wws": {
					RuntimeType: "io.containerd.wws.v2",
					BinaryName:  "/usr/bin/containerd-shim-wws-v1",
				},
				"wasmedge": {
					RuntimeType: "io.containerd.wasmedge.v2",
					BinaryName:  "/usr/bin/containerd-shim-wasmedge-v1",
				},
				"wasmer": {
					RuntimeType: "io.containerd.wasmer.v2",
					BinaryName:  "/usr/bin/containerd-shim-wasmer-v1",
				},
				"wasmtime": {
					RuntimeType: "io.containerd.wasmtime.v2",
					BinaryName:  "/usr/bin/containerd-shim-wasmtime-v1",
				},
			},
		},
		{
			name: "Both runtimes in both directories",
			args: args{
				root: fstest.MapFS{
					"opt/kwasm/bin/containerd-shim-slight-v1":   executable,
					"opt/kwasm/bin/containerd-shim-wasmtime-v1": executable,
					"usr/bin/containerd-shim-slight-v1":         executable,
					"usr/bin/containerd-shim-wasmtime-v1":       executable,
				},
				locationsToCheck:  locationsToCheck,
				potentialRuntimes: potentialRuntimes,
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
			},
		},
		{
			name: "Preserve already found runtimes",
			args: args{
				root: fstest.MapFS{
					"opt/kwasm/bin/containerd-shim-wasmtime-v1": executable,
					"usr/bin/nvidia-container-runtime":          executable,
				},
				locationsToCheck:  locationsToCheck,
				potentialRuntimes: potentialRuntimes,
			},
			want: runtimeConfigs{
				"nvidia": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/usr/bin/nvidia-container-runtime",
				},
				"wasmtime": {
					RuntimeType: "io.containerd.wasmtime.v2",
					BinaryName:  "/opt/kwasm/bin/containerd-shim-wasmtime-v1",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			foundRuntimes := runtimeConfigs{}
			searchForRuntimes(tt.args.root, tt.args.potentialRuntimes, tt.args.locationsToCheck, foundRuntimes)
			if !reflect.DeepEqual(foundRuntimes, tt.want) {
				t.Errorf("searchForRuntimes = %+v\nWant = %+v", foundRuntimes, tt.want)
			}
		})
	}
}

func Test_UnitSearchNvidiaContainerRuntimes(t *testing.T) {
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
		"nvidia-experimental": {
			RuntimeType: "io.containerd.runc.v2",
			BinaryName:  "nvidia-container-runtime-experimental",
		},
		"slight": {
			RuntimeType: "io.containerd.slight.v2",
			BinaryName:  "containerd-shim-slight-v1",
		},
		"wasmtime": {
			RuntimeType: "io.containerd.wasmtime.v2",
			BinaryName:  "containerd-shim-wasmtime-v1",
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
				potentialRuntimes: potentialRuntimes,
				locationsToCheck:  locationsToCheck,
			},
			want: runtimeConfigs{},
		},
		{
			name: "Nvidia runtime in /usr/bin",
			args: args{
				root: fstest.MapFS{
					"usr/bin/nvidia-container-runtime": executable,
				},
				potentialRuntimes: potentialRuntimes,
				locationsToCheck:  locationsToCheck,
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
				potentialRuntimes: potentialRuntimes,
				locationsToCheck:  locationsToCheck,
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
				potentialRuntimes: potentialRuntimes,
				locationsToCheck:  locationsToCheck,
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
				potentialRuntimes: potentialRuntimes,
				locationsToCheck:  locationsToCheck,
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
				potentialRuntimes: potentialRuntimes,
				locationsToCheck:  locationsToCheck,
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
				potentialRuntimes: potentialRuntimes,
				locationsToCheck:  locationsToCheck,
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
				potentialRuntimes: potentialRuntimes,
				locationsToCheck:  locationsToCheck,
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
				potentialRuntimes: potentialRuntimes,
				locationsToCheck:  locationsToCheck,
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
				potentialRuntimes: potentialRuntimes,
				locationsToCheck:  locationsToCheck,
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
				potentialRuntimes: potentialRuntimes,
				locationsToCheck:  locationsToCheck,
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
				potentialRuntimes: potentialRuntimes,
				locationsToCheck:  locationsToCheck,
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
					"usr/bin/nvidia-container-runtime":          executable,
					"opt/kwasm/bin/containerd-shim-wasmtime-v1": executable,
					"opt/kwasm/bin/containerd-shim-slight-v1":   executable,
					"usr/local/nvidia/toolkit/nvidia-container-runtime": &fstest.MapFile{
						Mode: fs.ModeDir,
					},
				},
				potentialRuntimes: potentialRuntimes,
				locationsToCheck:  locationsToCheck,
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
			foundRuntimes := runtimeConfigs{}
			searchForRuntimes(tt.args.root, tt.args.potentialRuntimes, tt.args.locationsToCheck, foundRuntimes)
			if !reflect.DeepEqual(foundRuntimes, tt.want) {
				t.Errorf("searchForRuntimes() = %+v\nWant = %+v", foundRuntimes, tt.want)
			}
		})
	}
}
