package dynacert

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rancher/dynamiclistener/storage/file"
	corev1 "k8s.io/api/core/v1"
)

func TestRecoveringStorage_CorruptJSONReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dynamic-cert.json")
	if err := os.WriteFile(path, []byte("-----BEGIN CERTIFICATE-----\n\x00\x00BADDATA"), 0o600); err != nil {
		t.Fatal(err)
	}

	s := &RecoveringStorage{Path: path, Inner: file.New(path)}
	secret, err := s.Get()
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if secret != nil {
		t.Fatalf("expected nil secret for corrupt cache, got %#v", secret)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected corrupt cache file to be removed, stat err=%v", err)
	}
}

func TestRecoveringStorage_ValidJSONPreserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dynamic-cert.json")
	want := &corev1.Secret{Data: map[string][]byte{"tls.crt": []byte("cert"), "tls.key": []byte("key")}}
	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	s := &RecoveringStorage{Path: path, Inner: file.New(path)}
	got, err := s.Get()
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got == nil || string(got.Data["tls.crt"]) != "cert" {
		t.Fatalf("unexpected secret: %#v", got)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("valid cache should remain: %v", err)
	}
}

func TestRecoveringStorage_MissingFileOK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dynamic-cert.json")
	s := &RecoveringStorage{Path: path, Inner: file.New(path)}
	secret, err := s.Get()
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if secret != nil {
		t.Fatalf("expected nil secret for missing cache, got %#v", secret)
	}
}
