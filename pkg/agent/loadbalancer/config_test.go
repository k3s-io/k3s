package loadbalancer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func readLBConfig(t *testing.T, path string) *lbConfig {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config %s: %v", path, err)
	}

	cfg := &lbConfig{}
	if err := json.Unmarshal(data, cfg); err != nil {
		t.Fatalf("failed to unmarshal config %s: %v", path, err)
	}
	return cfg
}

func Test_UnitWriteConfig(t *testing.T) {
	tests := []struct {
		name          string
		scheme        string
		defaultServer string
		servers       []string
		wantServerURL string
		wantServers   []string
	}{
		{
			name:          "writes default and server addresses",
			scheme:        "https",
			defaultServer: "127.0.0.1:6443",
			servers:       []string{"10.0.0.10:6443", "10.0.0.11:6443"},
			wantServerURL: "https://127.0.0.1:6443",
			wantServers:   []string{"10.0.0.10:6443", "10.0.0.11:6443"},
		},
		{
			name:          "writes only default when no additional servers",
			scheme:        "http",
			defaultServer: "127.0.0.1:8080",
			servers:       nil,
			wantServerURL: "http://127.0.0.1:8080",
			wantServers:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "lb.json")
			lb := &LoadBalancer{
				serviceName: "unit-test-lb",
				configFile:  configPath,
				scheme:      tt.scheme,
			}
			lb.servers.setDefaultAddress(lb.serviceName, tt.defaultServer)
			lb.servers.setAddresses(lb.serviceName, tt.servers)

			if err := lb.writeConfig(); err != nil {
				t.Fatalf("writeConfig() error = %v", err)
			}

			cfg := readLBConfig(t, configPath)
			if cfg.ServerURL != tt.wantServerURL {
				t.Fatalf("ServerURL = %q, want %q", cfg.ServerURL, tt.wantServerURL)
			}

			if len(cfg.ServerAddresses) != len(tt.wantServers) {
				t.Fatalf("ServerAddresses len = %d, want %d", len(cfg.ServerAddresses), len(tt.wantServers))
			}
			for _, want := range tt.wantServers {
				if !slices.Contains(cfg.ServerAddresses, want) {
					t.Fatalf("ServerAddresses = %v, missing %q", cfg.ServerAddresses, want)
				}
			}
		})
	}
}

func Test_UnitUpdateConfig(t *testing.T) {
	tests := []struct {
		name              string
		scheme            string
		defaultServer     string
		currentServers    []string
		storedConfig      *lbConfig
		storedRaw         []byte
		wantServerURL     string
		wantServerAddress []string
	}{
		{
			name:          "matching default loads addresses from stored config",
			scheme:        "https",
			defaultServer: "127.0.0.1:6443",
			storedConfig: &lbConfig{
				ServerURL:       "https://127.0.0.1:6443",
				ServerAddresses: []string{"10.0.0.20:6443", "10.0.0.21:6443"},
			},
			wantServerURL:     "https://127.0.0.1:6443",
			wantServerAddress: []string{"10.0.0.20:6443", "10.0.0.21:6443"},
		},
		{
			name:           "mismatched default rewrites config",
			scheme:         "https",
			defaultServer:  "127.0.0.1:6443",
			currentServers: []string{"10.0.0.30:6443"},
			storedConfig: &lbConfig{
				ServerURL:       "https://127.0.0.9:6443",
				ServerAddresses: []string{"10.0.0.99:6443"},
			},
			wantServerURL:     "https://127.0.0.1:6443",
			wantServerAddress: []string{"10.0.0.30:6443"},
		},
		{
			name:              "invalid config rewrites config",
			scheme:            "https",
			defaultServer:     "127.0.0.1:6443",
			currentServers:    []string{"10.0.0.40:6443"},
			storedRaw:         []byte("not-json"),
			wantServerURL:     "https://127.0.0.1:6443",
			wantServerAddress: []string{"10.0.0.40:6443"},
		},
		{
			name:              "missing config writes current config",
			scheme:            "http",
			defaultServer:     "127.0.0.1:8080",
			currentServers:    []string{"10.0.0.50:8080"},
			wantServerURL:     "http://127.0.0.1:8080",
			wantServerAddress: []string{"10.0.0.50:8080"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "lb.json")
			lb := &LoadBalancer{
				serviceName: "unit-test-lb",
				configFile:  configPath,
				scheme:      tt.scheme,
			}
			lb.servers.setDefaultAddress(lb.serviceName, tt.defaultServer)
			lb.servers.setAddresses(lb.serviceName, tt.currentServers)

			if tt.storedConfig != nil {
				data, err := json.Marshal(tt.storedConfig)
				if err != nil {
					t.Fatalf("json.Marshal() error = %v", err)
				}
				if err := os.WriteFile(configPath, data, 0644); err != nil {
					t.Fatalf("os.WriteFile() error = %v", err)
				}
			} else if tt.storedRaw != nil {
				if err := os.WriteFile(configPath, tt.storedRaw, 0644); err != nil {
					t.Fatalf("os.WriteFile() error = %v", err)
				}
			}

			if err := lb.updateConfig(); err != nil {
				t.Fatalf("updateConfig() error = %v", err)
			}

			cfg := readLBConfig(t, configPath)
			if cfg.ServerURL != tt.wantServerURL {
				t.Fatalf("ServerURL = %q, want %q", cfg.ServerURL, tt.wantServerURL)
			}
			if len(cfg.ServerAddresses) != len(tt.wantServerAddress) {
				t.Fatalf("ServerAddresses len = %d, want %d", len(cfg.ServerAddresses), len(tt.wantServerAddress))
			}
			for _, want := range tt.wantServerAddress {
				if !slices.Contains(cfg.ServerAddresses, want) {
					t.Fatalf("ServerAddresses = %v, missing %q", cfg.ServerAddresses, want)
				}
			}
		})
	}
}
