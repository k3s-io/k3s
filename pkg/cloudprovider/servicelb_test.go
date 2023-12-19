package cloudprovider

import (
	"math/rand"
	"reflect"
	"testing"

	core "k8s.io/api/core/v1"
)

const (
	addrv4   = "1.2.3.4"
	addrv4_2 = "2.3.4.5"
	addrv6   = "2001:db8::1"
	addrv6_2 = "3001:db8::1"
)

func Test_UnitFilterByIPFamily(t *testing.T) {
	type args struct {
		ips []string
		svc *core.Service
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "No IPFamily",
			args: args{
				ips: []string{addrv4, addrv6},
				svc: &core.Service{
					Spec: core.ServiceSpec{
						IPFamilies: []core.IPFamily{},
					},
				},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "IPv4 Only",
			args: args{
				ips: []string{addrv4, addrv6},
				svc: &core.Service{
					Spec: core.ServiceSpec{
						IPFamilies: []core.IPFamily{core.IPv4Protocol},
					},
				},
			},
			want:    []string{addrv4},
			wantErr: false,
		},
		{
			name: "IPv6 Only",
			args: args{
				ips: []string{addrv4, addrv6},
				svc: &core.Service{
					Spec: core.ServiceSpec{
						IPFamilies: []core.IPFamily{core.IPv6Protocol},
					},
				},
			},
			want:    []string{addrv6},
			wantErr: false,
		},
		{
			name: "Dual-Stack",
			args: args{
				ips: []string{addrv4, addrv6},
				svc: &core.Service{
					Spec: core.ServiceSpec{
						IPFamilies: []core.IPFamily{core.IPv4Protocol, core.IPv6Protocol},
					},
				},
			},
			want:    []string{addrv4, addrv6},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := filterByIPFamily(tt.args.ips, tt.args.svc)
			if (err != nil) != tt.wantErr {
				t.Errorf("filterByIPFamily() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("filterByIPFamily() = %+v\nWant = %+v", got, tt.want)
			}
		})
	}
}

func Test_UnitFilterByIPFamily_Ordering(t *testing.T) {
	want := []string{addrv4, addrv4_2, addrv6, addrv6_2}
	ips := []string{addrv4, addrv4_2, addrv6, addrv6_2}
	rand.Shuffle(len(ips), func(i, j int) {
		ips[i], ips[j] = ips[j], ips[i]
	})
	svc := &core.Service{
		Spec: core.ServiceSpec{
			IPFamilies: []core.IPFamily{core.IPv4Protocol, core.IPv6Protocol},
		},
	}

	got, _ := filterByIPFamily(ips, svc)

	if !reflect.DeepEqual(got, want) {
		t.Errorf("filterByIPFamily() = %+v\nWant = %+v", got, want)
	}
}
