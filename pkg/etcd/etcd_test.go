package etcd

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/k3s-io/k3s/pkg/clientaccess"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/etcd/s3"
	testutil "github.com/k3s-io/k3s/tests"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/etcdserver"
	utilnet "k8s.io/apimachinery/pkg/util/net"
)

func mustGetAddress() string {
	ipAddr, err := utilnet.ChooseHostInterface()
	if err != nil {
		panic(err)
	}
	return ipAddr.String()
}

func generateTestConfig() *config.Control {
	hostname, _ := os.Hostname()
	containerRuntimeReady := make(chan struct{})
	close(containerRuntimeReady)
	criticalControlArgs := config.CriticalControlArgs{
		ClusterDomain:  "cluster.local",
		ClusterDNS:     net.ParseIP("10.43.0.10"),
		ClusterIPRange: testutil.ClusterIPNet(),
		FlannelBackend: "vxlan",
		ServiceIPRange: testutil.ServiceIPNet(),
	}
	return &config.Control{
		ServerNodeName:        hostname,
		Runtime:               config.NewRuntime(containerRuntimeReady),
		HTTPSPort:             6443,
		SupervisorPort:        6443,
		AdvertisePort:         6443,
		DataDir:               "/tmp/k3s/", // Different than the default value
		EtcdSnapshotName:      "etcd-snapshot",
		EtcdSnapshotCron:      "0 */12 * * *",
		EtcdSnapshotRetention: 5,
		EtcdS3: &config.EtcdS3{
			Endpoint: "s3.amazonaws.com",
			Region:   "us-east-1",
		},
		SANs:                []string{"127.0.0.1", mustGetAddress()},
		CriticalControlArgs: criticalControlArgs,
	}
}

func generateTestHandler() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {})
}

func Test_UnitETCD_IsInitialized(t *testing.T) {
	type args struct {
		ctx    context.Context
		config *config.Control
	}
	tests := []struct {
		name     string
		args     args
		setup    func(*config.Control) error
		teardown func(*config.Control) error
		want     bool
		wantErr  bool
	}{
		{
			name: "Directory exists",
			args: args{
				ctx:    context.TODO(),
				config: generateTestConfig(),
			},
			setup: func(cnf *config.Control) error {
				if err := testutil.GenerateDataDir(cnf); err != nil {
					return err
				}
				return os.MkdirAll(walDir(cnf), 0700)
			},
			teardown: func(cnf *config.Control) error {
				testutil.CleanupDataDir(cnf)
				return os.Remove(walDir(cnf))
			},
			wantErr: false,
			want:    true,
		},
		{
			name: "Directory does not exist",
			args: args{
				ctx:    context.TODO(),
				config: generateTestConfig(),
			},
			setup: func(cnf *config.Control) error {
				if err := testutil.GenerateDataDir(cnf); err != nil {
					return err
				}
				// We don't care if removal fails to find the dir
				os.Remove(walDir(cnf))
				return nil
			},
			teardown: func(cnf *config.Control) error {
				testutil.CleanupDataDir(cnf)
				return nil
			},
			wantErr: false,
			want:    false,
		},
	}

	// enable logging
	logrus.SetLevel(logrus.DebugLevel)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewETCD()
			defer tt.teardown(tt.args.config)
			if err := tt.setup(tt.args.config); err != nil {
				t.Errorf("Prep for ETCD.IsInitialized() failed = %v", err)
				return
			}
			if err := e.SetControlConfig(tt.args.config); err != nil {
				t.Errorf("ETCD.SetControlConfig() failed= %v", err)
				return
			}
			got, err := e.IsInitialized()
			if (err != nil) != tt.wantErr {
				t.Errorf("ETCD.IsInitialized() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ETCD.IsInitialized() = %+v\nWant = %+v", got, tt.want)
				return
			}
		})
	}
}

