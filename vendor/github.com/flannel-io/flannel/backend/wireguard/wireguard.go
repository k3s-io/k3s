// Copyright 2021 flannel authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package wireguard

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/flannel-io/flannel/backend"
	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/subnet"
	"golang.org/x/net/context"
)

func init() {
	backend.Register("wireguard", New)
}

type WireguardBackend struct {
	sm       subnet.Manager
	extIface *backend.ExternalInterface
}

func New(sm subnet.Manager, extIface *backend.ExternalInterface) (backend.Backend, error) {
	be := &WireguardBackend{
		sm:       sm,
		extIface: extIface,
	}

	return be, nil
}

func newSubnetAttrs(publicIP net.IP, publicIPv6 net.IP, dev, v6Dev *wgDevice, publicKey string) (*subnet.LeaseAttrs, error) {
	data, err := json.Marshal(&wireguardLeaseAttrs{
		PublicKey: publicKey,
	})
	if err != nil {
		return nil, err
	}

	leaseAttrs := &subnet.LeaseAttrs{
		BackendType: "wireguard",
	}

	if publicIP != nil && dev != nil {
		leaseAttrs.PublicIP = ip.FromIP(publicIP)
		leaseAttrs.BackendData = json.RawMessage(data)
	}

	if publicIPv6 != nil && v6Dev != nil {
		leaseAttrs.PublicIPv6 = ip.FromIP6(publicIPv6)
		leaseAttrs.BackendV6Data = json.RawMessage(data)
	}

	return leaseAttrs, nil
}

func (be *WireguardBackend) RegisterNetwork(ctx context.Context, wg *sync.WaitGroup, config *subnet.Config) (backend.Network, error) {
	// Parse out configuration
	cfg := struct {
		ListenPort                  int
		ListenPortV6                int
		PSK                         string
		PersistentKeepaliveInterval time.Duration
	}{
		ListenPort:                  51820,
		ListenPortV6:                51821,
		PersistentKeepaliveInterval: 0,
	}

	if len(config.Backend) > 0 {
		if err := json.Unmarshal(config.Backend, &cfg); err != nil {
			return nil, fmt.Errorf("error decoding backend config: %w", err)
		}
	}

	keepalive := cfg.PersistentKeepaliveInterval * time.Second

	var err error
	var dev, v6Dev *wgDevice
	var publicKey string
	if config.EnableIPv4 {
		devAttrs := wgDeviceAttrs{
			keepalive:  &keepalive,
			listenPort: cfg.ListenPort,
			name:       "flannel-wg",
		}
		err := devAttrs.setupKeys(cfg.PSK)
		if err != nil {
			return nil, err
		}
		dev, err = newWGDevice(&devAttrs, ctx, wg)
		if err != nil {
			return nil, err
		}
		publicKey = devAttrs.publicKey.String()
	}

	// We create a second network device for IPv6 to ensure the inter-host communication is based on IPv6.
	// We are required to do this because wireguard does not allow to have a peer with multiple endpoints.
	if config.EnableIPv6 {
		v6DevAttrs := wgDeviceAttrs{
			keepalive:  &keepalive,
			listenPort: cfg.ListenPortV6,
			name:       "flannel-wg-v6",
		}
		err := v6DevAttrs.setupKeys(cfg.PSK)
		if err != nil {
			return nil, err
		}
		v6Dev, err = newWGDevice(&v6DevAttrs, ctx, wg)
		if err != nil {
			return nil, err
		}
		publicKey = v6DevAttrs.publicKey.String()
	}

	subnetAttrs, err := newSubnetAttrs(be.extIface.ExtAddr, be.extIface.ExtV6Addr, dev, v6Dev, publicKey)
	if err != nil {
		return nil, err
	}

	lease, err := be.sm.AcquireLease(ctx, subnetAttrs)
	switch err {
	case nil:
	case context.Canceled, context.DeadlineExceeded:
		return nil, err
	default:
		return nil, fmt.Errorf("failed to acquire lease: %w", err)

	}

	if config.EnableIPv4 {
		err = dev.Configure(lease.Subnet.IP, config.Network)
		if err != nil {
			return nil, err
		}
	}

	if config.EnableIPv6 {
		err = v6Dev.ConfigureV6(lease.IPv6Subnet.IP, config.IPv6Network)
		if err != nil {
			return nil, err
		}
	}

	return newNetwork(be.sm, be.extIface, dev, v6Dev, lease)
}
