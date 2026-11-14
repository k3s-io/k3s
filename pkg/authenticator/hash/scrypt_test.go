package hash

import (
	"crypto/fips140"
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

func Test_UnitSCrypt_CreateHash_SaltLength(t *testing.T) {
	hash, err := NewSCrypt().CreateHash("hello world")
	if err != nil {
		t.Fatal(err)
	}
	if salt := strings.Split(hash, ":")[1]; len(salt) != 32 {
		t.Errorf("salt is %d hex characters, want 32", len(salt))
	}
}

// legacyHash is "hello world" hashed by the 8-byte-salt SCrypt that predates the 16-byte salt.
const legacyHash = "$1:c229c44d4c4adb07:15:8:1:8TozcojzhYRmvoYMOzClhBJ5EomfBg+6yClpqo15t0j2ENlD+cVwcVdIyqo02V6JK05zToAusxPYmytPIus/kQ"

func Test_UnitSCrypt_VerifyHash_LegacySalt(t *testing.T) {
	if fips140.Enforced() {
		t.Skip("fips140=only rejects the 8-byte salt, VerifyHash does not guard it yet")
	}
	hasher := NewSCrypt()
	if err := hasher.VerifyHash(legacyHash, "hello world"); err != nil {
		t.Fatalf("SCrypt.VerifyHash() error = %v, want nil for a stock 8-byte-salt hash", err)
	}
	if err := hasher.VerifyHash(legacyHash, "wrong"); err == nil {
		t.Fatal("SCrypt.VerifyHash() accepted the wrong secret")
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
