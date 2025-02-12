//go:build linux
// +build linux

package etcd

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/k3s-io/k3s/pkg/clientaccess"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/etcd/s3"
	testutil "github.com/k3s-io/k3s/tests"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/wait"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

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
			name: "directory exists",
			args: args{
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
			name: "directory does not exist",
			args: args{
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
			name: "standard config",
			args: args{
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
			name: "with a tombstone file created",
			args: args{
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
	// dummy supervisor API for testing
	var memberAddr string
	server := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/db/info" {
			members := []*etcdserverpb.Member{{
				ClientURLs: []string{"https://" + net.JoinHostPort(memberAddr, "2379")},
				PeerURLs:   []string{"https://" + net.JoinHostPort(memberAddr, "2380")},
			}}
			resp.Header().Set("Content-Type", "application/json")
			json.NewEncoder(resp).Encode(&Members{
				Members: members,
			})
		}
	}))
	defer server.Close()

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
			name: "nil clientAccessInfo and nil cron",
			fields: fields{
				config:  generateTestConfig(),
				address: mustGetAddress(),
				name:    "default",
			},
			setup: func(e *ETCD, ctxInfo *contextInfo) error {
				ctxInfo.ctx, ctxInfo.cancel = context.WithCancel(context.Background())
				e.config.EtcdDisableSnapshots = true
				testutil.GenerateRuntime(e.config)
				return nil
			},
			teardown: func(e *ETCD, ctxInfo *contextInfo) error {
				// RemoveSelf will fail with a specific error, but it still does cleanup for testing purposes
				err := e.RemoveSelf(ctxInfo.ctx)
				ctxInfo.cancel()
				time.Sleep(5 * time.Second)
				testutil.CleanupDataDir(e.config)
				if err != nil && err.Error() != etcdserver.ErrNotEnoughStartedMembers.Error() {
					return err
				}
				return nil
			},
		},
		{
			name: "nil clientAccessInfo",
			fields: fields{
				config:  generateTestConfig(),
				address: mustGetAddress(),
				name:    "default",
				cron:    cron.New(),
			},
			setup: func(e *ETCD, ctxInfo *contextInfo) error {
				ctxInfo.ctx, ctxInfo.cancel = context.WithCancel(context.Background())
				testutil.GenerateRuntime(e.config)
				return nil
			},
			teardown: func(e *ETCD, ctxInfo *contextInfo) error {
				// RemoveSelf will fail with a specific error, but it still does cleanup for testing purposes
				err := e.RemoveSelf(ctxInfo.ctx)
				ctxInfo.cancel()
				time.Sleep(5 * time.Second)
				testutil.CleanupDataDir(e.config)
				if err != nil && err.Error() != etcdserver.ErrNotEnoughStartedMembers.Error() {
					return err
				}
				return nil
			},
		},
		{
			name: "valid clientAccessInfo",
			fields: fields{
				config:  generateTestConfig(),
				address: mustGetAddress(),
				name:    "default",
				cron:    cron.New(),
			},
			args: args{
				clientAccessInfo: &clientaccess.Info{
					BaseURL:  "http://" + server.Listener.Addr().String(),
					Username: "server",
					Password: "token",
				},
			},
			setup: func(e *ETCD, ctxInfo *contextInfo) error {
				ctxInfo.ctx, ctxInfo.cancel = context.WithCancel(context.Background())
				testutil.GenerateRuntime(e.config)
				return nil
			},
			teardown: func(e *ETCD, ctxInfo *contextInfo) error {
				// RemoveSelf will fail with a specific error, but it still does cleanup for testing purposes
				err := e.RemoveSelf(ctxInfo.ctx)
				ctxInfo.cancel()
				time.Sleep(5 * time.Second)
				testutil.CleanupDataDir(e.config)
				if err != nil && err.Error() != etcdserver.ErrNotEnoughStartedMembers.Error() {
					return err
				}
				return nil
			},
		},
		{
			name: "existing cluster",
			fields: fields{
				config:  generateTestConfig(),
				address: mustGetAddress(),
				name:    "default",
				cron:    cron.New(),
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
				err := e.RemoveSelf(ctxInfo.ctx)
				ctxInfo.cancel()
				time.Sleep(5 * time.Second)
				testutil.CleanupDataDir(e.config)
				os.Remove(walDir(e.config))
				if err != nil && err.Error() != etcdserver.ErrNotEnoughStartedMembers.Error() {
					return err
				}
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
			if !tt.wantErr {
				memberAddr = e.address
				if err := wait.PollUntilContextTimeout(tt.fields.context.ctx, time.Second, time.Minute, true, func(ctx context.Context) (bool, error) {
					if _, err := e.getETCDStatus(tt.fields.context.ctx, ""); err != nil {
						t.Logf("Waiting to get etcd status: %v", err)
						return false, nil
					}
					return true, nil
				}); err != nil {
					t.Errorf("Failed to get etcd status: %v", err)
				}
			}
			if err := tt.teardown(e, &tt.fields.context); err != nil {
				t.Errorf("Teardown for ETCD.Start() failed = %v", err)
			}
		})
	}
}

