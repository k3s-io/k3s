package cluster

import (
	"bytes"
	"crypto/fips140"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

func Test_UnitstorageKey(t *testing.T) {
	tests := []struct {
		name       string
		passphrase string
		other      string
	}{
		{
			name:       "prefix and deterministic hash",
			passphrase: "test-passphrase",
			other:      "different-passphrase",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := storageKey(tt.passphrase)
			if !strings.HasPrefix(got, "/bootstrap/") {
				t.Errorf("storageKey() = %q, want prefix /bootstrap/", got)
			}
			if got != storageKey(tt.passphrase) {
				t.Error("storageKey() should be deterministic for same passphrase")
			}
			if got == storageKey(tt.other) {
				t.Error("storageKey() should differ for different passphrases")
			}
		})
	}
}

func Test_Unitencrypt(t *testing.T) {
	setFIPS(t, false, false)
	tests := []struct {
		name       string
		passphrase string
		plaintext  []byte
	}{
		{
			name:       "round-trip and encoded output",
			passphrase: "strong-passphrase",
			plaintext:  []byte("bootstrap payload for unit tests"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, err := encrypt(tt.passphrase, tt.plaintext, false)
			if err != nil {
				t.Fatalf("encrypt() error = %v", err)
			}

			parts := strings.SplitN(string(ciphertext), ":", 2)
			if len(parts) != 2 {
				t.Fatalf("encrypt() output should be salt:ciphertext, got %q", string(ciphertext))
			}
			if len(parts[0]) != 16 {
				t.Fatalf("encrypt() salt length = %d, want 16 hex chars", len(parts[0]))
			}
			if _, err := base64.StdEncoding.DecodeString(parts[1]); err != nil {
				t.Fatalf("encrypt() ciphertext should be base64 encoded: %v", err)
			}

			decrypted, err := decrypt(tt.passphrase, ciphertext)
			if err != nil {
				t.Fatalf("decrypt() error = %v", err)
			}
			if !bytes.Equal(decrypted, tt.plaintext) {
				t.Fatalf("decrypt() = %q, want %q", string(decrypted), string(tt.plaintext))
			}

			second, err := encrypt(tt.passphrase, tt.plaintext, false)
			if err != nil {
				t.Fatalf("second encrypt() error = %v", err)
			}
			if bytes.Equal(ciphertext, second) {
				t.Fatal("encrypt() should produce unique ciphertext due to random salt/nonce")
			}
		})
	}
}

func Test_Unitdecrypt(t *testing.T) {
	setFIPS(t, false, false)
	goodCiphertext, err := encrypt("correct-passphrase", []byte("payload"), false)
	if err != nil {
		t.Fatalf("setup encrypt() error = %v", err)
	}

	tests := []struct {
		name        string
		passphrase  string
		ciphertext  []byte
		wantErr     bool
		wantErrText string
	}{
		{
			name:        "not colon delimited",
			passphrase:  "secret",
			ciphertext:  []byte("invalid-format"),
			wantErr:     true,
			wantErrText: "not : delimited",
		},
		{
			name:       "invalid base64",
			passphrase: "secret",
			ciphertext: []byte("salt:***"),
			wantErr:    true,
		},
		{name: "empty", passphrase: "secret", ciphertext: []byte(""), wantErr: true, wantErrText: "not : delimited"},
		{name: "legacy shorter than a nonce", passphrase: "secret", ciphertext: []byte("salt:AAAA"), wantErr: true, wantErrText: "too short"},
		{name: "v2 shorter than a nonce", passphrase: "secret", ciphertext: []byte(v2Prefix + "salt:AAAA"), wantErr: true, wantErrText: "too short"},
		{
			name:       "wrong passphrase",
			passphrase: "wrong-passphrase",
			ciphertext: goodCiphertext,
			wantErr:    true,
		},
		{
			name:       "success",
			passphrase: "correct-passphrase",
			ciphertext: goodCiphertext,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decrypt(tt.passphrase, tt.ciphertext)
			if (err != nil) != tt.wantErr {
				t.Fatalf("decrypt() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if tt.wantErrText != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErrText)) {
					t.Fatalf("decrypt() error = %v, expected to contain %q", err, tt.wantErrText)
				}
				return
			}
			if string(got) != "payload" {
				t.Errorf("decrypt() = %q, want %q", string(got), "payload")
			}
		})
	}
}

// legacyVector is "bootstrap payload" encrypted with "test-passphrase" by the SHA-1 encrypt that predates the v2 format.
const legacyVector = "663ef4545bce9fe4:FXvWg+7GFBzv4XCKLGvZeGAS2bGC0MwIKs8ZuEGJTtDlB1WKLDFRjidxCoNQ"

var boringEnabled bool

func setFIPS(t *testing.T, enabled, enforced bool) {
	if fips140.Enforced() && !enforced || enabled && boringEnabled {
		t.Skip("mode is not available in this build")
	}
	oldEnabled, oldEnforced := fipsEnabled, fipsEnforced
	fipsEnabled, fipsEnforced = func() bool { return enabled }, func() bool { return enforced }
	t.Cleanup(func() { fipsEnabled, fipsEnforced = oldEnabled, oldEnforced })
}

func Test_UnituseV2(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		existing string
		want     bool
	}{
		{"fips off writes legacy", false, legacyVector, false},
		{"fips off keeps v2", false, v2Prefix + "salt:data", true},
		{"fips on writes v2", true, legacyVector, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setFIPS(t, tt.enabled, false)
			if got := useV2([]byte(tt.existing)); got != tt.want {
				t.Fatalf("useV2() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_UnitencryptV2(t *testing.T) {
	for _, write := range []bool{true, false} {
		for _, read := range []bool{true, false} {
			t.Run(fmt.Sprintf("write fips=%v read fips=%v", write, read), func(t *testing.T) {
				setFIPS(t, write, false)
				ciphertext, err := encrypt("test-passphrase", []byte("payload"), true)
				if err != nil || !bytes.HasPrefix(ciphertext, []byte(v2Prefix)) {
					t.Fatalf("encrypt() = %q, %v, want a %q value", ciphertext, err, v2Prefix)
				}
				setFIPS(t, read, false)
				if got, err := decrypt("test-passphrase", ciphertext); err != nil || string(got) != "payload" {
					t.Fatalf("decrypt() = %q, %v", got, err)
				}
				if _, err := decrypt("wrong-passphrase", ciphertext); err == nil {
					t.Fatal("decrypt() accepted the wrong passphrase")
				}
			})
		}
	}
}

func Test_UnitdecryptLegacy(t *testing.T) {
	tests := []struct {
		name              string
		enabled, enforced bool
		wantErrText       string
	}{
		{"fips off", false, false, ""},
		{"fips on", true, false, ""},
		{"fips only", true, true, "fips140=on"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setFIPS(t, tt.enabled, tt.enforced)
			got, err := decrypt("test-passphrase", []byte(legacyVector))
			if (err != nil) != (tt.wantErrText != "") || err != nil && !strings.Contains(err.Error(), tt.wantErrText) {
				t.Fatalf("decrypt() error = %v, expected to contain %q", err, tt.wantErrText)
			}
			if err == nil && string(got) != "bootstrap payload" {
				t.Fatalf("decrypt() = %q", got)
			}
		})
	}
}
