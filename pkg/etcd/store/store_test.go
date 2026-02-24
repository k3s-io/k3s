package store

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/k3s-io/k3s/tests/fixtures"
	"github.com/k3s-io/kine/pkg/app"
	"github.com/k3s-io/kine/pkg/endpoint"
	"github.com/otiai10/copy"
	"github.com/sirupsen/logrus"
)

// These tests are shared by both NewStore and NewTemporaryStore, since
// NewTemporaryStore is just a wrapper around NewStore with a file copy operation.
// NewRemoteStore has different tests.

type args struct {
	dataDir string
}

var tests = []struct {
	name    string
	args    args
	setup   func(t *testing.T, args *args) error
	wantErr bool
}{
	{
		name:    "no db path",
		wantErr: true,
		setup: func(t *testing.T, args *args) error {
			return nil
		},
	},
	{
		name:    "empty data dir",
		wantErr: true,
		setup: func(t *testing.T, args *args) error {
			args.dataDir = t.TempDir()
			return os.MkdirAll(filepath.Join(args.dataDir, "member", "snap"), 0775)
		},
	},
	{
		name:    "empty db",
		wantErr: true,
		setup: func(t *testing.T, args *args) error {
			args.dataDir = t.TempDir()
			if err := os.MkdirAll(filepath.Join(args.dataDir, "member", "snap"), 0775); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(args.dataDir, "member", "snap", "db"), []byte(""), 0644)
		},
	},
	{
		name:    "empty db with wal",
		wantErr: true,
		setup: func(t *testing.T, args *args) error {
			args.dataDir = t.TempDir()
			if err := os.MkdirAll(filepath.Join(args.dataDir, "member", "snap"), 0775); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(args.dataDir, "member", "snap", "db"), []byte(""), 0644); err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Join(args.dataDir, "member", "wal"), 0775); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(args.dataDir, "member", "wal", "0000000000000000-0000000000000000.wal"), []byte(""), 0644)
		},
	},
	{
		name:    "invalid db",
		wantErr: true,
		setup: func(t *testing.T, args *args) error {
			args.dataDir = t.TempDir()
			if err := os.MkdirAll(filepath.Join(args.dataDir, "member", "snap"), 0775); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(args.dataDir, "member", "snap", "db"), []byte("test"), 0644)
		},
	},
	{
		name: "valid db with wal",
		setup: func(t *testing.T, args *args) error {
			args.dataDir = t.TempDir()
			copyOpts := copy.Options{
				FS:                fixtures.ETCD,
				PermissionControl: copy.AddPermission(0664),
			}
			return copy.Copy((filepath.Join("member")), filepath.Join(args.dataDir, "member"), copyOpts)
		},
	},
	{
		name: "valid db with no wal",
		setup: func(t *testing.T, args *args) error {
			args.dataDir = t.TempDir()
			copyOpts := copy.Options{
				FS:                fixtures.ETCD,
				PermissionControl: copy.AddPermission(0664),
			}
			if err := copy.Copy((filepath.Join("member")), filepath.Join(args.dataDir, "member"), copyOpts); err != nil {
				return err
			}
			return os.RemoveAll(filepath.Join(args.dataDir, "member", "wal"))
		},
	},
	{
		name:    "valid wal with no db",
		wantErr: true,
		setup: func(t *testing.T, args *args) error {
			args.dataDir = t.TempDir()
			copyOpts := copy.Options{
				FS:                fixtures.ETCD,
				PermissionControl: copy.AddPermission(0664),
			}
			if err := copy.Copy((filepath.Join("member")), filepath.Join(args.dataDir, "member"), copyOpts); err != nil {
				return err
			}
			return os.RemoveAll(filepath.Join(args.dataDir, "member", "snap", "db"))
		},
	},
}

func Test_UnitNewStore(t *testing.T) {
	logrus.SetLevel(logrus.TraceLevel)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.setup(t, &tt.args); err != nil {
				t.Errorf("Setup for NewStore() failed = %v", err)
				return
			}
			kv, err := NewStore(tt.args.dataDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewStore() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			t.Logf("NewStore() error = %v, kv = %#v", err, kv)

			if !tt.wantErr {
				kvs, err := kv.List(t.Context(), "/bootstrap", 0)
				if err != nil {
					t.Errorf("List() error = %v", err)
					return
				}
				for _, kv := range kvs {
					t.Logf("Got Key=%v", kv.Key)
				}
				if err := kv.Close(); err != nil {
					t.Errorf("Close() error = %v", err)
				}
			}
		})
	}
}

func Test_UnitNewTemporaryStore(t *testing.T) {
	logrus.SetLevel(logrus.TraceLevel)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.setup(t, &tt.args); err != nil {
				t.Errorf("Setup for NewTemporaryStore() failed = %v", err)
				return
			}
			kv, err := NewTemporaryStore(tt.args.dataDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTemporaryStore() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			t.Logf("NewTemporaryStore() error = %v, kv = %#v", err, kv)

			if !tt.wantErr {
				kvs, err := kv.List(t.Context(), "/bootstrap", 0)
				if err != nil {
					t.Errorf("List() error = %v", err)
					return
				}
				for _, kv := range kvs {
					t.Logf("Got Key=%v", kv.Key)
				}
				if err := kv.Close(); err != nil {
					t.Errorf("Close() error = %v", err)
				}
			}
		})
	}
}

func Test_UnitNewRemoteStore(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)

	type args struct {
		config endpoint.ETCDConfig
	}

	tests := []struct {
		name    string
		args    args
		setup   func(t *testing.T, args *args) error
		wantErr bool
	}{
		{
			name:    "no endpoints",
			setup:   func(t *testing.T, args *args) error { return nil },
			wantErr: true,
		},
		{
			name:    "no running endpoints",
			args:    args{config: endpoint.ETCDConfig{Endpoints: []string{"http://127.0.0.1:2379"}}},
			setup:   func(t *testing.T, args *args) error { return nil },
			wantErr: true,
		},
		{
			name: "kine endpoint",
			setup: func(t *testing.T, args *args) error {
				config := app.Config(nil)
				config.Listener = "0.0.0.0:0"
				config.Endpoint = "sqlite://" + t.TempDir() + "/state.db?_journal=WAL&cache=shared&_busy_timeout=30000&_txlock=immediate"
				config.WaitGroup = &sync.WaitGroup{}
				t.Cleanup(func() { config.WaitGroup.Wait() })
				etcdconfig, err := endpoint.Listen(t.Context(), config)
				args.config = etcdconfig
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.setup(t, &tt.args); err != nil {
				t.Errorf("Setup for NewRemoteStore() failed = %v", err)
				return
			}
			kv, err := NewRemoteStore(tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRemoteStore() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			t.Logf("NewRemoteStore() error = %v, kv = %#v", err, kv)

			if !tt.wantErr {
				kvs, err := kv.List(t.Context(), "/bootstrap", 0)
				if err != nil {
					t.Errorf("List() error = %v", err)
					return
				}
				for _, kv := range kvs {
					t.Logf("Got Key=%v", kv.Key)
				}
				if err := kv.Close(); err != nil {
					t.Errorf("Close() error = %v", err)
				}
			}
		})
	}
}
