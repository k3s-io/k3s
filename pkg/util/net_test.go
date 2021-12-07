package util

import (
	"net"
	"reflect"
	"testing"

	"github.com/urfave/cli"
)

func Test_UnitParseStringSliceToIPs(t *testing.T) {
	tests := []struct {
		name    string
		arg     cli.StringSlice
		want    []net.IP
		wantErr bool
	}{
		{
			name: "nil string slice must return no errors",
			arg:  nil,
			want: nil,
		},
		{
			name: "empty string slice must return no errors",
			arg:  cli.StringSlice{},
			want: nil,
		},
		{
			name: "single element slice with correct IP must succeed",
			arg:  cli.StringSlice{"10.10.10.10"},
			want: []net.IP{net.ParseIP("10.10.10.10")},
		},
		{
			name: "single element slice with correct IP list must succeed",
			arg:  cli.StringSlice{"10.10.10.10,10.10.10.11"},
			want: []net.IP{
				net.ParseIP("10.10.10.10"),
				net.ParseIP("10.10.10.11"),
			},
		},
		{
			name: "multi element slice with correct IP list must succeed",
			arg:  cli.StringSlice{"10.10.10.10,10.10.10.11", "10.10.10.12,10.10.10.13"},
			want: []net.IP{
				net.ParseIP("10.10.10.10"),
				net.ParseIP("10.10.10.11"),
				net.ParseIP("10.10.10.12"),
				net.ParseIP("10.10.10.13"),
			},
		},
		{
			name:    "single element slice with correct IP list with trailing comma must fail",
			arg:     cli.StringSlice{"10.10.10.10,"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "single element slice with incorrect IP (overflow) must fail",
			arg:     cli.StringSlice{"10.10.10.256"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "single element slice with incorrect IP (foreign symbols) must fail",
			arg:     cli.StringSlice{"xxx.yyy.zzz.www"},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				got, err := ParseStringSliceToIPs(tt.arg)
				if (err != nil) != tt.wantErr {
					t.Errorf("ParseStringSliceToIPs() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("ParseStringSliceToIPs() = %v, want %v", got, tt.want)
				}
			},
		)
	}
}
