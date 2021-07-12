package hash

import (
	"strings"
	"testing"
)

func TestSCrypt_VerifyHash(t *testing.T) {
	type args struct {
		secretKey string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Basic Hash Test",
			args: args{
				secretKey: "hello world",
			},
		},
		{
			name: "Long Hash Test",
			args: args{
				secretKey: strings.Repeat("A", 720),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasher := NewSCrypt()
			hash, _ := hasher.CreateHash(tt.args.secretKey)
			if err := hasher.VerifyHash(hash, tt.args.secretKey); (err != nil) != tt.wantErr {
				t.Errorf("SCrypt.VerifyHash() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
