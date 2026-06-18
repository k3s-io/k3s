package loadbalancer

import (
	"reflect"
	"strings"
	"testing"
)

func Test_UnitParseURL(t *testing.T) {
	tests := []struct {
		name         string
		serverURL    string
		newHost      string
		wantAddress  string
		wantURL      string
		wantErr      bool
		wantErrMatch string
	}{
		{
			name:        "https without port defaults to 443",
			serverURL:   "https://example.com/path",
			newHost:     "127.0.0.1:6443",
			wantAddress: "example.com:443",
			wantURL:     "https://127.0.0.1:6443/path",
		},
		{
			name:        "http without port defaults to 80",
			serverURL:   "http://example.com/v1",
			newHost:     "localhost:8080",
			wantAddress: "example.com:80",
			wantURL:     "http://localhost:8080/v1",
		},
		{
			name:        "explicit port is preserved in address",
			serverURL:   "https://example.com:9443/v1",
			newHost:     "127.0.0.1:6443",
			wantAddress: "example.com:9443",
			wantURL:     "https://127.0.0.1:6443/v1",
		},
		{
			name:         "missing host returns error",
			serverURL:    "https:///v1",
			newHost:      "127.0.0.1:6443",
			wantErr:      true,
			wantErrMatch: "host is not defined",
		},
		{
			name:         "invalid URL returns parse error",
			serverURL:    "://bad-url",
			newHost:      "127.0.0.1:6443",
			wantErr:      true,
			wantErrMatch: "missing protocol scheme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAddress, gotURL, err := parseURL(tt.serverURL, tt.newHost)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseURL() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if tt.wantErrMatch != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErrMatch)) {
					t.Fatalf("parseURL() error = %v, expected to contain %q", err, tt.wantErrMatch)
				}
				return
			}

			if gotAddress != tt.wantAddress {
				t.Fatalf("parseURL() address = %q, want %q", gotAddress, tt.wantAddress)
			}
			if gotURL != tt.wantURL {
				t.Fatalf("parseURL() URL = %q, want %q", gotURL, tt.wantURL)
			}
		})
	}
}

func Test_UnitSortServers(t *testing.T) {
	tests := []struct {
		name      string
		input     []string
		search    string
		want      []string
		wantFound bool
	}{
		{
			name:      "sorts and removes duplicates and empty entries",
			input:     []string{"", "b", "a", "b", "", "c"},
			search:    "b",
			want:      []string{"a", "b", "c"},
			wantFound: true,
		},
		{
			name:      "search not found",
			input:     []string{"z", "y", "x"},
			search:    "a",
			want:      []string{"x", "y", "z"},
			wantFound: false,
		},
		{
			name:      "search empty string is never found because empty values are skipped",
			input:     []string{"", "", "a"},
			search:    "",
			want:      []string{"a"},
			wantFound: false,
		},
		{
			name:      "all empty input yields empty output",
			input:     []string{"", ""},
			search:    "anything",
			want:      []string{},
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotFound := sortServers(tt.input, tt.search)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("sortServers() result = %v, want %v", got, tt.want)
			}
			if gotFound != tt.wantFound {
				t.Fatalf("sortServers() found = %v, want %v", gotFound, tt.wantFound)
			}
		})
	}
}
