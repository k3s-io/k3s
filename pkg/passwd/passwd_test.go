package passwd

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func Test_UnitRead(t *testing.T) {
	tests := []struct {
		name        string
		content     *string
		wantNames   map[string]entry
		wantErr     bool
		wantErrText string
	}{
		{
			name:      "missing file returns empty passwd",
			content:   nil,
			wantNames: map[string]entry{},
			wantErr:   false,
		},
		{
			name:    "reads two and four column records",
			content: strPtr("pass1,user1\npass2,user2,user2,admin\n"),
			wantNames: map[string]entry{
				"user1": {pass: "pass1", role: ""},
				"user2": {pass: "pass2", role: "admin"},
			},
			wantErr: false,
		},
		{
			name:        "less than two columns returns error",
			content:     strPtr("onlypass\n"),
			wantErr:     true,
			wantErrText: "must have at least 2 columns",
		},
		{
			name:        "malformed csv returns parse error",
			content:     strPtr("\"unterminated\n"),
			wantErr:     true,
			wantErrText: "quoted-field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			passwdFile := filepath.Join(t.TempDir(), "passwd")
			if tt.content != nil {
				if err := os.WriteFile(passwdFile, []byte(*tt.content), 0644); err != nil {
					t.Fatalf("failed to write fixture file: %v", err)
				}
			}

			got, err := Read(passwdFile)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Read() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if tt.wantErrText != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErrText)) {
					t.Fatalf("Read() error = %v, expected to contain %q", err, tt.wantErrText)
				}
				return
			}

			if got == nil {
				t.Fatalf("Read() returned nil Passwd")
			}
			if !reflect.DeepEqual(got.names, tt.wantNames) {
				t.Errorf("Read() names = %+v, want %+v", got.names, tt.wantNames)
			}
		})
	}
}

func Test_UnitPasswd_EnsureUser(t *testing.T) {
	tests := []struct {
		name        string
		passwd      *Passwd
		user        string
		role        string
		pass        string
		wantErr     bool
		wantChanged bool
		wantRole    string
		wantPass    string
		wantRandom  bool
	}{
		{
			name: "existing user unchanged",
			passwd: &Passwd{names: map[string]entry{
				"alice": {pass: "oldpass", role: "admin"},
			}},
			user:        "alice",
			role:        "admin",
			pass:        "",
			wantErr:     false,
			wantChanged: false,
			wantRole:    "admin",
			wantPass:    "oldpass",
		},
		{
			name: "existing user password updated",
			passwd: &Passwd{names: map[string]entry{
				"alice": {pass: "oldpass", role: "admin"},
			}},
			user:        "alice",
			role:        "admin",
			pass:        "newpass",
			wantErr:     false,
			wantChanged: true,
			wantRole:    "admin",
			wantPass:    "newpass",
		},
		{
			name: "existing user role updated",
			passwd: &Passwd{names: map[string]entry{
				"alice": {pass: "oldpass", role: "admin"},
			}},
			user:        "alice",
			role:        "server",
			pass:        "",
			wantErr:     false,
			wantChanged: true,
			wantRole:    "server",
			wantPass:    "oldpass",
		},
		{
			name:        "new user with explicit password",
			passwd:      &Passwd{names: map[string]entry{}},
			user:        "bob",
			role:        "server",
			pass:        "bobpass",
			wantErr:     false,
			wantChanged: true,
			wantRole:    "server",
			wantPass:    "bobpass",
		},
		{
			name:        "new user strips K10 token prefix",
			passwd:      &Passwd{names: map[string]entry{}},
			user:        "server",
			role:        "server",
			pass:        "K10abcdef::server:tokenpass",
			wantErr:     false,
			wantChanged: true,
			wantRole:    "server",
			wantPass:    "tokenpass",
		},
		{
			name:        "new user with empty password gets random token",
			passwd:      &Passwd{names: map[string]entry{}},
			user:        "carol",
			role:        "agent",
			pass:        "",
			wantErr:     false,
			wantChanged: true,
			wantRole:    "agent",
			wantRandom:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.passwd.EnsureUser(tt.user, tt.role, tt.pass)
			if (err != nil) != tt.wantErr {
				t.Fatalf("EnsureUser() error = %v, wantErr %v", err, tt.wantErr)
			}

			e, ok := tt.passwd.names[tt.user]
			if !ok {
				t.Fatalf("EnsureUser() did not create/update entry for %q", tt.user)
			}
			if e.role != tt.wantRole {
				t.Errorf("EnsureUser() role = %q, want %q", e.role, tt.wantRole)
			}
			if tt.wantRandom {
				if len(e.pass) != 32 {
					t.Fatalf("EnsureUser() random password length = %d, want 32", len(e.pass))
				}
				if _, err := hex.DecodeString(e.pass); err != nil {
					t.Fatalf("EnsureUser() random password is not hex: %v", err)
				}
			} else if e.pass != tt.wantPass {
				t.Errorf("EnsureUser() password = %q, want %q", e.pass, tt.wantPass)
			}
			if tt.passwd.changed != tt.wantChanged {
				t.Errorf("EnsureUser() changed = %v, want %v", tt.passwd.changed, tt.wantChanged)
			}
		})
	}
}

