package deps

import (
	"net"
	"reflect"
	"testing"

	certutil "github.com/rancher/dynamiclistener/cert"
)

func Test_UnitAddSANs(t *testing.T) {
	type args struct {
		altNames *certutil.AltNames
		sans     []string
	}
	tests := []struct {
		name string
		args args
		want certutil.AltNames
	}{
		{
			name: "One IP, One DNS",
			args: args{
				altNames: &certutil.AltNames{},
				sans:     []string{"192.168.205.10", "192.168.205.10.nip.io"},
			},
			want: certutil.AltNames{
				IPs:      []net.IP{net.ParseIP("192.168.205.10")},
				DNSNames: []string{"192.168.205.10.nip.io"},
			},
		},
		{
			name: "Two IP, No DNS",
			args: args{
				altNames: &certutil.AltNames{},
				sans:     []string{"192.168.205.10", "10.168.21.15"},
			},
			want: certutil.AltNames{
				IPs: []net.IP{net.ParseIP("192.168.205.10"), net.ParseIP("10.168.21.15")},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addSANs(tt.args.altNames, tt.args.sans)
			if !reflect.DeepEqual(*tt.args.altNames, tt.want) {
				t.Errorf("addSANs() = %v, want %v", *tt.args.altNames, tt.want)
			}
		})
	}
}
