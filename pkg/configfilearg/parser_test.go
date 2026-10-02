package configfilearg

import (
	"os"
	"reflect"
	"testing"
)

func Test_UnitParser_findStart(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		prefix []string
		suffix []string
		found  bool
	}{
		{
			name:   "default case",
			args:   nil,
			prefix: nil,
			suffix: nil,
			found:  false,
		},
		{
			name:   "simple case",
			args:   []string{"server"},
			prefix: []string{"server"},
			suffix: []string{},
			found:  true,
		},
		{
			name:   "also simple case",
			args:   []string{"server", "foo"},
			prefix: []string{"server"},
			suffix: []string{"foo"},
			found:  true,
		},
		{
			name:   "longer simple case",
			args:   []string{"server", "foo", "bar"},
			prefix: []string{"server"},
			suffix: []string{"foo", "bar"},
			found:  true,
		},
		{
			name:   "not found",
			args:   []string{"not-server", "foo", "bar"},
			prefix: []string{"not-server", "foo", "bar"},
			found:  false,
		},
		{
			name:   "command (with optional subcommands) but no flags",
			args:   []string{"etcd-snapshot"},
			prefix: []string{"etcd-snapshot"},
			suffix: []string{},
			found:  true,
		},
		{
			name:   "command (with optional subcommands) and flags",
			args:   []string{"etcd-snapshot", "-f"},
			prefix: []string{"etcd-snapshot"},
			suffix: []string{"-f"},
			found:  true,
		},
		{
			name:   "command and subcommand with no flags",
			args:   []string{"etcd-snapshot", "list"},
			prefix: []string{"etcd-snapshot", "list"},
			suffix: []string{},
			found:  true,
		},
		{
			name:   "command and subcommand with flags",
			args:   []string{"etcd-snapshot", "list", "-f"},
			prefix: []string{"etcd-snapshot", "list"},
			suffix: []string{"-f"},
			found:  true,
		},
		{
			name:   "another command and subcommand with flags",
			args:   []string{"etcd-snapshot", "save", "--s3"},
			prefix: []string{"etcd-snapshot", "save"},
			suffix: []string{"--s3"},
			found:  true,
		},
		{
			name:   "command and too many subcommands",
			args:   []string{"etcd-snapshot", "list", "delete", "foo", "bar"},
			prefix: []string{"etcd-snapshot", "list"},
			suffix: []string{"delete", "foo", "bar"},
			found:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Parser{
				After: []string{"server", "agent", "etcd-snapshot:1"},
			}
			prefix, suffix, found := p.findStart(tt.args)
			if !reflect.DeepEqual(prefix, tt.prefix) {
				t.Errorf("Parser.findStart() prefix = %+v\nWant = %+v", prefix, tt.prefix)
			}
			if !reflect.DeepEqual(suffix, tt.suffix) {
				t.Errorf("Parser.findStart() suffix = %+v\nWant = %+v", suffix, tt.suffix)
			}
			if found != tt.found {
				t.Errorf("Parser.findStart() found = %+v\nWant = %+v", found, tt.found)
			}
		})
	}
}

func Test_UnitParser_findConfigFileFlag(t *testing.T) {
	type fields struct {
		DefaultConfig string
		env           string
	}
	tests := []struct {
		name   string
		fields fields
		arg    []string
		want   string
	}{
		{
			name: "default case",
			arg:  nil,
		},
		{
			name: "simple case",
			arg:  []string{"asdf", "-c", "value"},
			want: "value",
		},
		{
			name: "invalid args string",
			arg:  []string{"-c"},
		},
		{
			name: "empty arg value",
			arg:  []string{"-c="},
		},
		{
			name: "empty arg value override default",
			fields: fields{
				DefaultConfig: "def",
			},
			arg: []string{"-c="},
		},
		{
			fields: fields{
				DefaultConfig: "def",
			},
			arg:  []string{"-c"},
			name: "invalid args always return no value",
		},
		{
			fields: fields{
				DefaultConfig: "def",
			},
			arg:  []string{"-c", "value"},
			want: "value",
			name: "value override default",
		},
		{
			fields: fields{
				DefaultConfig: "def",
			},
			want: "def",
			name: "default gets used when nothing is passed",
		},
		{
			name: "env override args",
			fields: fields{
				DefaultConfig: "def",
				env:           "env",
			},
			arg:  []string{"-c", "value"},
			want: "env",
		},
		{
			name: "garbage in start and end",
			fields: fields{
				DefaultConfig: "def",
			},
			arg:  []string{"before", "-c", "value", "after"},
			want: "value",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Parser{
				ConfigFlags:   []string{"--config", "-c"},
				EnvName:       "_TEST_FLAG_ENV",
				DefaultConfig: tt.fields.DefaultConfig,
			}
			// Setup
			defer os.Unsetenv(tt.fields.env)
			os.Setenv(p.EnvName, tt.fields.env)

			got := p.findConfigFileFlag(tt.arg)
			if got != tt.want {
				t.Errorf("Parser.findConfigFileFlag() got = %+v\nWant = %+v", got, tt.want)
			}
		})
	}
}