func Test_UnitETCD_Test(t *testing.T) {
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
	}
	type args struct {
		clientAccessInfo *clientaccess.Info
	}
	tests := []struct {
		name     string
		fields   fields
		setup    func(e *ETCD, ctxInfo *contextInfo) error
		teardown func(e *ETCD, ctxInfo *contextInfo) error
		wantErr  bool
	}{
		{
			name: "no server running",
			fields: fields{
				config:  generateTestConfig(),
				address: mustGetAddress(),
				name:    "default",
			},
			setup: func(e *ETCD, ctxInfo *contextInfo) error {
				ctxInfo.ctx, ctxInfo.cancel = context.WithCancel(context.Background())
				testutil.GenerateRuntime(e.config)
				return e.startClient(ctxInfo.ctx)
			},
			teardown: func(e *ETCD, ctxInfo *contextInfo) error {
				ctxInfo.cancel()
				time.Sleep(1 * time.Second)
				testutil.CleanupDataDir(e.config)
				return nil
			},
			wantErr: true,
		},
		{
			name: "unreachable server",
			fields: fields{
				config:  generateTestConfig(),
				address: mustGetAddress(),
				name:    "default",
			},
			setup: func(e *ETCD, ctxInfo *contextInfo) error {
				ctxInfo.ctx, ctxInfo.cancel = context.WithCancel(context.Background())
				testutil.GenerateRuntime(e.config)
				e.config.Runtime.EtcdConfig.Endpoints = []string{"https://192.0.2.0:2379"} // RFC5737
				return e.startClient(ctxInfo.ctx)
			},
			teardown: func(e *ETCD, ctxInfo *contextInfo) error {
				ctxInfo.cancel()
				time.Sleep(1 * time.Second)
				testutil.CleanupDataDir(e.config)
				return nil
			},
			wantErr: true,
		},
		{
			name: "learner server",
			fields: fields{
				config:  generateTestConfig(),
				address: mustGetAddress(),
				name:    "default",
			},
			setup: func(e *ETCD, ctxInfo *contextInfo) error {
				ctxInfo.ctx, ctxInfo.cancel = context.WithCancel(context.Background())
				testutil.GenerateRuntime(e.config)
				if err := startMock(ctxInfo.ctx, e, true, false, false, time.Second); err != nil {
					return err
				}
				return e.startClient(ctxInfo.ctx)
			},
			teardown: func(e *ETCD, ctxInfo *contextInfo) error {
				ctxInfo.cancel()
				time.Sleep(1 * time.Second)
				testutil.CleanupDataDir(e.config)
				return nil
			},
			wantErr: true,
		},
		{
			name: "corrupt server",
			fields: fields{
				config:  generateTestConfig(),
				address: mustGetAddress(),
				name:    "default",
			},
			setup: func(e *ETCD, ctxInfo *contextInfo) error {
				ctxInfo.ctx, ctxInfo.cancel = context.WithCancel(context.Background())
				testutil.GenerateRuntime(e.config)
				if err := startMock(ctxInfo.ctx, e, false, true, false, time.Second); err != nil {
					return err
				}
				return e.startClient(ctxInfo.ctx)
			},
			teardown: func(e *ETCD, ctxInfo *contextInfo) error {
				ctxInfo.cancel()
				time.Sleep(1 * time.Second)
				testutil.CleanupDataDir(e.config)
				return nil
			},
			wantErr: true,
		},
		{
			name: "leaderless server",
			fields: fields{
				config:  generateTestConfig(),
				address: mustGetAddress(),
				name:    "default",
			},
			setup: func(e *ETCD, ctxInfo *contextInfo) error {
				ctxInfo.ctx, ctxInfo.cancel = context.WithCancel(context.Background())
				testutil.GenerateRuntime(e.config)
				if err := startMock(ctxInfo.ctx, e, false, false, true, time.Second); err != nil {
					return err
				}
				return e.startClient(ctxInfo.ctx)
			},
			teardown: func(e *ETCD, ctxInfo *contextInfo) error {
				ctxInfo.cancel()
				time.Sleep(1 * time.Second)
				testutil.CleanupDataDir(e.config)
				return nil
			},
			wantErr: true,
		},
		{
			name: "normal server",
			fields: fields{
				config:  generateTestConfig(),
				address: mustGetAddress(),
				name:    "default",
			},
			setup: func(e *ETCD, ctxInfo *contextInfo) error {
				ctxInfo.ctx, ctxInfo.cancel = context.WithCancel(context.Background())
				testutil.GenerateRuntime(e.config)
				if err := startMock(ctxInfo.ctx, e, false, false, false, time.Second); err != nil {
					return err
				}
				return e.startClient(ctxInfo.ctx)
			},
			teardown: func(e *ETCD, ctxInfo *contextInfo) error {
				ctxInfo.cancel()
				time.Sleep(1 * time.Second)
				testutil.CleanupDataDir(e.config)
				return nil
			},
			wantErr: false,
		},
		{
			name: "alarm on other server",
			fields: fields{
				config:  generateTestConfig(),
				address: mustGetAddress(),
				name:    "default",
			},
			setup: func(e *ETCD, ctxInfo *contextInfo) error {
				ctxInfo.ctx, ctxInfo.cancel = context.WithCancel(context.Background())
				testutil.GenerateRuntime(e.config)
				extraAlarm := &etcdserverpb.AlarmMember{MemberID: 2, Alarm: etcdserverpb.AlarmType_NOSPACE}
				if err := startMock(ctxInfo.ctx, e, false, false, false, time.Second, extraAlarm); err != nil {
					return err
				}
				return e.startClient(ctxInfo.ctx)
			},
			teardown: func(e *ETCD, ctxInfo *contextInfo) error {
				ctxInfo.cancel()
				time.Sleep(1 * time.Second)
				testutil.CleanupDataDir(e.config)
				return nil
			},
			wantErr: false,
		},
		{
			name: "slow defrag",
			fields: fields{
				config:  generateTestConfig(),
				address: mustGetAddress(),
				name:    "default",
			},
			setup: func(e *ETCD, ctxInfo *contextInfo) error {
				ctxInfo.ctx, ctxInfo.cancel = context.WithCancel(context.Background())
				testutil.GenerateRuntime(e.config)
				if err := startMock(ctxInfo.ctx, e, false, false, false, 40*time.Second); err != nil {
					return err
				}
				return e.startClient(ctxInfo.ctx)
			},
			teardown: func(e *ETCD, ctxInfo *contextInfo) error {
				ctxInfo.cancel()
				time.Sleep(1 * time.Second)
				testutil.CleanupDataDir(e.config)
				return nil
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &ETCD{
				client:  tt.fields.client,
				config:  tt.fields.config,
				name:    tt.fields.name,
				address: tt.fields.address,
			}

			if err := tt.setup(e, &tt.fields.context); err != nil {
				t.Errorf("Setup for ETCD.Test() failed = %v", err)
				return
			}
			start := time.Now()
			err := e.Test(tt.fields.context.ctx)
			duration := time.Now().Sub(start)
			t.Logf("ETCD.Test() completed in %v with err=%v", duration, err)
			if (err != nil) != tt.wantErr {
				t.Errorf("ETCD.Test() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := tt.teardown(e, &tt.fields.context); err != nil {
				t.Errorf("Teardown for ETCD.Test() failed = %v", err)
			}
		})
	}
}

