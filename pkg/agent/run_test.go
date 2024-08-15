package agent

import (
	"reflect"
	"testing"
	"time"

	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	v1alpha1 "k8s.io/kube-proxy/config/v1alpha1"
	kubeproxyconfig "k8s.io/kubernetes/pkg/proxy/apis/config"
	kubeproxyconfigv1alpha1 "k8s.io/kubernetes/pkg/proxy/apis/config/v1alpha1"
	utilsptr "k8s.io/utils/ptr"
)

func Test_UnitGetConntrackConfig(t *testing.T) {
	// There are only helpers to default the typed config, so we have to set defaults on the typed config,
	// then convert it to the internal config representation in order to use it for tests.
	typedConfig := &v1alpha1.KubeProxyConfiguration{}
	defaultConfig := &kubeproxyconfig.KubeProxyConfiguration{}
	kubeproxyconfigv1alpha1.SetDefaults_KubeProxyConfiguration(typedConfig)
	if err := kubeproxyconfigv1alpha1.Convert_v1alpha1_KubeProxyConfiguration_To_config_KubeProxyConfiguration(typedConfig, defaultConfig, nil); err != nil {
		t.Fatalf("Failed to generate default KubeProxyConfiguration: %v", err)
	}

	customConfig := defaultConfig.DeepCopy()
	customConfig.Linux.Conntrack.Min = utilsptr.To(int32(100))
	customConfig.Linux.Conntrack.TCPCloseWaitTimeout.Duration = 42 * time.Second

	type args struct {
		nodeConfig *daemonconfig.Node
	}
	tests := []struct {
		name    string
		args    args
		want    *kubeproxyconfig.KubeProxyConntrackConfiguration
		wantErr bool
	}{
		{
			name: "Default args",
			args: args{
				nodeConfig: &daemonconfig.Node{
					AgentConfig: daemonconfig.Agent{
						ExtraKubeProxyArgs: []string{},
					},
				},
			},
			want:    &defaultConfig.Linux.Conntrack,
			wantErr: false,
		},
		{
			name: "Logging args",
			args: args{
				nodeConfig: &daemonconfig.Node{
					AgentConfig: daemonconfig.Agent{
						ExtraKubeProxyArgs: []string{"v=9"},
					},
				},
			},
			want:    &defaultConfig.Linux.Conntrack,
			wantErr: false,
		},
		{
			name: "Invalid args",
			args: args{
				nodeConfig: &daemonconfig.Node{
					AgentConfig: daemonconfig.Agent{
						ExtraKubeProxyArgs: []string{"conntrack-tcp-timeout-close-wait=invalid", "bogus=true"},
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Conntrack args",
			args: args{
				nodeConfig: &daemonconfig.Node{
					AgentConfig: daemonconfig.Agent{
						ExtraKubeProxyArgs: []string{"conntrack-tcp-timeout-close-wait=42s", "conntrack-min=100"},
					},
				},
			},
			want:    &customConfig.Linux.Conntrack,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getConntrackConfig(tt.args.nodeConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("getConntrackConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getConntrackConfig() = %+v\nWant = %+v", got, tt.want)
			}
		})
	}
}
