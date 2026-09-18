package util

import "testing"

func TestImageWithRegistry(t *testing.T) {
	tests := []struct {
		name     string
		registry string
		ref      string
		want     string
	}{
		{
			name:     "no registry configured leaves a qualified reference alone",
			registry: "",
			ref:      "ghcr.io/k3s-io/klipper-lb:v0.4.17",
			want:     "ghcr.io/k3s-io/klipper-lb:v0.4.17",
		},
		{
			name:     "no registry configured leaves a bare reference alone",
			registry: "",
			ref:      "rancher/mirrored-pause:3.10.2",
			want:     "rancher/mirrored-pause:3.10.2",
		},
		{
			name:     "qualified reference has its host replaced",
			registry: "registry.example.com",
			ref:      "ghcr.io/k3s-io/klipper-lb:v0.4.17",
			want:     "registry.example.com/k3s-io/klipper-lb:v0.4.17",
		},
		{
			name:     "bare reference is prefixed",
			registry: "registry.example.com",
			ref:      "rancher/mirrored-pause:3.10.2",
			want:     "registry.example.com/rancher/mirrored-pause:3.10.2",
		},
		{
			name:     "registry with a path component keeps the repository path",
			registry: "registry.example.com/mirror",
			ref:      "ghcr.io/k3s-io/mirrored-local-path-provisioner:v0.0.37",
			want:     "registry.example.com/mirror/k3s-io/mirrored-local-path-provisioner:v0.0.37",
		},
		{
			name:     "slash on the registry is ignored",
			registry: "registry.example.com/",
			ref:      "ghcr.io/k3s-io/klipper-helm:v0.13.3",
			want:     "registry.example.com/k3s-io/klipper-helm:v0.13.3",
		},
		{
			name:     "digest is carried along",
			registry: "registry.example.com",
			ref:      "ghcr.io/k3s-io/klipper-lb@sha256:0000000000000000000000000000000000000000000000000000000000000000",
			want:     "registry.example.com/k3s-io/klipper-lb@sha256:0000000000000000000000000000000000000000000000000000000000000000",
		},
		{
			name:     "registry with a port keeps the port when replacing a host",
			registry: "registry.example.com:5000",
			ref:      "ghcr.io/k3s-io/klipper-lb:v0.4.17",
			want:     "registry.example.com:5000/k3s-io/klipper-lb:v0.4.17",
		},
		{
			name:     "registry with a port keeps the port when prefixing",
			registry: "registry.example.com:5000",
			ref:      "rancher/mirrored-pause:3.10.2",
			want:     "registry.example.com:5000/rancher/mirrored-pause:3.10.2",
		},
		{
			name:     "port makes the reference's first component a host, so it is replaced",
			registry: "registry.example.com",
			ref:      "myhost:5000/k3s-io/klipper-lb:v0.4.17",
			want:     "registry.example.com/k3s-io/klipper-lb:v0.4.17",
		},
		{
			name:     "reference already served from the registry is untouched",
			registry: "registry.example.com",
			ref:      "registry.example.com/k3s-io/klipper-lb:v0.4.17",
			want:     "registry.example.com/k3s-io/klipper-lb:v0.4.17",
		},
		{
			name:     "bare reference already prefixed is not prefixed twice",
			registry: "registry.example.com",
			ref:      "registry.example.com/rancher/mirrored-pause:3.10.2",
			want:     "registry.example.com/rancher/mirrored-pause:3.10.2",
		},
		{
			name:     "empty reference stays empty",
			registry: "registry.example.com",
			ref:      "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ImageWithRegistry(tt.registry, tt.ref); got != tt.want {
				t.Errorf("ImageWithRegistry(%q, %q) = %q, want %q", tt.registry, tt.ref, got, tt.want)
			}
		})
	}
}

func TestImageWithRegistryIdempotent(t *testing.T) {
	for _, ref := range []string{
		"ghcr.io/k3s-io/klipper-lb:v0.4.17",
		"rancher/mirrored-pause:3.10.2",
		"pause:3.10.2",
	} {
		once := ImageWithRegistry("registry.example.com", ref)
		twice := ImageWithRegistry("registry.example.com", once)
		if once != twice {
			t.Errorf("ImageWithRegistry not idempotent for %q: %q then %q", ref, once, twice)
		}
	}
}
