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
		name     string
		args     args
		setup    func() error // Optional, delete if unused
		teardown func() error // Optional, delete if unused
		wantErr  bool
	}{
		{
			name: "Basic Hash Test",
			args: args{
				secretKey: "hello world",
			},
			setup:    func() error { return nil },
			teardown: func() error { return nil },
		},
		{
			name: "Long Hash Test",
			args: args{
				secretKey: strings.Repeat("A", 720),
			},
			setup:    func() error { return nil },
			teardown: func() error { return nil },
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