func Test_UnitParser_Parse(t *testing.T) {
	testDataOutput := []string{
		"--foo-bar=bar-foo",
		"--alice=bob",
		"--a-slice=1",
		"--a-slice=1.5",
		"--a-slice=2",
		"--a-slice=",
		"--a-slice=three",
		"--isempty=",
		"-c=b",
		"--isfalse=false",
		"--islast=true",
		"--b-string=one",
		"--b-string=two",
		"--c-slice=one",
		"--c-slice=two",
		"--c-slice=three",
		"--d-slice=three",
		"--d-slice=four",
		"--f-string=beta",
		"--e-slice=one",
		"--e-slice=two",
	}
	type fields struct {
		After         []string
		FlagNames     []string
		EnvName       string
		DefaultConfig string
	}
	tests := []struct {
		name    string
		fields  fields
		arg     []string
		want    []string
		wantErr bool
	}{
		{
			name: "default case",
			fields: fields{
				After:         []string{"server", "agent"},
				FlagNames:     []string{"-c", "--config"},
				EnvName:       "_TEST_ENV",
				DefaultConfig: "./testdata/data.yaml",
			},
		},
		{
			name: "read config file when not specified",
			fields: fields{
				After:         []string{"server", "agent"},
				FlagNames:     []string{"-c", "--config"},
				EnvName:       "_TEST_ENV",
				DefaultConfig: "./testdata/data.yaml",
			},
			arg:  []string{"server"},
			want: append([]string{"server"}, testDataOutput...),
		},
		{
			name: "ignore missing config when not set",
			fields: fields{
				After:         []string{"server", "agent"},
				FlagNames:     []string{"-c", "--config"},
				DefaultConfig: "missing",
			},
			arg:  []string{"server"},
			want: []string{"server"},
		},
		{
			name: "ignore missing config when set",
			fields: fields{
				After:         []string{"server", "agent"},
				FlagNames:     []string{"-c", "--config"},
				DefaultConfig: "missing",
			},
			arg:  []string{"server", "-c=missing"},
			want: []string{"server", "-c=missing"},
		},
		{
			name: "fail when config cannot be parsed",
			fields: fields{
				After:         []string{"server", "agent"},
				FlagNames:     []string{"-c", "--config"},
				DefaultConfig: "./testdata/invalid.yaml",
			},
			arg:     []string{"server"},
			wantErr: true,
		},
		{
			name: "fail when dropin cannot be parsed",
			fields: fields{
				After:         []string{"server", "agent"},
				FlagNames:     []string{"-c", "--config"},
				DefaultConfig: "./testdata/invalid-dropin.yaml",
			},
			arg:     []string{"server"},
			wantErr: true,
		},
		{
			name: "read config file",
			fields: fields{
				After:         []string{"server", "agent"},
				FlagNames:     []string{"-c", "--config"},
				DefaultConfig: "missing",
			},
			arg:  []string{"before", "server", "before", "-c", "./testdata/data.yaml", "after"},
			want: append(append([]string{"before", "server"}, testDataOutput...), "before", "-c", "./testdata/data.yaml", "after"),
		},
		{
			name: "read single config file",
			fields: fields{
				After:         []string{"server", "agent"},
				FlagNames:     []string{"-c", "--config"},
				DefaultConfig: "missing",
			},
			arg: []string{"before", "server", "before", "-c", "./testdata/data.yaml.d/02-data.yaml", "after"},
			want: []string{"before", "server",
				"--foo-bar=bar-foo",
				"--b-string=two",
				"--c-slice=three",
				"--d-slice=three",
				"--d-slice=four",
				"--e-slice=one",
				"--e-slice=two",
				"before", "-c", "./testdata/data.yaml.d/02-data.yaml", "after"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Parser{
				After:         tt.fields.After,
				ConfigFlags:   tt.fields.FlagNames,
				EnvName:       tt.fields.EnvName,
				DefaultConfig: tt.fields.DefaultConfig,
			}

			got, err := p.Parse(tt.arg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parser.Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parser.Parse() = %+v\nWant = %+v", got, tt.want)
			}
		})
	}
}

