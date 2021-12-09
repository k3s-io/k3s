//go:build !linux && !openbsd && !windows
// +build !linux,!openbsd,!windows

package wgctrl

import (
	"golang.zx2c4.com/wireguard/wgctrl/internal/wginternal"
	"golang.zx2c4.com/wireguard/wgctrl/internal/wguser"
)

// newClients configures wginternal.Clients for systems which only support
// userspace WireGuard implementations.
func newClients() ([]wginternal.Client, error) {
	c, err := wguser.New()
	if err != nil {
		return nil, err
	}

	return []wginternal.Client{c}, nil
}
