package hash

import (
	"strings"
	"testing"
)

func Test_UnitSCrypt_VerifyHash(t *testing.T) {
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

func FuzzVerifyHash(f *testing.F) {
	hasher := NewSCrypt()
	validSecret := "my-secret-password"
	validHash, _ := hasher.CreateHash(validSecret)

	// Seed the fuzzer with some valid and invalid inputs
	f.Add(validHash, validSecret)
	f.Add(validHash, "wrong-password")
	f.Add("", "")
	f.Add("$1:deadbeef:f:8:1:corrupt-hash", "any-password")
	f.Add("", validSecret)
	f.Add(validHash, "")

	f.Fuzz(func(t *testing.T, hash, secretKey string) {
		_ = hasher.VerifyHash(hash, secretKey)
	})
}