// startMock starts up a mock etcd grpc service with canned responses
// that can be used to test specific scenarios.
func startMock(ctx context.Context, e *ETCD, isLearner, isCorrupt, noLeader bool, defragDelay time.Duration, extraAlarms ...*etcdserverpb.AlarmMember) error {
	address := authority(getEndpoints(e.config)[0])
	// listen on endpoint and close listener on context cancel
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	// set up tls if enabled
	gopts := []grpc.ServerOption{}
	if e.config.Datastore.ServerTLSConfig.CertFile != "" && e.config.Datastore.ServerTLSConfig.KeyFile != "" {
		creds, err := credentials.NewServerTLSFromFile(e.config.Datastore.ServerTLSConfig.CertFile, e.config.Datastore.ServerTLSConfig.KeyFile)
		if err != nil {
			return err
		}
		gopts = append(gopts, grpc.Creds(creds))
	}
	server := grpc.NewServer(gopts...)

	mock := &mockEtcd{
		e:           e,
		mu:          &sync.RWMutex{},
		isLearner:   isLearner,
		isCorrupt:   isCorrupt,
		noLeader:    noLeader,
		defragDelay: defragDelay,
		extraAlarms: extraAlarms,
	}

	// register grpc services
	etcdserverpb.RegisterKVServer(server, mock)
	etcdserverpb.RegisterClusterServer(server, mock)
	etcdserverpb.RegisterMaintenanceServer(server, mock)

	hsrv := health.NewServer()
	hsrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(server, hsrv)

	reflection.Register(server)

	// shutdown on context cancel
	go func() {
		<-ctx.Done()
		server.GracefulStop()
		listener.Close()
	}()

	// start serving
	go func() {
		logrus.Infof("Mock etcd server starting on %s", listener.Addr())
		logrus.Infof("Mock etcd server exited: %v", server.Serve(listener))
	}()

	return nil
}

