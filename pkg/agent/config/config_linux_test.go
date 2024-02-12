//go:build linux
// +build linux

package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/tests"
)

func Test_UnitApplyContainerdQoSClassConfigFileIfPresent(t *testing.T) {
	configControl := config.Control{
		DataDir: "/tmp/k3s/",
	}

	if err := tests.GenerateDataDir(&configControl); err != nil {
		t.Errorf("Test_UnitApplyContainerdQoSClassConfigFileIfPresent() setup failed = %v", err)
	}
	defer tests.CleanupDataDir(&configControl)

	containerdConfigDir := filepath.Join(configControl.DataDir, "agent", "etc", "containerd")
	os.MkdirAll(containerdConfigDir, 0700)

	type args struct {
		envInfo          *cmds.Agent
		containerdConfig *config.Containerd
	}
	tests := []struct {
		name     string
		args     args
		setup    func() error
		teardown func()
		want     *config.Containerd
	}{
		{
			name: "No config file",
			args: args{
				envInfo: &cmds.Agent{
					DataDir: configControl.DataDir,
				},
				containerdConfig: &config.Containerd{},
			},
			setup: func() error {
				return nil
			},
			teardown: func() {},
			want:     &config.Containerd{},
		},
		{
			name: "BlockIO config file",
			args: args{
				envInfo: &cmds.Agent{
					DataDir: configControl.DataDir,
				},
				containerdConfig: &config.Containerd{},
			},
			setup: func() error {
				_, err := os.Create(filepath.Join(containerdConfigDir, "blockio_config.yaml"))
				return err
			},
			teardown: func() {
				os.Remove(filepath.Join(containerdConfigDir, "blockio_config.yaml"))
			},
			want: &config.Containerd{
				BlockIOConfig: filepath.Join(containerdConfigDir, "blockio_config.yaml"),
			},
		},
		{
			name: "RDT config file",
			args: args{
				envInfo: &cmds.Agent{
					DataDir: configControl.DataDir,
				},
				containerdConfig: &config.Containerd{},
			},
			setup: func() error {
				_, err := os.Create(filepath.Join(containerdConfigDir, "rdt_config.yaml"))
				return err
			},
			teardown: func() {
				os.Remove(filepath.Join(containerdConfigDir, "rdt_config.yaml"))
			},
			want: &config.Containerd{
				RDTConfig: filepath.Join(containerdConfigDir, "rdt_config.yaml"),
			},
		},
		{
			name: "Both config files",
			args: args{
				envInfo: &cmds.Agent{
					DataDir: configControl.DataDir,
				},
				containerdConfig: &config.Containerd{},
			},
			setup: func() error {
				_, err := os.Create(filepath.Join(containerdConfigDir, "blockio_config.yaml"))
				if err != nil {
					return err
				}
				_, err = os.Create(filepath.Join(containerdConfigDir, "rdt_config.yaml"))
				return err
			},
			teardown: func() {
				os.Remove(filepath.Join(containerdConfigDir, "blockio_config.yaml"))
				os.Remove(filepath.Join(containerdConfigDir, "rdt_config.yaml"))
			},
			want: &config.Containerd{
				BlockIOConfig: filepath.Join(containerdConfigDir, "blockio_config.yaml"),
				RDTConfig:     filepath.Join(containerdConfigDir, "rdt_config.yaml"),
			},
		},
		{
			name: "BlockIO path is a directory",
			args: args{
				envInfo: &cmds.Agent{
					DataDir: configControl.DataDir,
				},
				containerdConfig: &config.Containerd{},
			},
			setup: func() error {
				return os.Mkdir(filepath.Join(containerdConfigDir, "blockio_config.yaml"), 0700)
			},
			teardown: func() {
				os.Remove(filepath.Join(containerdConfigDir, "blockio_config.yaml"))
			},
			want: &config.Containerd{},
		},
		{
			name: "RDT path is a directory",
			args: args{
				envInfo: &cmds.Agent{
					DataDir: configControl.DataDir,
				},
				containerdConfig: &config.Containerd{},
			},
			setup: func() error {
				return os.Mkdir(filepath.Join(containerdConfigDir, "rdt_config.yaml"), 0700)
			},
			teardown: func() {
				os.Remove(filepath.Join(containerdConfigDir, "rdt_config.yaml"))
			},
			want: &config.Containerd{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			defer tt.teardown()

			envInfo := tt.args.envInfo
			containerdConfig := tt.args.containerdConfig
			applyContainerdQoSClassConfigFileIfPresent(envInfo, containerdConfig)
			if !reflect.DeepEqual(containerdConfig, tt.want) {
				t.Errorf("applyContainerdQoSClassConfigFileIfPresent() = %+v\nWant %+v", containerdConfig, tt.want)
			}
		})
	}
}
