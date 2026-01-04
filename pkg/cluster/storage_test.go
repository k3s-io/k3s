package cluster

import (
	"testing"

	"github.com/k3s-io/kine/pkg/client"
)

func Test_UnitValidateBootstrapTokenRequirement(t *testing.T) {
	tests := []struct {
		name    string
		values  []client.Value
		wantErr bool
	}{
		{
			name:    "no bootstrap data",
			values:  nil,
			wantErr: false,
		},
		{
			name: "bootstrap lock exists",
			values: []client.Value{
				{
					Key:  []byte("/bootstrap/test"),
					Data: []byte{},
				},
			},
			wantErr: true,
		},
		{
			name: "bootstrap data exists",
			values: []client.Value{
				{
					Key:  []byte("/bootstrap/test"),
					Data: []byte("payload"),
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBootstrapTokenRequirement(tt.values)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateBootstrapTokenRequirement() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
