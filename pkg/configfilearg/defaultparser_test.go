package configfilearg

import (
	"os"
	"reflect"
	"testing"
)

func Test_UnitMustParse(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		config string
		want   []string
	}{
		{
			name: "Basic server",
			args: []string{"k3s", "server"},

			want: []string{"k3s", "server"},
		},
		{
			name: "Server with known flags",
			args: []string{"k3s", "server", "-t 12345", "--write-kubeconfig-mode 644"},

			want: []string{"k3s", "server", "-t 12345", "--write-kubeconfig-mode 644"},
		},
		{
			name:   "Server with known flags and config with known and unknown flags",
			args:   []string{"k3s", "server", "--write-kubeconfig-mode 644"},
			config: "./testdata/defaultdata.yaml",
			want: []string{"k3s", "server", "--token=12345", "--node-label=DEAFBEEF",
				"--etcd-s3=true", "--etcd-s3-bucket=my-backup", "--kubelet-arg=max-pods=999", "--write-kubeconfig-mode 644"},
		},
		{
			name: "Basic etcd-snapshot",
			args: []string{"k3s", "etcd-snapshot", "save"},

			want: []string{"k3s", "etcd-snapshot", "save"},
		},
		{
			name: "Etcd-snapshot with known flags",
			args: []string{"k3s", "etcd-snapshot", "save", "--s3=true"},

			want: []string{"k3s", "etcd-snapshot", "save", "--s3=true"},
		},
		{
			name:   "Etcd-snapshot with config with known and unknown flags",
			args:   []string{"k3s", "etcd-snapshot", "save"},
			config: "./testdata/defaultdata.yaml",
			want:   []string{"k3s", "etcd-snapshot", "save", "--etcd-s3=true", "--etcd-s3-bucket=my-backup"},
		},
		{
			name: "Agent with known flags",
			args: []string{"k3s", "agent", "--token=12345"},

			want: []string{"k3s", "agent", "--token=12345"},
		},
		{
			name:   "Agent with config with known and unknown flags, flags are not skipped",
			args:   []string{"k3s", "agent"},
			config: "./testdata/defaultdata.yaml",
			want: []string{"k3s", "agent", "--token=12345", "--node-label=DEAFBEEF",
				"--etcd-s3=true", "--etcd-s3-bucket=my-backup", "--notaflag=true", "--kubelet-arg=max-pods=999"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			DefaultParser.DefaultConfig = tt.config
			if got := MustParse(tt.args); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MustParse() = %+v\nWant = %+v", got, tt.want)
			}
		})
	}
}

func Test_UnitMustFindString(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		target   string
		setup    func() error // Optional, delete if unused
		teardown func() error // Optional, delete if unused
		want     string
	}{
		{
			name:   "Target not found in config file",
			args:   []string{"--foo", "bar"},
			target: "token",

			want: "",

			setup:    func() error { return os.Setenv("K3S_CONFIG_FILE", "./testdata/data.yaml") },
			teardown: func() error { return os.Unsetenv("K3S_CONFIG_FILE") },
		},
		{
			name:   "Target found in config file",
			args:   []string{"--foo", "bar"},
			target: "token",

			want: "12345",

			setup:    func() error { return os.Setenv("K3S_CONFIG_FILE", "./testdata/defaultdata.yaml") },
			teardown: func() error { return os.Unsetenv("K3S_CONFIG_FILE") },
		},
		{
			name:   "Override flag found, function is short-circuited",
			args:   []string{"--foo", "bar", "-h"},
			target: "token",

			want: "-h",

			setup:    func() error { return os.Setenv("K3S_CONFIG_FILE", "./testdata/defaultdata.yaml") },
			teardown: func() error { return os.Unsetenv("K3S_CONFIG_FILE") },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer tt.teardown()
			if err := tt.setup(); err != nil {
				t.Errorf("Setup for MustFindString() failed = %v", err)
				return
			}
			if got := MustFindString(tt.args, tt.target); got != tt.want {
				t.Errorf("MustFindString() = %+v\nWant = %+v", got, tt.want)
			}
		})
	}
}