func Test_UnitParser_FindString(t *testing.T) {
	type fields struct {
		After         []string
		FlagNames     []string
		EnvName       string
		DefaultConfig string
	}
	type args struct {
		osArgs []string
		target string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "Default config does not exist",
			fields: fields{
				After:         []string{"server", "agent"},
				FlagNames:     []string{"-c", "--config"},
				EnvName:       "_TEST_ENV",
				DefaultConfig: "missing",
			},
			args: args{
				osArgs: []string{},
				target: "",
			},
			want: "",
		},
		{
			name: "A custom config exists, target exists",
			fields: fields{
				FlagNames:     []string{"-c", "--config"},
				EnvName:       "_TEST_ENV",
				DefaultConfig: "./testdata/data.yaml",
			},
			args: args{
				osArgs: []string{"-c", "./testdata/data.yaml"},
				target: "alice",
			},
			want: "bob",
		},
		{
			name: "A custom config exists, target does not exist",
			fields: fields{
				FlagNames:     []string{"-c", "--config"},
				EnvName:       "_TEST_ENV",
				DefaultConfig: "./testdata/data.yaml",
			},
			args: args{
				osArgs: []string{"-c", "./testdata/data.yaml"},
				target: "tls",
			},
			want: "",
		},
		{
			name: "Default config file does not exist but dropin exists, target does not exist",
			fields: fields{
				FlagNames:     []string{"-c", "--config"},
				EnvName:       "_TEST_ENV",
				DefaultConfig: "./testdata/dropin-only.yaml",
			},
			args: args{
				osArgs: []string{},
				target: "tls",
			},
			want: "",
		},
		{
			name: "Default config file does not exist but dropin exists, target exists",
			fields: fields{
				FlagNames:     []string{"-c", "--config"},
				EnvName:       "_TEST_ENV",
				DefaultConfig: "./testdata/dropin-only.yaml",
			},
			args: args{
				osArgs: []string{},
				target: "foo-bar",
			},
			want: "bar-foo",
		},
		{
			name: "Custom config file does not exist but dropin exists, target does not exist",
			fields: fields{
				FlagNames:     []string{"-c", "--config"},
				EnvName:       "_TEST_ENV",
				DefaultConfig: "./testdata/defaultdata.yaml",
			},
			args: args{
				osArgs: []string{"-c", "./testdata/dropin-only.yaml"},
				target: "tls",
			},
			want: "",
		},
		{
			name: "Custom config file does not exist but dropin exists, target exists",
			fields: fields{
				FlagNames:     []string{"-c", "--config"},
				EnvName:       "_TEST_ENV",
				DefaultConfig: "./testdata/defaultdata.yaml",
			},
			args: args{
				osArgs: []string{"-c", "./testdata/dropin-only.yaml"},
				target: "foo-bar",
			},
			want: "bar-foo",
		},
		{
			name: "Multiple custom configs exist, target exists in a dropin config",
			fields: fields{
				FlagNames:     []string{"-c", "--config"},
				EnvName:       "_TEST_ENV",
				DefaultConfig: "./testdata/data.yaml",
			},
			args: args{
				osArgs: []string{"-c", "./testdata/data.yaml"},
				target: "f-string",
			},
			want: "beta",
		},
		{
			name: "Multiple custom configs exist, multiple targets exist in multiple dropin config, replacement",
			fields: fields{
				FlagNames:     []string{"-c", "--config"},
				EnvName:       "_TEST_ENV",
				DefaultConfig: "./testdata/data.yaml",
			},
			args: args{
				osArgs: []string{"-c", "./testdata/data.yaml"},
				target: "foo-bar",
			},
			want: "bar-foo",
		},
		{
			name: "Multiple custom configs exist, multiple targets exist in multiple dropin config, appending",
			fields: fields{
				FlagNames:     []string{"-c", "--config"},
				EnvName:       "_TEST_ENV",
				DefaultConfig: "./testdata/data.yaml",
			},
			args: args{
				osArgs: []string{"-c", "./testdata/data.yaml"},
				target: "b-string",
			},
			want: "one,two",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Parser{
				After:         tt.fields.After,
				ConfigFlags:   tt.fields.FlagNames,
				EnvName:       tt.fields.EnvName,
				DefaultConfig: tt.fields.DefaultConfig,
			}
			got, err := p.FindString(tt.args.osArgs, tt.args.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parser.FindString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Parser.FindString() = %+v\nWant = %+v", got, tt.want)
			}
		})
	}
}
