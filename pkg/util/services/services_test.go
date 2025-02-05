package services

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/control/deps"
)

func Test_UnitFilesForServices(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "k3s")
	serverDir := filepath.Join(dataDir, "server")
	agentDir := filepath.Join(dataDir, "agent")
	type args struct {
		controlConfig config.Control
		services      []string
	}
	tests := []struct {
		name    string
		args    args
		setup   func(controlConfig *config.Control) error
		want    map[string][]string
		wantErr bool
	}{
		{
			name: "All Services",
			args: args{
				services: All,
				controlConfig: config.Control{
					DataDir: serverDir,
					Runtime: &config.ControlRuntime{},
				},
			},
			setup: func(controlConfig *config.Control) error {
				deps.CreateRuntimeCertFiles(controlConfig)
				return nil
			},
			want: map[string][]string{
				"admin": []string{
					filepath.Join(serverDir, "tls", "client-admin.crt"),
					filepath.Join(serverDir, "tls", "client-admin.key"),
				},
				"api-server": []string{
					filepath.Join(serverDir, "tls", "client-kube-apiserver.crt"),
					filepath.Join(serverDir, "tls", "client-kube-apiserver.key"),
					filepath.Join(serverDir, "tls", "serving-kube-apiserver.crt"),
					filepath.Join(serverDir, "tls", "serving-kube-apiserver.key"),
				},
				"auth-proxy": []string{
					filepath.Join(serverDir, "tls", "client-auth-proxy.crt"),
					filepath.Join(serverDir, "tls", "client-auth-proxy.key"),
				},
				"cloud-controller": []string{
					filepath.Join(serverDir, "tls", "client-k3s-cloud-controller.crt"),
					filepath.Join(serverDir, "tls", "client-k3s-cloud-controller.key"),
				},
				"controller-manager": []string{
					filepath.Join(serverDir, "tls", "client-controller.crt"),
					filepath.Join(serverDir, "tls", "client-controller.key"),
				},
				"etcd": []string{
					filepath.Join(serverDir, "tls", "etcd", "client.crt"),
					filepath.Join(serverDir, "tls", "etcd", "client.key"),
					filepath.Join(serverDir, "tls", "etcd", "server-client.crt"),
					filepath.Join(serverDir, "tls", "etcd", "server-client.key"),
					filepath.Join(serverDir, "tls", "etcd", "peer-server-client.crt"),
					filepath.Join(serverDir, "tls", "etcd", "peer-server-client.key"),
				},
				"k3s-controller": []string{
					filepath.Join(serverDir, "tls", "client-k3s-controller.crt"),
					filepath.Join(serverDir, "tls", "client-k3s-controller.key"),
					filepath.Join(agentDir, "client-k3s-controller.crt"),
					filepath.Join(agentDir, "client-k3s-controller.key"),
				},
				"kube-proxy": []string{
					filepath.Join(serverDir, "tls", "client-kube-proxy.crt"),
					filepath.Join(serverDir, "tls", "client-kube-proxy.key"),
					filepath.Join(agentDir, "client-kube-proxy.crt"),
					filepath.Join(agentDir, "client-kube-proxy.key"),
				},
				"kubelet": []string{
					filepath.Join(serverDir, "tls", "client-kubelet.key"),
					filepath.Join(serverDir, "tls", "serving-kubelet.key"),
					filepath.Join(agentDir, "client-kubelet.crt"),
					filepath.Join(agentDir, "client-kubelet.key"),
					filepath.Join(agentDir, "serving-kubelet.crt"),
					filepath.Join(agentDir, "serving-kubelet.key"),
				},
				"scheduler": []string{
					filepath.Join(serverDir, "tls", "client-scheduler.crt"),
					filepath.Join(serverDir, "tls", "client-scheduler.key"),
				},
				"supervisor": []string{
					filepath.Join(serverDir, "tls", "client-supervisor.crt"),
					filepath.Join(serverDir, "tls", "client-supervisor.key"),
				},
			},
		},
		{
			name: "Server Only",
			args: args{
				services: Server,
				controlConfig: config.Control{
					DataDir: serverDir,
					Runtime: &config.ControlRuntime{},
				},
			},
			setup: func(controlConfig *config.Control) error {
				deps.CreateRuntimeCertFiles(controlConfig)
				return nil
			},
			want: map[string][]string{
				"admin": []string{
					filepath.Join(serverDir, "tls", "client-admin.crt"),
					filepath.Join(serverDir, "tls", "client-admin.key"),
				},
				"api-server": []string{
					filepath.Join(serverDir, "tls", "client-kube-apiserver.crt"),
					filepath.Join(serverDir, "tls", "client-kube-apiserver.key"),
					filepath.Join(serverDir, "tls", "serving-kube-apiserver.crt"),
					filepath.Join(serverDir, "tls", "serving-kube-apiserver.key"),
				},
				"auth-proxy": []string{
					filepath.Join(serverDir, "tls", "client-auth-proxy.crt"),
					filepath.Join(serverDir, "tls", "client-auth-proxy.key"),
				},
				"cloud-controller": []string{
					filepath.Join(serverDir, "tls", "client-k3s-cloud-controller.crt"),
					filepath.Join(serverDir, "tls", "client-k3s-cloud-controller.key"),
				},
				"controller-manager": []string{
					filepath.Join(serverDir, "tls", "client-controller.crt"),
					filepath.Join(serverDir, "tls", "client-controller.key"),
				},
				"etcd": []string{
					filepath.Join(serverDir, "tls", "etcd", "client.crt"),
					filepath.Join(serverDir, "tls", "etcd", "client.key"),
					filepath.Join(serverDir, "tls", "etcd", "server-client.crt"),
					filepath.Join(serverDir, "tls", "etcd", "server-client.key"),
					filepath.Join(serverDir, "tls", "etcd", "peer-server-client.crt"),
					filepath.Join(serverDir, "tls", "etcd", "peer-server-client.key"),
				},
				"scheduler": []string{
					filepath.Join(serverDir, "tls", "client-scheduler.crt"),
					filepath.Join(serverDir, "tls", "client-scheduler.key"),
				},
				"supervisor": []string{
					filepath.Join(serverDir, "tls", "client-supervisor.crt"),
					filepath.Join(serverDir, "tls", "client-supervisor.key"),
				},
			},
		},
		{
			name: "Agent Only",
			args: args{
				services: Agent,
				controlConfig: config.Control{
					DataDir: serverDir,
					Runtime: &config.ControlRuntime{},
				},
			},
			setup: func(controlConfig *config.Control) error {
				deps.CreateRuntimeCertFiles(controlConfig)
				return nil
			},
			want: map[string][]string{
				"k3s-controller": []string{
					filepath.Join(serverDir, "tls", "client-k3s-controller.crt"),
					filepath.Join(serverDir, "tls", "client-k3s-controller.key"),
					filepath.Join(agentDir, "client-k3s-controller.crt"),
					filepath.Join(agentDir, "client-k3s-controller.key"),
				},
				"kube-proxy": []string{
					filepath.Join(serverDir, "tls", "client-kube-proxy.crt"),
					filepath.Join(serverDir, "tls", "client-kube-proxy.key"),
					filepath.Join(agentDir, "client-kube-proxy.crt"),
					filepath.Join(agentDir, "client-kube-proxy.key"),
				},
				"kubelet": []string{
					filepath.Join(serverDir, "tls", "client-kubelet.key"),
					filepath.Join(serverDir, "tls", "serving-kubelet.key"),
					filepath.Join(agentDir, "client-kubelet.crt"),
					filepath.Join(agentDir, "client-kubelet.key"),
					filepath.Join(agentDir, "serving-kubelet.crt"),
					filepath.Join(agentDir, "serving-kubelet.key"),
				},
			},
		},
		{
			name: "Invalid",
			args: args{
				services: []string{CertificateAuthority},
				controlConfig: config.Control{
					DataDir: serverDir,
					Runtime: &config.ControlRuntime{},
				},
			},
			setup: func(controlConfig *config.Control) error {
				deps.CreateRuntimeCertFiles(controlConfig)
				return nil
			},
			want: map[string][]string{
				"certificate-authority": []string{
					filepath.Join(serverDir, "tls", "server-ca.crt"),
					filepath.Join(serverDir, "tls", "server-ca.key"),
					filepath.Join(serverDir, "tls", "client-ca.crt"),
					filepath.Join(serverDir, "tls", "client-ca.key"),
					filepath.Join(serverDir, "tls", "request-header-ca.crt"),
					filepath.Join(serverDir, "tls", "request-header-ca.key"),
					filepath.Join(serverDir, "tls", "etcd", "peer-ca.crt"),
					filepath.Join(serverDir, "tls", "etcd", "peer-ca.key"),
					filepath.Join(serverDir, "tls", "etcd", "server-ca.crt"),
					filepath.Join(serverDir, "tls", "etcd", "server-ca.key"),
				},
			},
		},
		{
			name: "Invalid",
			args: args{
				services: []string{"foo"},
				controlConfig: config.Control{
					DataDir: serverDir,
					Runtime: &config.ControlRuntime{},
				},
			},
			setup: func(controlConfig *config.Control) error {
				deps.CreateRuntimeCertFiles(controlConfig)
				return nil
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.setup(&tt.args.controlConfig); err != nil {
				t.Errorf("Setup for FilesForServices() failed = %v", err)
				return
			}
			got, err := FilesForServices(tt.args.controlConfig, tt.args.services)
			if (err != nil) != tt.wantErr {
				t.Errorf("FilesForServices() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FilesForServices() = %+v\nWant = %+v", got, tt.want)
			}
		})
	}
}
