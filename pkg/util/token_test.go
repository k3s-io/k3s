package util

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/pkg/daemons/config"
)

func Test_UnitRandom(t *testing.T) {
	tests := []struct {
		name    string
		size    int
		wantLen int
		wantErr bool
	}{
		{
			name:    "zero length",
			size:    0,
			wantLen: 0,
			wantErr: false,
		},
		{
			name:    "non-zero length",
			size:    8,
			wantLen: 16,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Random(tt.size)
			if (err != nil) != tt.wantErr {
				t.Errorf("Random() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != tt.wantLen {
				t.Errorf("Random() length = %d, want %d", len(got), tt.wantLen)
			}
			if _, err := hex.DecodeString(got); err != nil {
				t.Errorf("Random() output is not valid hex: %v", err)
			}
		})
	}
}

func Test_UnitReadTokenFromFile(t *testing.T) {
	tests := []struct {
		name        string
		serverToken string
		certs       string
		tokenFile   *string
		want        string
		wantErr     bool
	}{
		{
			name:        "token file is read and trimmed",
			serverToken: "ignored-token",
			certs:       "/nonexistent-ca",
			tokenFile:   strPtr("  file-token  \n"),
			want:        "file-token",
			wantErr:     false,
		},
		{
			name:        "missing token file falls back to empty server token",
			serverToken: "",
			certs:       "/nonexistent-ca",
			tokenFile:   nil,
			want:        "",
			wantErr:     false,
		},
		{
			name:        "empty token file falls back to format token and returns cert error",
			serverToken: "server-token",
			certs:       "/nonexistent-ca",
			tokenFile:   strPtr("\n"),
			wantErr:     true,
		},
		{
			name:        "missing token file falls back to format token and returns cert error",
			serverToken: "server-token",
			certs:       "/nonexistent-ca",
			tokenFile:   nil,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := t.TempDir()
			if tt.tokenFile != nil {
				tokenPath := filepath.Join(dataDir, "token")
				if err := os.WriteFile(tokenPath, []byte(*tt.tokenFile), 0644); err != nil {
					t.Fatalf("failed writing token file: %v", err)
				}
			}

			got, err := ReadTokenFromFile(tt.serverToken, tt.certs, dataDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadTokenFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ReadTokenFromFile() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_UnitNormalizeToken(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		want    string
		wantErr bool
	}{
		{
			name:    "bare password token",
			token:   "secret-password",
			want:    "secret-password",
			wantErr: false,
		},
		{
			name:    "full k10 username password token",
			token:   "K10" + strings.Repeat("a", 64) + "::server:secret-password",
			want:    "secret-password",
			wantErr: false,
		},
		{
			name:    "bootstrap token is not valid for normalize",
			token:   "abcdef.0123456789abcdef",
			wantErr: true,
		},
		{
			name:    "invalid token format",
			token:   "K10::x",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeToken(tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizeToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("NormalizeToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_UnitGetTokenHash(t *testing.T) {
	tests := []struct {
		name           string
		token          string
		runtimeToken   string
		runtimeCA      string
		tokenFile      *string
		wantTokenValue string
		wantErr        bool
	}{
		{
			name:           "uses config token directly",
			token:          "direct-password",
			wantTokenValue: "direct-password",
			wantErr:        false,
		},
		{
			name:           "uses token from data dir when config token is empty",
			token:          "",
			tokenFile:      strPtr("from-file-token\n"),
			wantTokenValue: "from-file-token",
			wantErr:        false,
		},
		{
			name:         "empty token file falls back to runtime token and returns cert error",
			token:        "",
			runtimeToken: "runtime-token",
			runtimeCA:    "/nonexistent-ca",
			tokenFile:    strPtr("\n"),
			wantErr:      true,
		},
		{
			name:    "invalid config token returns normalize error",
			token:   "K10::x",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := t.TempDir()
			if tt.tokenFile != nil {
				tokenPath := filepath.Join(dataDir, "token")
				if err := os.WriteFile(tokenPath, []byte(*tt.tokenFile), 0644); err != nil {
					t.Fatalf("failed writing token file: %v", err)
				}
			}

			cfg := &config.Control{
				Token:   tt.token,
				DataDir: dataDir,
				Runtime: &config.ControlRuntime{
					ControlRuntimeBootstrap: config.ControlRuntimeBootstrap{
						ServerCA: tt.runtimeCA,
					},
					ServerToken: tt.runtimeToken,
				},
			}

			got, err := GetTokenHash(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTokenHash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				want := ShortHash(tt.wantTokenValue, 12)
				if got != want {
					t.Errorf("GetTokenHash() = %q, want %q", got, want)
				}
			}
		})
	}
}

func Test_UnitShortHash(t *testing.T) {
	tests := []struct {
		name  string
		value string
		len   int
		want  string
	}{
		{
			name:  "returns expected 12 character hash",
			value: "abc",
			len:   12,
			want:  "ba7816bf8f01",
		},
		{
			name:  "zero length returns empty string",
			value: "abc",
			len:   0,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShortHash(tt.value, tt.len); got != tt.want {
				t.Errorf("ShortHash() = %q, want %q", got, tt.want)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