type mockEtcd struct {
	e           *ETCD
	mu          *sync.RWMutex
	calls       map[string]int
	isLearner   bool
	isCorrupt   bool
	noLeader    bool
	defragDelay time.Duration
	extraAlarms []*etcdserverpb.AlarmMember
}

// increment call counter for this function
func (m *mockEtcd) inc(call string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.calls == nil {
		m.calls = map[string]int{}
	}
	m.calls[call] = m.calls[call] + 1
}

// get call counter for this function
func (m *mockEtcd) get(call string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.calls[call]
}

// get alarm list
func (m *mockEtcd) alarms() []*etcdserverpb.AlarmMember {
	alarms := m.extraAlarms
	if m.get("alarm") < 2 {
		// on the first check, return NOSPACE so that we can clear it after defragging
		alarms = append(alarms, &etcdserverpb.AlarmMember{
			Alarm:    etcdserverpb.AlarmType_NOSPACE,
			MemberID: 1,
		})
	}
	if m.isCorrupt {
		// return CORRUPT if so requested
		alarms = append(alarms, &etcdserverpb.AlarmMember{
			Alarm:    etcdserverpb.AlarmType_CORRUPT,
			MemberID: 1,
		})
	}
	return alarms
}

// KV mocks
func (m *mockEtcd) Range(context.Context, *etcdserverpb.RangeRequest) (*etcdserverpb.RangeResponse, error) {
	m.inc("range")
	return nil, unsupported("range")
}
func (m *mockEtcd) Put(context.Context, *etcdserverpb.PutRequest) (*etcdserverpb.PutResponse, error) {
	m.inc("put")
	return nil, unsupported("put")
}
func (m *mockEtcd) DeleteRange(context.Context, *etcdserverpb.DeleteRangeRequest) (*etcdserverpb.DeleteRangeResponse, error) {
	m.inc("deleterange")
	return nil, unsupported("deleterange")
}
func (m *mockEtcd) Txn(context.Context, *etcdserverpb.TxnRequest) (*etcdserverpb.TxnResponse, error) {
	m.inc("txn")
	return nil, unsupported("txn")
}
func (m *mockEtcd) Compact(context.Context, *etcdserverpb.CompactionRequest) (*etcdserverpb.CompactionResponse, error) {
	m.inc("compact")
	return nil, unsupported("compact")
}

