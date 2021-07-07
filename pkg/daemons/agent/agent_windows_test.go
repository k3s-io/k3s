// +build windows

package agent

import (
	"testing"

	"github.com/rancher/k3s/pkg/daemons/config"
)

func TestCheckRuntimeEndpoint(t *testing.T) {
	type args struct {
		cfg *config.Agent
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Runtime endpoint unaltered",
			args: args{
				cfg: &config.Agent{RuntimeSocket: "npipe:////./pipe/containerd-containerd"},
			},
			want: "npipe:////./pipe/containerd-containerd",
		},
		{
			name: "Runtime endpoint altered",
			args: args{
				cfg: &config.Agent{RuntimeSocket: "//./pipe/containerd-containerd"},
			},
			want: "npipe:////./pipe/containerd-containerd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			argsMap := map[string]string{}
			checkRuntimeEndpoint(tt.args.cfg, argsMap)
			if argsMap["container-runtime-endpoint"] != tt.want {
				got := argsMap["container-runtime-endpoint"]
				t.Errorf("error, input was " + tt.args.cfg.RuntimeSocket + " should be " + tt.want + ", but got " + got)
			}
		})

	}
}
