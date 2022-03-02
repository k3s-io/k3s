package bootstrap

import (
	"testing"

	"github.com/k3s-io/k3s/pkg/daemons/config"
)

func TestObjToMap(t *testing.T) {
	type args struct {
		obj interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr bool
	}{
		{
			name: "Minimal Valid",
			args: args{
				obj: &config.ControlRuntimeBootstrap{
					ServerCA:    "/var/lib/rancher/k3s/server/tls/server-ca.crt",
					ServerCAKey: "/var/lib/rancher/k3s/server/tls/server-ca.key",
				},
			},
			wantErr: false,
		},
		{
			name: "Minimal Invalid",
			args: args{
				obj: 1,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ObjToMap(tt.args.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("ObjToMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
