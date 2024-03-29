package services

import (
	"reflect"
	"testing"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/control/deps"
)

func Test_UnitFilesForServices(t *testing.T) {
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
					DataDir: "/var/lib/rancher/k3s/server",
					Runtime: &config.ControlRuntime{},
				},
			},
			setup: func(controlConfig *config.Control) error {
				deps.CreateRuntimeCertFiles(controlConfig)
				return nil
			},
			want: map[string][]string{
				"admin": []string{
					"/var/lib/rancher/k3s/server/tls/client-admin.crt",
					"/var/lib/rancher/k3s/server/tls/client-admin.key",
				},
				"api-server": []string{
					"/var/lib/rancher/k3s/server/tls/client-kube-apiserver.crt",
					"/var/lib/rancher/k3s/server/tls/client-kube-apiserver.key",
					"/var/lib/rancher/k3s/server/tls/serving-kube-apiserver.crt",
					"/var/lib/rancher/k3s/server/tls/serving-kube-apiserver.key",
				},
				"auth-proxy": []string{
					"/var/lib/rancher/k3s/server/tls/client-auth-proxy.crt",
					"/var/lib/rancher/k3s/server/tls/client-auth-proxy.key",
				},
				"cloud-controller": []string{
					"/var/lib/rancher/k3s/server/tls/client-k3s-cloud-controller.crt",
					"/var/lib/rancher/k3s/server/tls/client-k3s-cloud-controller.key",
				},
				"controller-manager": []string{
					"/var/lib/rancher/k3s/server/tls/client-controller.crt",
					"/var/lib/rancher/k3s/server/tls/client-controller.key",
				},
				"etcd": []string{
					"/var/lib/rancher/k3s/server/tls/etcd/client.crt",
					"/var/lib/rancher/k3s/server/tls/etcd/client.key",
					"/var/lib/rancher/k3s/server/tls/etcd/server-client.crt",
					"/var/lib/rancher/k3s/server/tls/etcd/server-client.key",
					"/var/lib/rancher/k3s/server/tls/etcd/peer-server-client.crt",
					"/var/lib/rancher/k3s/server/tls/etcd/peer-server-client.key",
				},
				"k3s-controller": []string{
					"/var/lib/rancher/k3s/server/tls/client-k3s-controller.crt",
					"/var/lib/rancher/k3s/server/tls/client-k3s-controller.key",
					"/var/lib/rancher/k3s/agent/client-k3s-controller.crt",
					"/var/lib/rancher/k3s/agent/client-k3s-controller.key",
				},
				"kube-proxy": []string{
					"/var/lib/rancher/k3s/server/tls/client-kube-proxy.crt",
					"/var/lib/rancher/k3s/server/tls/client-kube-proxy.key",
					"/var/lib/rancher/k3s/agent/client-kube-proxy.crt",
					"/var/lib/rancher/k3s/agent/client-kube-proxy.key",
				},
				"kubelet": []string{
					"/var/lib/rancher/k3s/server/tls/client-kubelet.key",
					"/var/lib/rancher/k3s/server/tls/serving-kubelet.key",
					"/var/lib/rancher/k3s/agent/client-kubelet.crt",
					"/var/lib/rancher/k3s/agent/client-kubelet.key",
					"/var/lib/rancher/k3s/agent/serving-kubelet.crt",
					"/var/lib/rancher/k3s/agent/serving-kubelet.key",
				},
				"scheduler": []string{
					"/var/lib/rancher/k3s/server/tls/client-scheduler.crt",
					"/var/lib/rancher/k3s/server/tls/client-scheduler.key",
				},
				"supervisor": []string{
					"/var/lib/rancher/k3s/server/tls/client-supervisor.crt",
					"/var/lib/rancher/k3s/server/tls/client-supervisor.key",
				},
			},
		},
		{
			name: "Server Only",
			args: args{
				services: Server,
				controlConfig: config.Control{
					DataDir: "/var/lib/rancher/k3s/server",
					Runtime: &config.ControlRuntime{},
				},
			},
			setup: func(controlConfig *config.Control) error {
				deps.CreateRuntimeCertFiles(controlConfig)
				return nil
			},
			want: map[string][]string{
				"admin": []string{
					"/var/lib/rancher/k3s/server/tls/client-admin.crt",
					"/var/lib/rancher/k3s/server/tls/client-admin.key",
				},
				"api-server": []string{
					"/var/lib/rancher/k3s/server/tls/client-kube-apiserver.crt",
					"/var/lib/rancher/k3s/server/tls/client-kube-apiserver.key",
					"/var/lib/rancher/k3s/server/tls/serving-kube-apiserver.crt",
					"/var/lib/rancher/k3s/server/tls/serving-kube-apiserver.key",
				},
				"auth-proxy": []string{
					"/var/lib/rancher/k3s/server/tls/client-auth-proxy.crt",
					"/var/lib/rancher/k3s/server/tls/client-auth-proxy.key",
				},
				"cloud-controller": []string{
					"/var/lib/rancher/k3s/server/tls/client-k3s-cloud-controller.crt",
					"/var/lib/rancher/k3s/server/tls/client-k3s-cloud-controller.key",
				},
				"controller-manager": []string{
					"/var/lib/rancher/k3s/server/tls/client-controller.crt",
					"/var/lib/rancher/k3s/server/tls/client-controller.key",
				},
				"etcd": []string{
					"/var/lib/rancher/k3s/server/tls/etcd/client.crt",
					"/var/lib/rancher/k3s/server/tls/etcd/client.key",
					"/var/lib/rancher/k3s/server/tls/etcd/server-client.crt",
					"/var/lib/rancher/k3s/server/tls/etcd/server-client.key",
					"/var/lib/rancher/k3s/server/tls/etcd/peer-server-client.crt",
					"/var/lib/rancher/k3s/server/tls/etcd/peer-server-client.key",
				},
				"scheduler": []string{
					"/var/lib/rancher/k3s/server/tls/client-scheduler.crt",
					"/var/lib/rancher/k3s/server/tls/client-scheduler.key",
				},
				"supervisor": []string{
					"/var/lib/rancher/k3s/server/tls/client-supervisor.crt",
					"/var/lib/rancher/k3s/server/tls/client-supervisor.key",
				},
			},
		},
		{
			name: "Agent Only",
			args: args{
				services: Agent,
				controlConfig: config.Control{
					DataDir: "/var/lib/rancher/k3s/server",
					Runtime: &config.ControlRuntime{},
				},
			},
			setup: func(controlConfig *config.Control) error {
				deps.CreateRuntimeCertFiles(controlConfig)
				return nil
			},
			want: map[string][]string{
				"k3s-controller": []string{
					"/var/lib/rancher/k3s/server/tls/client-k3s-controller.crt",
					"/var/lib/rancher/k3s/server/tls/client-k3s-controller.key",
					"/var/lib/rancher/k3s/agent/client-k3s-controller.crt",
					"/var/lib/rancher/k3s/agent/client-k3s-controller.key",
				},
				"kube-proxy": []string{
					"/var/lib/rancher/k3s/server/tls/client-kube-proxy.crt",
					"/var/lib/rancher/k3s/server/tls/client-kube-proxy.key",
					"/var/lib/rancher/k3s/agent/client-kube-proxy.crt",
					"/var/lib/rancher/k3s/agent/client-kube-proxy.key",
				},
				"kubelet": []string{
					"/var/lib/rancher/k3s/server/tls/client-kubelet.key",
					"/var/lib/rancher/k3s/server/tls/serving-kubelet.key",
					"/var/lib/rancher/k3s/agent/client-kubelet.crt",
					"/var/lib/rancher/k3s/agent/client-kubelet.key",
					"/var/lib/rancher/k3s/agent/serving-kubelet.crt",
					"/var/lib/rancher/k3s/agent/serving-kubelet.key",
				},
			},
		},
		{
			name: "Invalid",
			args: args{
				services: []string{CertificateAuthority},
				controlConfig: config.Control{
					DataDir: "/var/lib/rancher/k3s/server",
					Runtime: &config.ControlRuntime{},
				},
			},
			setup: func(controlConfig *config.Control) error {
				deps.CreateRuntimeCertFiles(controlConfig)
				return nil
			},
			want: map[string][]string{
				"certificate-authority": []string{
					"/var/lib/rancher/k3s/server/tls/server-ca.crt",
					"/var/lib/rancher/k3s/server/tls/server-ca.key",
					"/var/lib/rancher/k3s/server/tls/client-ca.crt",
					"/var/lib/rancher/k3s/server/tls/client-ca.key",
					"/var/lib/rancher/k3s/server/tls/request-header-ca.crt",
					"/var/lib/rancher/k3s/server/tls/request-header-ca.key",
					"/var/lib/rancher/k3s/server/tls/etcd/peer-ca.crt",
					"/var/lib/rancher/k3s/server/tls/etcd/peer-ca.key",
					"/var/lib/rancher/k3s/server/tls/etcd/server-ca.crt",
					"/var/lib/rancher/k3s/server/tls/etcd/server-ca.key",
				},
			},
		},
		{
			name: "Invalid",
			args: args{
				services: []string{"foo"},
				controlConfig: config.Control{
					DataDir: "/var/lib/rancher/k3s/server",
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
