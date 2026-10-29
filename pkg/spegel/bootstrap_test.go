package spegel

import (
	"context"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func Test_UnitBootstrapperRun(t *testing.T) {
	tests := []struct {
		name         string
		bootstrapper Bootstrapper
		wantSelf     bool
	}{
		{name: "self", bootstrapper: NewSelfBootstrapper(), wantSelf: true},
		{name: "not self", bootstrapper: NewNotSelfBootstrapper(NewSelfBootstrapper())},
		{name: "chain", bootstrapper: NewChainingBootstrapper(NewSelfBootstrapper()), wantSelf: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			id := peer.AddrInfo{ID: peer.ID("self")}
			require.NoError(t, tt.bootstrapper.Run(ctx, id))
			peers, err := tt.bootstrapper.Get(t.Context())
			require.NoError(t, err)
			if tt.wantSelf {
				require.Equal(t, []peer.AddrInfo{id}, peers)
			} else {
				require.Empty(t, peers)
			}
		})
	}
}