func Test_UnitPasswd_Users(t *testing.T) {
	tests := []struct {
		name   string
		passwd *Passwd
		want   []string
	}{
		{
			name: "returns sorted user list from map",
			passwd: &Passwd{names: map[string]entry{
				"charlie": {pass: "c"},
				"alice":   {pass: "a"},
				"bob":     {pass: "b"},
			}},
			want: []string{"alice", "bob", "charlie"},
		},
		{
			name:   "empty map returns empty list",
			passwd: &Passwd{names: map[string]entry{}},
			want:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.passwd.Users()
			slices.Sort(got)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Users() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_UnitPasswd_Pass(t *testing.T) {
	tests := []struct {
		name      string
		passwd    *Passwd
		user      string
		wantPass  string
		wantFound bool
	}{
		{
			name: "existing user",
			passwd: &Passwd{names: map[string]entry{
				"alice": {pass: "alice-pass"},
			}},
			user:      "alice",
			wantPass:  "alice-pass",
			wantFound: true,
		},
		{
			name:      "missing user",
			passwd:    &Passwd{names: map[string]entry{}},
			user:      "nobody",
			wantPass:  "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPass, gotFound := tt.passwd.Pass(tt.user)
			if gotFound != tt.wantFound {
				t.Errorf("Pass() found = %v, want %v", gotFound, tt.wantFound)
			}
			if gotPass != tt.wantPass {
				t.Errorf("Pass() = %q, want %q", gotPass, tt.wantPass)
			}
		})
	}
}

func Test_UnitPasswd_Write(t *testing.T) {
	tests := []struct {
		name      string
		passwd    *Passwd
		pathParts []string
		wantErr   bool
		wantFile  bool
		wantNames map[string]entry
	}{
		{
			name: "unchanged passwd does not write file",
			passwd: &Passwd{
				changed: false,
				names: map[string]entry{
					"alice": {pass: "alice-pass", role: "admin"},
				},
			},
			pathParts: []string{"passwd"},
			wantErr:   false,
			wantFile:  false,
		},
		{
			name: "changed passwd writes file",
			passwd: &Passwd{
				changed: true,
				names: map[string]entry{
					"alice": {pass: "alice-pass", role: "admin"},
					"bob":   {pass: "bob-pass", role: "server"},
				},
			},
			pathParts: []string{"passwd"},
			wantErr:   false,
			wantFile:  true,
			wantNames: map[string]entry{
				"alice": {pass: "alice-pass", role: "admin"},
				"bob":   {pass: "bob-pass", role: "server"},
			},
		},
		{
			name: "write to missing parent directory returns error",
			passwd: &Passwd{
				changed: true,
				names: map[string]entry{
					"alice": {pass: "alice-pass", role: "admin"},
				},
			},
			pathParts: []string{"missing", "passwd"},
			wantErr:   true,
			wantFile:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			passwdFile := filepath.Join(append([]string{t.TempDir()}, tt.pathParts...)...)
			err := tt.passwd.Write(passwdFile)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Write() error = %v, wantErr %v", err, tt.wantErr)
			}

			_, statErr := os.Stat(passwdFile)
			hasFile := statErr == nil
			if hasFile != tt.wantFile {
				t.Fatalf("Write() wrote file = %v, want %v", hasFile, tt.wantFile)
			}

			if tt.wantFile {
				got, err := Read(passwdFile)
				if err != nil {
					t.Fatalf("Read() after Write() failed: %v", err)
				}
				if !reflect.DeepEqual(got.names, tt.wantNames) {
					t.Errorf("Write()/Read() names = %+v, want %+v", got.names, tt.wantNames)
				}
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
