//go:build linux
// +build linux

package containerd

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func Test_UnitFindContainerRuntimes(t *testing.T) {
	type args struct {
		exec []string
	}

	tests := []struct {
		name string
		args args
		want runtimeConfigs
	}{
		{
			name: "No runtimes",
			args: args{},
			want: runtimeConfigs{},
		},
		{
			name: "Found crun, nvidia and wasm",
			args: args{
				exec: []string{
					"nvidia-container-runtime",
					"crun",
					"containerd-shim-lunatic-v1",
				},
			},
			want: runtimeConfigs{
				"nvidia": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/tmp/testExecutables/nvidia-container-runtime",
				},
				"crun": {
					RuntimeType: "io.containerd.runc.v2",
					BinaryName:  "/tmp/testExecutables/crun",
				},
				"lunatic": {
					RuntimeType: "io.containerd.lunatic.v1",
					BinaryName:  "/tmp/testExecutables/containerd-shim-lunatic-v1",
				},
			},
		},
		{
			name: "Found only wasm",
			args: args{
				exec: []string{
					"containerd-shim-lunatic-v1",
					"containerd-shim-wasmtime-v1",
					"containerd-shim-lunatic-v1",
					"containerd-shim-slight-v1",
					"containerd-shim-spin-v2",
					"containerd-shim-wws-v1",
					"containerd-shim-wasmedge-v1",
					"containerd-shim-wasmer-v1",
				},
			},
			want: runtimeConfigs{
				"wasmtime": {
					RuntimeType: "io.containerd.wasmtime.v1",
					BinaryName:  "/tmp/testExecutables/containerd-shim-wasmtime-v1",
				},
				"lunatic": {
					RuntimeType: "io.containerd.lunatic.v1",
					BinaryName:  "/tmp/testExecutables/containerd-shim-lunatic-v1",
				},
				"slight": {
					RuntimeType: "io.containerd.slight.v1",
					BinaryName:  "/tmp/testExecutables/containerd-shim-slight-v1",
				},
				"spin": {
					RuntimeType: "io.containerd.spin.v2",
					BinaryName:  "/tmp/testExecutables/containerd-shim-spin-v2",
				},
				"wws": {
					RuntimeType: "io.containerd.wws.v1",
					BinaryName:  "/tmp/testExecutables/containerd-shim-wws-v1",
				},
				"wasmedge": {
					RuntimeType: "io.containerd.wasmedge.v1",
					BinaryName:  "/tmp/testExecutables/containerd-shim-wasmedge-v1",
				},
				"wasmer": {
					RuntimeType: "io.containerd.wasmer.v1",
					BinaryName:  "/tmp/testExecutables/containerd-shim-wasmer-v1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDirPath := filepath.Join(os.TempDir(), "testExecutables")
			err := os.Mkdir(tempDirPath, 0755)
			if err != nil {
				t.Errorf("Error creating directory: %v", err)
			}

			defer os.RemoveAll(tempDirPath)

			for _, execName := range tt.args.exec {
				execPath := filepath.Join(tempDirPath, execName)
				if err := createExec(execPath); err != nil {
					t.Errorf("Failed to create executable %s: %v", execPath, err)
				}
			}

			originalPath := os.Getenv("PATH")
			os.Setenv("PATH", tempDirPath)
			defer os.Setenv("PATH", originalPath)

			foundRuntimes := findContainerRuntimes()
			if !reflect.DeepEqual(foundRuntimes, tt.want) {
				t.Errorf("findContainerRuntimes = %+v\nWant = %+v", foundRuntimes, tt.want)
			}
		})
	}
}

func createExec(path string) error {
	if err := os.WriteFile(path, []byte{}, 0755); err != nil {
		return err
	}

	if err := os.Chmod(path, 0755); err != nil {
		return err
	}

	return nil
}
