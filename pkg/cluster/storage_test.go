package cluster

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/etcd/store"
	"github.com/k3s-io/kine/pkg/app"
	"github.com/k3s-io/kine/pkg/endpoint"
	"go.etcd.io/etcd/api/v3/mvccpb"
)

type updateStore struct {
	store.ReadWriteCloser
	rev   int64
	value []byte
	err   error
}

func (s *updateStore) Create(_ context.Context, _ string, value []byte) error {
	s.value = value
	return nil
}

func (s *updateStore) Delete(_ context.Context, _ string, rev int64) error {
	s.rev = rev
	return nil
}

func (s *updateStore) Update(_ context.Context, _ string, rev int64, value []byte) error {
	s.rev, s.value = rev, value
	return s.err
}

func Test_UnitmigrateTokens(t *testing.T) {
	const token = "test-passphrase"
	tests := []struct {
		name              string
		enabled, enforced bool
		value             string
		updateErr         error
		wantWrite         bool
		rotate            bool
	}{
		{"fips off leaves legacy data", false, false, legacyVector, nil, false, false},
		{"fips on migrates legacy data", true, false, legacyVector, nil, true, false},
		{"fips only does not migrate", true, true, legacyVector, nil, false, false},
		{"fips on leaves v2 data", true, false, v2Prefix + "salt:data", nil, false, false},
		{"lost compare-and-swap is not an error", true, false, legacyVector, errors.New("revision mismatch"), true, false},
		{"token rotation keeps v2 data", false, false, "", nil, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setFIPS(t, tt.enabled, tt.enforced)
			s := &updateStore{err: tt.updateErr}
			value, oldToken := []byte(tt.value), token
			if tt.rotate {
				oldToken = "old-passphrase"
				value, _ = encrypt(oldToken, []byte("bootstrap payload"), true)
			}
			list := []*mvccpb.KeyValue{{Key: []byte(storageKey(oldToken)), Value: value, ModRevision: 7}}
			if err := migrateTokens(context.Background(), list, s, storageKey(""), storageKey(token), token, oldToken); err != nil {
				t.Fatalf("migrateTokens() error = %v", err)
			}
			if !tt.wantWrite {
				if s.value != nil {
					t.Fatalf("migrateTokens() wrote %q", s.value)
				}
				return
			}
			got, err := decrypt(token, s.value)
			if s.rev != 7 || !bytes.HasPrefix(s.value, []byte(v2Prefix)) || err != nil || string(got) != "bootstrap payload" {
				t.Fatalf("migrateTokens() wrote %q at revision %d, decrypt() = %q, %v", s.value, s.rev, got, err)
			}
		})
	}
}

func Test_UnitSave(t *testing.T) {
	const token = "test-passphrase"
	tests := []struct {
		name     string
		existing string
		wantV2   bool
	}{
		{"fips off keeps legacy data", "salt:data", false},
		{"fips off keeps v2 data", v2Prefix + "salt:data", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setFIPS(t, false, false)
			kine := app.Config(nil)
			kine.Listener = "0.0.0.0:0"
			kine.Endpoint = "sqlite://" + t.TempDir() + "/state.db?_journal=WAL&cache=shared&_busy_timeout=30000&_txlock=immediate"
			kine.WaitGroup = &sync.WaitGroup{}
			t.Cleanup(func() { kine.WaitGroup.Wait() })
			etcdConfig, err := endpoint.Listen(t.Context(), kine)
			if err != nil {
				t.Fatalf("endpoint.Listen() error = %v", err)
			}
			s, err := store.NewRemoteStore(etcdConfig)
			if err != nil {
				t.Fatalf("NewRemoteStore() error = %v", err)
			}
			defer s.Close()
			if err := s.Create(t.Context(), storageKey(token), []byte(tt.existing)); err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			control := &config.Control{Token: token, Runtime: &config.ControlRuntime{EtcdConfig: etcdConfig}}
			if err := Save(t.Context(), control, true); err != nil {
				t.Fatalf("Save() error = %v", err)
			}
			kv, err := s.Get(t.Context(), storageKey(token))
			if err != nil || bytes.HasPrefix(kv.Value, []byte(v2Prefix)) != tt.wantV2 {
				t.Fatalf("Get() = %v, %v, want v2 %v", kv, err, tt.wantV2)
			}
			if _, err := decrypt(token, kv.Value); err != nil {
				t.Fatalf("decrypt() error = %v", err)
			}
		})
	}
}