func Test_UnitETCD_Register(t *testing.T) {
	type args struct {
		ctx     context.Context
		config  *config.Control
		handler http.Handler
	}
	tests := []struct {
		name     string
		args     args
		setup    func(cnf *config.Control) error
		teardown func(cnf *config.Control) error
		wantErr  bool
	}{
		{
			name: "Call Register with standard config",
			args: args{
				ctx:     context.TODO(),
				config:  generateTestConfig(),
				handler: generateTestHandler(),
			},
			setup: func(cnf *config.Control) error {
				return testutil.GenerateRuntime(cnf)
			},
			teardown: func(cnf *config.Control) error {
				testutil.CleanupDataDir(cnf)
				return nil
			},
		},
		{
			name: "Call Register with a tombstone file created",
			args: args{
				ctx:     context.TODO(),
				config:  generateTestConfig(),
				handler: generateTestHandler(),
			},
			setup: func(cnf *config.Control) error {
				if err := testutil.GenerateRuntime(cnf); err != nil {
					return err
				}
				if err := os.MkdirAll(dbDir(cnf), 0700); err != nil {
					return err
				}
				tombstoneFile := filepath.Join(dbDir(cnf), "tombstone")
				if _, err := os.Create(tombstoneFile); err != nil {
					return err
				}
				return nil
			},
			teardown: func(cnf *config.Control) error {
				tombstoneFile := filepath.Join(dbDir(cnf), "tombstone")
				os.Remove(tombstoneFile)
				testutil.CleanupDataDir(cnf)
				return nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewETCD()

			defer tt.teardown(tt.args.config)
			if err := tt.setup(tt.args.config); err != nil {
				t.Errorf("Setup for ETCD.Register() failed = %v", err)
				return
			}
			if err := e.SetControlConfig(tt.args.config); err != nil {
				t.Errorf("ETCD.SetControlConfig() failed = %v", err)
				return
			}
			_, err := e.Register(tt.args.handler)
			if (err != nil) != tt.wantErr {
				t.Errorf("ETCD.Register() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func Test_UnitETCD_Start(t *testing.T) {
	type contextInfo struct {
		ctx    context.Context
		cancel context.CancelFunc
	}
	type fields struct {
		context contextInfo
		client  *clientv3.Client
		config  *config.Control
		name    string
		address string
		cron    *cron.Cron
		s3      *s3.Controller
	}
	type args struct {
		clientAccessInfo *clientaccess.Info
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		setup    func(e *ETCD, ctxInfo *contextInfo) error
		teardown func(e *ETCD, ctxInfo *contextInfo) error
		wantErr  bool
	}{
		{
			name: "Start etcd without clientAccessInfo and without snapshots",
			fields: fields{
				config:  generateTestConfig(),
				address: mustGetAddress(),
				name:    "default",
			},
			args: args{
				clientAccessInfo: nil,
			},
			setup: func(e *ETCD, ctxInfo *contextInfo) error {
				ctxInfo.ctx, ctxInfo.cancel = context.WithCancel(context.Background())
				e.config.EtcdDisableSnapshots = true
				testutil.GenerateRuntime(e.config)
				return nil
			},
			teardown: func(e *ETCD, ctxInfo *contextInfo) error {
				// RemoveSelf will fail with a specific error, but it still does cleanup for testing purposes
				if err := e.RemoveSelf(ctxInfo.ctx); err != nil && err.Error() != etcdserver.ErrNotEnoughStartedMembers.Error() {
					return err
				}
				ctxInfo.cancel()
				time.Sleep(10 * time.Second)
				testutil.CleanupDataDir(e.config)
				return nil
			},
		},
		{
			name: "Start etcd without clientAccessInfo on",
			fields: fields{
				config:  generateTestConfig(),
				address: mustGetAddress(),
				name:    "default",
				cron:    cron.New(),
			},
			args: args{
				clientAccessInfo: nil,
			},
			setup: func(e *ETCD, ctxInfo *contextInfo) error {
				ctxInfo.ctx, ctxInfo.cancel = context.WithCancel(context.Background())
				testutil.GenerateRuntime(e.config)
				return nil
			},
			teardown: func(e *ETCD, ctxInfo *contextInfo) error {
				// RemoveSelf will fail with a specific error, but it still does cleanup for testing purposes
				if err := e.RemoveSelf(ctxInfo.ctx); err != nil && err.Error() != etcdserver.ErrNotEnoughStartedMembers.Error() {
					return err
				}
				ctxInfo.cancel()
				time.Sleep(5 * time.Second)
				testutil.CleanupDataDir(e.config)
				return nil
			},
		},
		{
			name: "Start etcd with an existing cluster",
			fields: fields{
				config:  generateTestConfig(),
				address: mustGetAddress(),
				name:    "default",
				cron:    cron.New(),
			},
			args: args{
				clientAccessInfo: nil,
			},
			setup: func(e *ETCD, ctxInfo *contextInfo) error {
				ctxInfo.ctx, ctxInfo.cancel = context.WithCancel(context.Background())
				if err := testutil.GenerateRuntime(e.config); err != nil {
					return err
				}
				return os.MkdirAll(walDir(e.config), 0700)
			},
			teardown: func(e *ETCD, ctxInfo *contextInfo) error {
				// RemoveSelf will fail with a specific error, but it still does cleanup for testing purposes
				if err := e.RemoveSelf(ctxInfo.ctx); err != nil && err.Error() != etcdserver.ErrNotEnoughStartedMembers.Error() {
					return err
				}
				ctxInfo.cancel()
				time.Sleep(5 * time.Second)
				testutil.CleanupDataDir(e.config)
				os.Remove(walDir(e.config))
				return nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &ETCD{
				client:  tt.fields.client,
				config:  tt.fields.config,
				name:    tt.fields.name,
				address: tt.fields.address,
				cron:    tt.fields.cron,
				s3:      tt.fields.s3,
			}

			if err := tt.setup(e, &tt.fields.context); err != nil {
				t.Errorf("Setup for ETCD.Start() failed = %v", err)
				return
			}
			if err := e.Start(tt.fields.context.ctx, tt.args.clientAccessInfo); (err != nil) != tt.wantErr {
				t.Errorf("ETCD.Start() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := tt.teardown(e, &tt.fields.context); err != nil {
				t.Errorf("Teardown for ETCD.Start() failed = %v", err)
				return
			}
		})
	}
}
