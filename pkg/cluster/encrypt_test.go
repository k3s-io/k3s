package cluster

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

func Test_storageKey(t *testing.T) {
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
				t.Errorf("storageKey() should be deterministic for same passphrase")
			}
			if got == storageKey(tt.other) {
				t.Errorf("storageKey() should differ for different passphrases")
			}
		})
	}
}

func Test_encrypt(t *testing.T) {
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
			ciphertext, err := encrypt(tt.passphrase, tt.plaintext)
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

			second, err := encrypt(tt.passphrase, tt.plaintext)
			if err != nil {
				t.Fatalf("second encrypt() error = %v", err)
			}
			if bytes.Equal(ciphertext, second) {
				t.Fatalf("encrypt() should produce unique ciphertext due to random salt/nonce")
			}
		})
	}
}

func Test_decrypt(t *testing.T) {
	goodCiphertext, err := encrypt("correct-passphrase", []byte("payload"))
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
