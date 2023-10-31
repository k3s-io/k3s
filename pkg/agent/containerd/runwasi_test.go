//go:build linux
// +build linux

package containerd

import (
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"
)

func Test_UnitFindWasiRuntimes(t *testing.T) {
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
			name: "wasmtime runtime in /usr/sbin",
			args: args{
				root: fstest.MapFS{
					"usr/sbin/containerd-shim-wasmtime-v1": executable,
				},
				alreadyFoundRuntimes: runtimeConfigs{},
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
				alreadyFoundRuntimes: runtimeConfigs{},
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
				alreadyFoundRuntimes: runtimeConfigs{},
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
				alreadyFoundRuntimes: runtimeConfigs{},
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
				alreadyFoundRuntimes: runtimeConfigs{},
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
				alreadyFoundRuntimes: runtimeConfigs{},
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
				},
				alreadyFoundRuntimes: runtimeConfigs{
					"nvidia": {
						RuntimeType: "io.containerd.runc.v2",
						BinaryName:  "/usr/bin/nvidia-container-runtime",
					},
				},
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
			foundRuntimes := tt.args.alreadyFoundRuntimes
			findWasiRuntimes(tt.args.root, foundRuntimes)
			if !reflect.DeepEqual(foundRuntimes, tt.want) {
				t.Errorf("findWasiRuntimes() = %+v\nWant = %+v", foundRuntimes, tt.want)
			}
		})
	}
}