// Maintenance mocks
func (m *mockEtcd) Alarm(ctx context.Context, r *etcdserverpb.AlarmRequest) (*etcdserverpb.AlarmResponse, error) {
	m.inc("alarm")
	res := &etcdserverpb.AlarmResponse{
		Header: &etcdserverpb.ResponseHeader{
			MemberId: 1,
		},
	}
	if r.Action == etcdserverpb.AlarmRequest_GET {
		res.Alarms = m.alarms()
	}
	return res, nil
}
func (m *mockEtcd) Status(context.Context, *etcdserverpb.StatusRequest) (*etcdserverpb.StatusResponse, error) {
	m.inc("status")
	res := &etcdserverpb.StatusResponse{
		Header: &etcdserverpb.ResponseHeader{
			MemberId: 1,
		},
		Leader:      1,
		Version:     "v3.5.0-mock0",
		DbSize:      1024,
		DbSizeInUse: 512,
		IsLearner:   m.isLearner,
	}
	if m.noLeader {
		res.Leader = 0
		res.Errors = append(res.Errors, etcdserver.ErrNoLeader.Error())
	}
	for _, a := range m.alarms() {
		res.Errors = append(res.Errors, a.String())
	}
	return res, nil
}
func (m *mockEtcd) Defragment(ctx context.Context, r *etcdserverpb.DefragmentRequest) (*etcdserverpb.DefragmentResponse, error) {
	m.inc("defragment")
	// delay defrag response by configured time, or until the request is cancelled
	select {
	case <-ctx.Done():
	case <-time.After(m.defragDelay):
	}
	return &etcdserverpb.DefragmentResponse{
		Header: &etcdserverpb.ResponseHeader{
			MemberId: 1,
		},
	}, nil
}
func (m *mockEtcd) Hash(context.Context, *etcdserverpb.HashRequest) (*etcdserverpb.HashResponse, error) {
	m.inc("hash")
	return nil, unsupported("hash")
}
func (m *mockEtcd) HashKV(context.Context, *etcdserverpb.HashKVRequest) (*etcdserverpb.HashKVResponse, error) {
	m.inc("hashkv")
	return nil, unsupported("hashkv")
}
func (m *mockEtcd) Snapshot(*etcdserverpb.SnapshotRequest, etcdserverpb.Maintenance_SnapshotServer) error {
	m.inc("snapshot")
	return unsupported("snapshot")
}
func (m *mockEtcd) MoveLeader(context.Context, *etcdserverpb.MoveLeaderRequest) (*etcdserverpb.MoveLeaderResponse, error) {
	m.inc("moveleader")
	return nil, unsupported("moveleader")
}
func (m *mockEtcd) Downgrade(context.Context, *etcdserverpb.DowngradeRequest) (*etcdserverpb.DowngradeResponse, error) {
	m.inc("downgrade")
	return nil, unsupported("downgrade")
}

// Cluster mocks
func (m *mockEtcd) MemberAdd(context.Context, *etcdserverpb.MemberAddRequest) (*etcdserverpb.MemberAddResponse, error) {
	m.inc("memberadd")
	return nil, unsupported("memberadd")
}
func (m *mockEtcd) MemberRemove(context.Context, *etcdserverpb.MemberRemoveRequest) (*etcdserverpb.MemberRemoveResponse, error) {
	m.inc("memberremove")
	return nil, etcdserver.ErrNotEnoughStartedMembers
}
func (m *mockEtcd) MemberUpdate(context.Context, *etcdserverpb.MemberUpdateRequest) (*etcdserverpb.MemberUpdateResponse, error) {
	m.inc("memberupdate")
	return nil, unsupported("memberupdate")
}
func (m *mockEtcd) MemberList(context.Context, *etcdserverpb.MemberListRequest) (*etcdserverpb.MemberListResponse, error) {
	m.inc("memberlist")
	scheme := "http"
	if m.e.config.Datastore.ServerTLSConfig.CertFile != "" {
		scheme = "https"
	}

	return &etcdserverpb.MemberListResponse{
		Header: &etcdserverpb.ResponseHeader{
			MemberId: 1,
		},
		Members: []*etcdserverpb.Member{
			{
				ID:         1,
				Name:       m.e.name,
				IsLearner:  m.isLearner,
				ClientURLs: []string{scheme + "://127.0.0.1:2379"},
				PeerURLs:   []string{scheme + "://" + m.e.address + ":2380"},
			},
		},
	}, nil
}

func (m *mockEtcd) MemberPromote(context.Context, *etcdserverpb.MemberPromoteRequest) (*etcdserverpb.MemberPromoteResponse, error) {
	m.inc("memberpromote")
	return nil, unsupported("memberpromote")
}

func unsupported(field string) error {
	return status.New(codes.Unimplemented, field+" is not implemented").Err()
}
