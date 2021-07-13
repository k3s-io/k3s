package configfilearg

import (
	"os"
	"reflect"
	"testing"
)

func TestParser_findStart(t *testing.T) {
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Parser{
				After: []string{"server", "agent"},
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

func TestParser_findConfigFileFlag(t *testing.T) {
	type fields struct {
		DefaultConfig string
		env           string
	}
	tests := []struct {
		name   string
		fields fields
		arg    []string
		want   string
		found  bool
	}{
		{
			name:  "default case",
			arg:   nil,
			found: false,
		},
		{
			name:  "simple case",
			arg:   []string{"asdf", "-c", "value"},
			want:  "value",
			found: true,
		},
		{
			name:  "invalid args string",
			arg:   []string{"-c"},
			found: false,
		},
		{
			name:  "empty arg value",
			arg:   []string{"-c="},
			found: true,
		},
		{
			name: "empty arg value override default",
			fields: fields{
				DefaultConfig: "def",
			},
			arg:   []string{"-c="},
			found: true,
		},
		{
			fields: fields{
				DefaultConfig: "def",
			},
			arg:   []string{"-c"},
			found: false,
			name:  "invalid args always return no value",
		},
		{
			fields: fields{
				DefaultConfig: "def",
			},
			arg:   []string{"-c", "value"},
			want:  "value",
			found: true,
			name:  "value override default",
		},
		{
			fields: fields{
				DefaultConfig: "def",
			},
			want:  "def",
			found: false,
			name:  "default gets used when nothing is passed",
		},
		{
			name: "env override args",
			fields: fields{
				DefaultConfig: "def",
				env:           "env",
			},
			arg:   []string{"-c", "value"},
			want:  "env",
			found: true,
		},
		{
			name: "garbage in start and end",
			fields: fields{
				DefaultConfig: "def",
			},
			arg:   []string{"before", "-c", "value", "after"},
			want:  "value",
			found: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Parser{
				FlagNames:     []string{"--config", "-c"},
				EnvName:       "_TEST_FLAG_ENV",
				DefaultConfig: tt.fields.DefaultConfig,
			}
			// Setup
			defer os.Unsetenv(tt.fields.env)
			os.Setenv(p.EnvName, tt.fields.env)

			got, found := p.findConfigFileFlag(tt.arg)
			if got != tt.want {
				t.Errorf("Parser.findConfigFileFlag() got = %+v\nWant = %+v", got, tt.want)
			}
			if found != tt.found {
				t.Errorf("Parser.findConfigFileFlag() found = %+v\nWant = %+v", found, tt.found)
			}
		})
	}
}

func TestParser_Parse(t *testing.T) {
	testDataOutput := []string{
		"--foo-bar=bar-foo",
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
			name: "fail when missing config",
			fields: fields{
				After:         []string{"server", "agent"},
				FlagNames:     []string{"-c", "--config"},
				DefaultConfig: "missing",
			},
			arg:     []string{"server", "-c=missing"},
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
				FlagNames:     tt.fields.FlagNames,
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

func TestParser_FindString(t *testing.T) {
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
			name: "A custom config yaml exists, target exists",
			fields: fields{
				FlagNames:     []string{"-c", "--config"},
				EnvName:       "_TEST_ENV",
				DefaultConfig: "./testdata/data.yaml",
			},
			args: args{
				osArgs: []string{"-c", "./testdata/data.yaml"},
				target: "foo-bar",
			},
			want: "baz",
		},
		{
			name: "A custom config yaml exists, target does not exist",
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Parser{
				After:         tt.fields.After,
				FlagNames:     tt.fields.FlagNames,
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
