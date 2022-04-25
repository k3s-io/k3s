package cluster

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/k3s-io/k3s/pkg/bootstrap"
	"github.com/k3s-io/k3s/pkg/clientaccess"
	"github.com/k3s-io/k3s/pkg/cluster/managed"
	"github.com/k3s-io/k3s/pkg/daemons/config"
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
	const testDataDir = "/tmp/k3s/"

	testCredDir := filepath.Join(testDataDir, "server", "cred")
	testTLSDir := filepath.Join(testDataDir, "server", "tls")
	testTLSEtcdDir := filepath.Join(testDataDir, "server", "tls", "etcd")

	type fields struct {
		clientAccessInfo *clientaccess.Info
		config           *config.Control
		managedDB        managed.Driver
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
					DataDir: filepath.Join(testDataDir, "server"),
				},
			},
			setup: func() error {
				os.MkdirAll(testCredDir, 0700)
				os.MkdirAll(testTLSEtcdDir, 0700)

				_, _ = os.Create(filepath.Join(testCredDir, "test_file"))
				_, _ = os.Create(filepath.Join(testTLSDir, "test_file"))
				_, _ = os.Create(filepath.Join(testTLSEtcdDir, "test_file"))

				return nil
			},
			teardown: func() error {
				return os.RemoveAll(testDataDir)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Cluster{
				clientAccessInfo: tt.fields.clientAccessInfo,
				config:           tt.fields.config,
				managedDB:        tt.fields.managedDB,
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

func TestCluster_migrateBootstrapData(t *testing.T) {
	type fields struct {
		clientAccessInfo *clientaccess.Info
		config           *config.Control
		managedDB        managed.Driver
		joining          bool
		storageStarted   bool
		saveBootstrap    bool
		shouldBootstrap  bool
	}
	type args struct {
		ctx   context.Context
		data  *bytes.Buffer
		files bootstrap.PathsDataformat
	}
	tests := []struct {
		name     string
		args     args
		setup    func() error // Optional, delete if unused
		teardown func() error // Optional, delete if unused
		wantErr  bool
	}{
		{
			name: "Success",
			args: args{
				ctx:   context.Background(),
				data:  bytes.NewBuffer([]byte(`{"ServerCA": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURSBSRVFVRVNULS0tLS0KTUlJQ3ZEQ0NBYVFDQVFBd2R6RUxNQWtHQTFVRUJoTUNWVk14RFRBTEJnTlZCQWdNQkZWMFlXZ3hEekFOQmdOVgpCQWNNQmt4cGJtUnZiakVXTUJRR0ExVUVDZ3dOUkdsbmFVTmxjblFnU1c1akxqRVJNQThHQTFVRUN3d0lSR2xuCmFVTmxjblF4SFRBYkJnTlZCQU1NRkdWNFlXMXdiR1V1WkdsbmFXTmxjblF1WTI5dE1JSUJJakFOQmdrcWhraUcKOXcwQkFRRUZBQU9DQVE4QU1JSUJDZ0tDQVFFQTgrVG83ZCsya1BXZUJ2L29yVTNMVmJKd0RyU1FiZUthbUNtbwp3cDVicUR4SXdWMjB6cVJiN0FQVU9LWW9WRUZGT0VRczZUNmdJbW5Jb2xoYmlINm00emdaL0NQdldCT2taYytjCjFQbzJFbXZCeitBRDVzQmRUNWt6R1FBNk5iV3laR2xkeFJ0aE5MT3MxZWZPaGRuV0Z1aEkxNjJxbWNmbGdwaUkKV0R1d3E0QzlmK1lrZUpoTm45ZEY1K293bThjT1FtRHJWOE5OZGlUcWluOHEzcVlBSEhKUlcyOGdsSlVDWmtUWgp3SWFTUjZjckJROFRiWU5FMGRjK0NhYTNET0lrejFFT3NIV3pUeCtuMHpLZnFjYmdYaTRESngrQzFianB0WVBSCkJQWkw4REFlV3VBOGVidWRWVDQ0eUVwODJHOTYvR2djZjdGMzN4TXhlMHljK1hhNm93SURBUUFCb0FBd0RRWUoKS29aSWh2Y05BUUVGQlFBRGdnRUJBQjBrY3JGY2NTbUZEbXhveDBOZTAxVUlxU3NEcUhnTCtYbUhUWEp3cmU2RApoSlNad2J2RXRPSzBHMytkcjRGczExV3VVTnQ1cWNMc3g1YTh1azRHNkFLSE16dWhMc0o3WFpqZ21RWEdFQ3BZClE0bUMzeVQzWm9DR3BJWGJ3K2lQM2xtRUVYZ2FRTDBUeDVMRmwvb2tLYktZd0lxTml5S1dPTWo3WlIvd3hXZy8KWkRHUnM1NXh1b2VMREovWlJGZjliSStJYUNVZDFZcmZZY0hJbDNHODdBdityNDlZVndxUkRUMFZEVjd1TGdxbgoyOVhJMVBwVlVOQ1BRR245cC9lWDZRbzd2cERhUHliUnRBMlI3WExLalFhRjlvWFdlQ1VxeTFodkphYzlRRk8yCjk3T2IxYWxwSFBvWjdtV2lFdUp3akJQaWk2YTlNOUczMG5VbzM5bEJpMXc9Ci0tLS0tRU5EIENFUlRJRklDQVRFIFJFUVVFU1QtLS0tLQ=="}`)),
				files: make(bootstrap.PathsDataformat),
			},
			wantErr: false,
		},
		{
			name: "Invalid Old Format",
			args: args{
				ctx:  context.Background(),
				data: &bytes.Buffer{},
				files: bootstrap.PathsDataformat{
					"ServerCA": bootstrap.File{
						Timestamp: time.Now(),
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := migrateBootstrapData(tt.args.ctx, tt.args.data, tt.args.files); (err != nil) != tt.wantErr {
				t.Errorf("Cluster.migrateBootstrapData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCluster_Snapshot(t *testing.T) {
	type fields struct {
		clientAccessInfo *clientaccess.Info
		config           *config.Control
		managedDB        managed.Driver
		joining          bool
		storageStarted   bool
		saveBootstrap    bool
		shouldBootstrap  bool
	}
	type args struct {
		ctx    context.Context
		config *config.Control
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name:   "Fail on non etcd cluster",
			fields: fields{},
			args: args{
				ctx: context.Background(),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Cluster{
				clientAccessInfo: tt.fields.clientAccessInfo,
				config:           tt.fields.config,
				managedDB:        tt.fields.managedDB,
				joining:          tt.fields.joining,
				storageStarted:   tt.fields.storageStarted,
				saveBootstrap:    tt.fields.saveBootstrap,
				shouldBootstrap:  tt.fields.shouldBootstrap,
			}
			if err := c.Snapshot(tt.args.ctx, tt.args.config); (err != nil) != tt.wantErr {
				t.Errorf("Cluster.Snapshot() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
