package cluster

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/k3s-io/kine/pkg/endpoint"
	"github.com/rancher/k3s/pkg/clientaccess"
	"github.com/rancher/k3s/pkg/cluster/managed"
	"github.com/rancher/k3s/pkg/daemons/config"
)

func Test_isDirEmpty(t *testing.T) {
	const tmpDir = "test_dir"

	type args struct {
		name string
	}
	tests := []struct {
		name     string
		args     args
		setup    func() error
		teardown func() error
		want     bool
		wantErr  bool
	}{
		{
			name: "is empty",
			args: args{
				name: tmpDir,
			},
			setup: func() error {
				return os.Mkdir(tmpDir, 0700)
			},
			teardown: func() error {
				return os.RemoveAll(tmpDir)
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "is not empty",
			args: args{
				name: tmpDir,
			},
			setup: func() error {
				os.Mkdir(tmpDir, 0700)
				_, _ = os.Create(filepath.Join(filepath.Dir(tmpDir), tmpDir, "test_file"))
				return nil
			},
			teardown: func() error {
				return os.RemoveAll(tmpDir)
			},
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer tt.teardown()
			if err := tt.setup(); err != nil {
				t.Errorf("Setup for isDirEmpty() failed = %v", err)
				return
			}
			got, err := isDirEmpty(tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("isDirEmpty() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("isDirEmpty() = %+v\nWant = %+v", got, tt.want)
			}
		})
	}
}

func TestCluster_certDirsExist(t *testing.T) {
	const testDataDir = "/tmp/k3s"

	testTLSDir := filepath.Join(testDataDir, "server", "tls")
	testTLSEtcdDir := filepath.Join(testDataDir, "server", "tls", "etcd")

	type fields struct {
		clientAccessInfo *clientaccess.Info
		config           *config.Control
		runtime          *config.ControlRuntime
		managedDB        managed.Driver
		etcdConfig       endpoint.ETCDConfig
		shouldBootstrap  bool
		storageStarted   bool
		saveBootstrap    bool
	}
	tests := []struct {
		name     string
		fields   fields
		setup    func() error
		teardown func() error
		wantErr  bool
	}{
		{
			name: "exists",
			fields: fields{
				config: &config.Control{
					DataDir: testDataDir,
				},
			},
			setup: func() error {
				os.MkdirAll(testTLSEtcdDir, 0700)

				testTLSFileFullPath := filepath.Join(testTLSDir, "test_file")
				testEtcdTLSFullPath := filepath.Join(testTLSEtcdDir, "test_file")

				_, _ = os.Create(testTLSFileFullPath)
				_, _ = os.Create(testEtcdTLSFullPath)

				fmt.Println(testEtcdTLSFullPath + "  --  " + testTLSFileFullPath)
				return nil
			},
			teardown: func() error {
				return os.RemoveAll(testTLSDir)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Cluster{
				clientAccessInfo: tt.fields.clientAccessInfo,
				config:           tt.fields.config,
				runtime:          tt.fields.runtime,
				managedDB:        tt.fields.managedDB,
				etcdConfig:       tt.fields.etcdConfig,
				shouldBootstrap:  tt.fields.shouldBootstrap,
				storageStarted:   tt.fields.storageStarted,
				saveBootstrap:    tt.fields.saveBootstrap,
			}
			defer tt.teardown()
			if err := tt.setup(); err != nil {
				t.Errorf("Setup for Cluster.certDirsExist() failed = %v", err)
				return
			}
			if err := c.certDirsExist(); (err != nil) != tt.wantErr {
				t.Errorf("Cluster.certDirsExist() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
